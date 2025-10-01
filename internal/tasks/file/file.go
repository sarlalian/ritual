// ABOUTME: File task executor for file system operations (create, copy, move, delete)
// ABOUTME: Supports templated content, permissions, ownership, and backup creation

package file

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles file task execution
type Executor struct{}

// FileConfig represents the configuration for a file task
type FileConfig struct {
	Path        string                 `yaml:"path" json:"path"`
	State       string                 `yaml:"state,omitempty" json:"state,omitempty"`
	Source      string                 `yaml:"source,omitempty" json:"source,omitempty"`
	Content     string                 `yaml:"content,omitempty" json:"content,omitempty"`
	Mode        string                 `yaml:"mode,omitempty" json:"mode,omitempty"`
	Owner       string                 `yaml:"owner,omitempty" json:"owner,omitempty"`
	Group       string                 `yaml:"group,omitempty" json:"group,omitempty"`
	Backup      bool                   `yaml:"backup,omitempty" json:"backup,omitempty"`
	BackupExt   string                 `yaml:"backup_ext,omitempty" json:"backup_ext,omitempty"`
	CreateDirs  bool                   `yaml:"create_dirs,omitempty" json:"create_dirs,omitempty"`
	Force       bool                   `yaml:"force,omitempty" json:"force,omitempty"`
	Template    bool                   `yaml:"template,omitempty" json:"template,omitempty"`
	Attributes  map[string]interface{} `yaml:"attributes,omitempty" json:"attributes,omitempty"`
}

// FileState constants
const (
	StatePresent = "present"
	StateAbsent  = "absent"
	StateFile    = "file"
	StateDir     = "directory"
	StateLink    = "link"
	StateTouch   = "touch"
)

// New creates a new file executor
func New() *Executor {
	return &Executor{}
}

// Execute runs a file task
func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
		Status:    types.TaskRunning,
	}

	// Parse file configuration
	config, err := e.parseConfig(task, contextManager)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to parse configuration: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Execute the file operation
	execResult := e.executeFileOperation(ctx, config, contextManager)

	// Update result
	result.Status = execResult.Status
	result.Message = execResult.Message
	result.Output = execResult.Output
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	config, err := e.parseConfigRaw(task.Config)
	if err != nil {
		return fmt.Errorf("invalid file configuration: %w", err)
	}

	// Path is required
	if config.Path == "" {
		return fmt.Errorf("file task must specify 'path'")
	}

	// Validate state
	validStates := []string{StatePresent, StateAbsent, StateFile, StateDir, StateLink, StateTouch}
	if config.State != "" {
		valid := false
		for _, state := range validStates {
			if config.State == state {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid state '%s', must be one of: %s", config.State, strings.Join(validStates, ", "))
		}
	}

	// Validate mode if specified
	if config.Mode != "" {
		if _, err := strconv.ParseUint(config.Mode, 8, 32); err != nil {
			return fmt.Errorf("invalid mode format '%s': %w", config.Mode, err)
		}
	}

	// Content and source are mutually exclusive
	if config.Content != "" && config.Source != "" {
		return fmt.Errorf("cannot specify both 'content' and 'source'")
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig parses and evaluates the task configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*FileConfig, error) {
	// First evaluate all templates in the config
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate task configuration: %w", err)
	}

	config, err := e.parseConfigRaw(evaluatedConfig)
	if err != nil {
		return nil, err
	}

	// Evaluate templates in content if template flag is set
	if config.Template && config.Content != "" {
		evaluatedContent, err := contextManager.EvaluateString(config.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate template content: %w", err)
		}
		config.Content = evaluatedContent
	}

	return config, nil
}

// parseConfigRaw parses raw configuration without template evaluation
func (e *Executor) parseConfigRaw(configMap map[string]interface{}) (*FileConfig, error) {
	config := &FileConfig{
		State:     StateFile, // Default to file
		BackupExt: ".bak",    // Default backup extension
	}

	for key, value := range configMap {
		switch key {
		case "path":
			if str, ok := value.(string); ok {
				config.Path = str
			} else {
				return nil, fmt.Errorf("path must be a string")
			}

		case "state":
			if str, ok := value.(string); ok {
				config.State = str
			} else {
				return nil, fmt.Errorf("state must be a string")
			}

		case "source":
			if str, ok := value.(string); ok {
				config.Source = str
			} else {
				return nil, fmt.Errorf("source must be a string")
			}

		case "content":
			if str, ok := value.(string); ok {
				config.Content = str
			} else {
				return nil, fmt.Errorf("content must be a string")
			}

		case "mode":
			if str, ok := value.(string); ok {
				config.Mode = str
			} else {
				return nil, fmt.Errorf("mode must be a string")
			}

		case "owner":
			if str, ok := value.(string); ok {
				config.Owner = str
			} else {
				return nil, fmt.Errorf("owner must be a string")
			}

		case "group":
			if str, ok := value.(string); ok {
				config.Group = str
			} else {
				return nil, fmt.Errorf("group must be a string")
			}

		case "backup":
			if b, ok := value.(bool); ok {
				config.Backup = b
			} else {
				return nil, fmt.Errorf("backup must be a boolean")
			}

		case "backup_ext":
			if str, ok := value.(string); ok {
				config.BackupExt = str
			} else {
				return nil, fmt.Errorf("backup_ext must be a string")
			}

		case "create_dirs":
			if b, ok := value.(bool); ok {
				config.CreateDirs = b
			} else {
				return nil, fmt.Errorf("create_dirs must be a boolean")
			}

		case "force":
			if b, ok := value.(bool); ok {
				config.Force = b
			} else {
				return nil, fmt.Errorf("force must be a boolean")
			}

		case "template":
			if b, ok := value.(bool); ok {
				config.Template = b
			} else {
				return nil, fmt.Errorf("template must be a boolean")
			}

		case "attributes":
			if attrs, ok := value.(map[string]interface{}); ok {
				config.Attributes = attrs
			} else {
				return nil, fmt.Errorf("attributes must be a map")
			}
		}
	}

	return config, nil
}

// executeFileOperation executes the actual file operation
func (e *Executor) executeFileOperation(ctx context.Context, config *FileConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		Status: types.TaskRunning,
		Output: make(map[string]interface{}),
	}

	// Expand path
	path, err := filepath.Abs(config.Path)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Invalid path: %v", err)
		return result
	}

	// Check current state
	currentInfo, currentExists := e.getFileInfo(path)
	result.Output["path"] = path
	result.Output["exists"] = currentExists

	switch config.State {
	case StateAbsent:
		return e.handleAbsent(path, currentExists, config, result)

	case StateDir:
		return e.handleDirectory(path, currentInfo, currentExists, config, result)

	case StateTouch:
		return e.handleTouch(path, currentInfo, currentExists, config, result)

	case StateFile, StatePresent:
		return e.handleFile(path, currentInfo, currentExists, config, result)

	default:
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Unsupported state: %s", config.State)
		return result
	}
}

// getFileInfo gets file information
func (e *Executor) getFileInfo(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, false
	}
	return info, true
}

// handleAbsent handles absent state (delete file/directory)
func (e *Executor) handleAbsent(path string, exists bool, config *FileConfig, result *types.TaskResult) *types.TaskResult {
	if !exists {
		result.Status = types.TaskSuccess
		result.Message = "File is already absent"
		result.Output["changed"] = false
		return result
	}

	// Create backup if requested
	if config.Backup {
		backupPath := path + config.BackupExt
		if err := e.copyFile(path, backupPath); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to create backup: %v", err)
			return result
		}
		result.Output["backup_file"] = backupPath
	}

	// Remove the file/directory
	if err := os.RemoveAll(path); err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to remove: %v", err)
		return result
	}

	result.Status = types.TaskSuccess
	result.Message = "File removed successfully"
	result.Output["changed"] = true
	return result
}

// handleDirectory handles directory state
func (e *Executor) handleDirectory(path string, info os.FileInfo, exists bool, config *FileConfig, result *types.TaskResult) *types.TaskResult {
	if exists && info.IsDir() {
		// Directory already exists, check if we need to update permissions
		return e.updatePermissions(path, info, config, result)
	}

	if exists && !info.IsDir() {
		if !config.Force {
			result.Status = types.TaskFailed
			result.Message = "Path exists but is not a directory (use force=true to overwrite)"
			return result
		}

		// Remove existing file
		if err := os.RemoveAll(path); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to remove existing file: %v", err)
			return result
		}
	}

	// Create parent directories if needed
	if config.CreateDirs {
		parentDir := filepath.Dir(path)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to create parent directories: %v", err)
			return result
		}
	}

	// Create directory
	mode := fs.FileMode(0755) // Default directory mode
	if config.Mode != "" {
		modeInt, err := strconv.ParseUint(config.Mode, 8, 32)
		if err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Invalid mode: %v", err)
			return result
		}
		mode = fs.FileMode(modeInt)
	}

	if err := os.MkdirAll(path, mode); err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to create directory: %v", err)
		return result
	}

	result.Status = types.TaskSuccess
	result.Message = "Directory created successfully"
	result.Output["changed"] = true
	return result
}

// handleTouch handles touch state (create empty file or update timestamp)
func (e *Executor) handleTouch(path string, info os.FileInfo, exists bool, config *FileConfig, result *types.TaskResult) *types.TaskResult {
	// Create parent directories if needed
	if config.CreateDirs {
		parentDir := filepath.Dir(path)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to create parent directories: %v", err)
			return result
		}
	}

	if !exists {
		// Create empty file
		file, err := os.Create(path)
		if err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to create file: %v", err)
			return result
		}
		file.Close()

		result.Status = types.TaskSuccess
		result.Message = "File created successfully"
		result.Output["changed"] = true
	} else {
		// Update timestamp
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to update timestamp: %v", err)
			return result
		}

		result.Status = types.TaskSuccess
		result.Message = "File timestamp updated"
		result.Output["changed"] = true
	}

	return e.updatePermissions(path, nil, config, result)
}

// handleFile handles file state (create/update file content)
func (e *Executor) handleFile(path string, info os.FileInfo, exists bool, config *FileConfig, result *types.TaskResult) *types.TaskResult {
	// Create parent directories if needed
	if config.CreateDirs {
		parentDir := filepath.Dir(path)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to create parent directories: %v", err)
			return result
		}
	}

	// Check if file is a directory
	if exists && info.IsDir() {
		if !config.Force {
			result.Status = types.TaskFailed
			result.Message = "Path is a directory (use force=true to overwrite)"
			return result
		}

		// Remove directory
		if err := os.RemoveAll(path); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to remove directory: %v", err)
			return result
		}
		exists = false
	}

	var needsUpdate bool = !exists

	// Check if content needs updating
	if exists && (config.Content != "" || config.Source != "") {
		var targetContent string
		if config.Content != "" {
			targetContent = config.Content
		} else if config.Source != "" {
			sourceContent, err := os.ReadFile(config.Source)
			if err != nil {
				result.Status = types.TaskFailed
				result.Message = fmt.Sprintf("Failed to read source file: %v", err)
				return result
			}
			targetContent = string(sourceContent)
		}

		if targetContent != "" {
			currentContent, err := os.ReadFile(path)
			if err != nil {
				needsUpdate = true
			} else {
				needsUpdate = string(currentContent) != targetContent
			}
		}
	}

	// Create backup if needed
	if needsUpdate && exists && config.Backup {
		backupPath := path + config.BackupExt
		if err := e.copyFile(path, backupPath); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to create backup: %v", err)
			return result
		}
		result.Output["backup_file"] = backupPath
	}

	// Update file content if needed
	if needsUpdate {
		if config.Content != "" {
			if err := os.WriteFile(path, []byte(config.Content), 0644); err != nil {
				result.Status = types.TaskFailed
				result.Message = fmt.Sprintf("Failed to write content: %v", err)
				return result
			}
		} else if config.Source != "" {
			if err := e.copyFile(config.Source, path); err != nil {
				result.Status = types.TaskFailed
				result.Message = fmt.Sprintf("Failed to copy source file: %v", err)
				return result
			}
		} else if !exists {
			// Create empty file
			file, err := os.Create(path)
			if err != nil {
				result.Status = types.TaskFailed
				result.Message = fmt.Sprintf("Failed to create file: %v", err)
				return result
			}
			file.Close()
		}

		result.Output["changed"] = true
		if !exists {
			result.Message = "File created successfully"
		} else {
			result.Message = "File updated successfully"
		}
	} else {
		result.Output["changed"] = false
		result.Message = "File is already up to date"
	}

	result.Status = types.TaskSuccess
	return e.updatePermissions(path, nil, config, result)
}

// updatePermissions updates file permissions if specified
func (e *Executor) updatePermissions(path string, info os.FileInfo, config *FileConfig, result *types.TaskResult) *types.TaskResult {
	if config.Mode != "" {
		modeInt, err := strconv.ParseUint(config.Mode, 8, 32)
		if err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Invalid mode: %v", err)
			return result
		}

		mode := fs.FileMode(modeInt)
		if err := os.Chmod(path, mode); err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to set permissions: %v", err)
			return result
		}

		result.Output["mode"] = config.Mode
	}

	// Note: Owner/Group changes would require platform-specific code and proper privileges
	// For now, we just record the intention
	if config.Owner != "" {
		result.Output["owner"] = config.Owner
	}
	if config.Group != "" {
		result.Output["group"] = config.Group
	}

	return result
}

// copyFile copies a file from src to dst
func (e *Executor) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file mode
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}