// ABOUTME: Copy task executor for copying files across any Afero-supported filesystem
// ABOUTME: Supports local, S3, HTTP, GCS, SSH/SCP, and other filesystems with progress tracking

package copy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/sarlalian/ritual/internal/filesystem"
	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles copy task execution
type Executor struct{}

// CopyConfig represents the configuration for a copy task
type CopyConfig struct {
	Source      string                 `yaml:"src" json:"src"`
	Destination string                 `yaml:"dest" json:"dest"`
	Recursive   bool                   `yaml:"recursive,omitempty" json:"recursive,omitempty"`
	Force       bool                   `yaml:"force,omitempty" json:"force,omitempty"`
	CreateDirs  bool                   `yaml:"create_dirs,omitempty" json:"create_dirs,omitempty"`
	Backup      bool                   `yaml:"backup,omitempty" json:"backup,omitempty"`
	BackupExt   string                 `yaml:"backup_ext,omitempty" json:"backup_ext,omitempty"`
	Mode        string                 `yaml:"mode,omitempty" json:"mode,omitempty"`
	FollowLinks bool                   `yaml:"follow_links,omitempty" json:"follow_links,omitempty"`
	Attributes  map[string]interface{} `yaml:"attributes,omitempty" json:"attributes,omitempty"`

	// AWS S3 credentials (optional)
	AWSAccessKeyID     string `yaml:"aws_access_key_id,omitempty" json:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key,omitempty" json:"aws_secret_access_key,omitempty"`
	AWSSessionToken    string `yaml:"aws_session_token,omitempty" json:"aws_session_token,omitempty"`
	AWSRegion          string `yaml:"aws_region,omitempty" json:"aws_region,omitempty"`

	// SSH/SFTP credentials (optional)
	SSHUser           string `yaml:"ssh_user,omitempty" json:"ssh_user,omitempty"`
	SSHPassword       string `yaml:"ssh_password,omitempty" json:"ssh_password,omitempty"`
	SSHPrivateKey     string `yaml:"ssh_private_key,omitempty" json:"ssh_private_key,omitempty"`
	SSHPrivateKeyPath string `yaml:"ssh_private_key_path,omitempty" json:"ssh_private_key_path,omitempty"`
}

// CopyResult holds the result of a copy operation
type CopyResult struct {
	Status        string
	Message       string
	SourcePath    string
	DestPath      string
	BytesCopied   int64
	FilesCopied   int
	Skipped       int
	AlreadyExists bool
	Output        map[string]interface{}
}

// New creates a new copy executor
func New() *Executor {
	return &Executor{}
}

// Execute runs a copy task
func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
		Status:    types.TaskRunning,
		Output:    make(map[string]interface{}),
	}

	// Parse copy configuration
	config, err := e.parseConfig(task, contextManager)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to parse configuration: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Execute the copy operation
	copyResult := e.executeCopy(ctx, config)

	// Update result
	if copyResult.Status == "success" {
		result.Status = types.TaskSuccess
	} else {
		result.Status = types.TaskFailed
	}
	result.Message = copyResult.Message
	result.Output = copyResult.Output
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	config, err := e.parseConfigRaw(task.Config)
	if err != nil {
		return fmt.Errorf("invalid copy configuration: %w", err)
	}

	// Source is required
	if config.Source == "" {
		return fmt.Errorf("copy task must specify 'src' (source)")
	}

	// Destination is required
	if config.Destination == "" {
		return fmt.Errorf("copy task must specify 'dest' (destination)")
	}

	// Validate mode if specified
	if config.Mode != "" {
		if _, err := parseFileMode(config.Mode); err != nil {
			return fmt.Errorf("invalid mode '%s': %w", config.Mode, err)
		}
	}

	return nil
}

// SupportsDryRun returns true as copy operations can be simulated
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig parses and evaluates the configuration with template evaluation
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*CopyConfig, error) {
	// First evaluate templates in the config
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate templates: %w", err)
	}

	// Then parse into struct
	return e.parseConfigRaw(evaluatedConfig)
}

// parseConfigRaw parses raw configuration map into CopyConfig struct
func (e *Executor) parseConfigRaw(config map[string]interface{}) (*CopyConfig, error) {
	copyConfig := &CopyConfig{
		BackupExt:  ".bak",
		CreateDirs: true,
	}

	for key, value := range config {
		switch key {
		case "src", "source":
			if str, ok := value.(string); ok {
				copyConfig.Source = str
			} else {
				return nil, fmt.Errorf("src must be a string")
			}
		case "dest", "destination":
			if str, ok := value.(string); ok {
				copyConfig.Destination = str
			} else {
				return nil, fmt.Errorf("dest must be a string")
			}
		case "recursive":
			if b, ok := value.(bool); ok {
				copyConfig.Recursive = b
			}
		case "force":
			if b, ok := value.(bool); ok {
				copyConfig.Force = b
			}
		case "create_dirs":
			if b, ok := value.(bool); ok {
				copyConfig.CreateDirs = b
			}
		case "backup":
			if b, ok := value.(bool); ok {
				copyConfig.Backup = b
			}
		case "backup_ext":
			if str, ok := value.(string); ok {
				copyConfig.BackupExt = str
			}
		case "mode":
			if str, ok := value.(string); ok {
				copyConfig.Mode = str
			}
		case "follow_links":
			if b, ok := value.(bool); ok {
				copyConfig.FollowLinks = b
			}
		case "attributes":
			if m, ok := value.(map[string]interface{}); ok {
				copyConfig.Attributes = m
			}
		case "aws_access_key_id":
			if str, ok := value.(string); ok {
				copyConfig.AWSAccessKeyID = str
			}
		case "aws_secret_access_key":
			if str, ok := value.(string); ok {
				copyConfig.AWSSecretAccessKey = str
			}
		case "aws_session_token":
			if str, ok := value.(string); ok {
				copyConfig.AWSSessionToken = str
			}
		case "aws_region":
			if str, ok := value.(string); ok {
				copyConfig.AWSRegion = str
			}
		case "ssh_user":
			if str, ok := value.(string); ok {
				copyConfig.SSHUser = str
			}
		case "ssh_password":
			if str, ok := value.(string); ok {
				copyConfig.SSHPassword = str
			}
		case "ssh_private_key":
			if str, ok := value.(string); ok {
				copyConfig.SSHPrivateKey = str
			}
		case "ssh_private_key_path":
			if str, ok := value.(string); ok {
				copyConfig.SSHPrivateKeyPath = str
			}
		}
	}

	return copyConfig, nil
}

// executeCopy performs the actual copy operation
func (e *Executor) executeCopy(ctx context.Context, config *CopyConfig) *CopyResult {
	result := &CopyResult{
		SourcePath: config.Source,
		DestPath:   config.Destination,
		Output:     make(map[string]interface{}),
	}

	// Create filesystem config from copy config
	fsConfig := &filesystem.Config{
		AWSAccessKeyID:     config.AWSAccessKeyID,
		AWSSecretAccessKey: config.AWSSecretAccessKey,
		AWSSessionToken:    config.AWSSessionToken,
		AWSRegion:          config.AWSRegion,
		SSHUser:            config.SSHUser,
		SSHPassword:        config.SSHPassword,
		SSHPrivateKey:      config.SSHPrivateKey,
		SSHPrivateKeyPath:  config.SSHPrivateKeyPath,
	}

	// Parse source and destination to determine filesystems
	srcInfo, err := filesystem.ParsePath(config.Source)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Invalid source path: %v", err)
		return result
	}

	dstInfo, err := filesystem.ParsePath(config.Destination)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Invalid destination path: %v", err)
		return result
	}

	// Get source filesystem
	srcFs, err := filesystem.GetFilesystem(config.Source, fsConfig)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to create source filesystem: %v", err)
		return result
	}

	// Get destination filesystem
	dstFs, err := filesystem.GetFilesystem(config.Destination, fsConfig)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to create destination filesystem: %v", err)
		return result
	}

	// Check if source exists
	sourceInfo, err := srcFs.Stat(srcInfo.Path)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Source does not exist: %s", config.Source)
		return result
	}

	// Determine if source is a directory
	isDir := sourceInfo.IsDir()

	// If source is a directory and recursive is not enabled, fail
	if isDir && !config.Recursive {
		result.Status = "failed"
		result.Message = "Source is a directory but recursive=false"
		return result
	}

	// Check if destination exists
	destInfo, destErr := dstFs.Stat(dstInfo.Path)
	destExists := destErr == nil

	// Handle backup if requested and destination exists
	if config.Backup && destExists && !destInfo.IsDir() {
		backupPath := dstInfo.Path + config.BackupExt
		if err := e.copyFileCrossFS(dstFs, dstInfo.Path, dstFs, backupPath); err != nil {
			result.Status = "failed"
			result.Message = fmt.Sprintf("Failed to create backup: %v", err)
			return result
		}
		result.Output["backup_created"] = config.Destination + config.BackupExt
	}

	// Create destination directory if needed
	if config.CreateDirs {
		destDir := filepath.Dir(dstInfo.Path)
		if err := dstFs.MkdirAll(destDir, 0755); err != nil {
			result.Status = "failed"
			result.Message = fmt.Sprintf("Failed to create destination directory: %v", err)
			return result
		}
	}

	// Perform the copy
	if isDir {
		// Copy directory recursively
		err = e.copyDirCrossFS(srcFs, srcInfo.Path, dstFs, dstInfo.Path, config, result)
	} else {
		// Copy single file
		if destExists && !config.Force {
			result.Status = "success"
			result.Message = "Destination already exists (use force=true to overwrite)"
			result.AlreadyExists = true
			result.Skipped = 1
			result.Output["source"] = config.Source
			result.Output["destination"] = config.Destination
			result.Output["files_copied"] = 0
			result.Output["bytes_copied"] = int64(0)
			result.Output["skipped"] = 1
			return result
		}

		err = e.copyFileCrossFS(srcFs, srcInfo.Path, dstFs, dstInfo.Path)
		if err == nil {
			result.FilesCopied = 1
			result.BytesCopied = sourceInfo.Size()
		}
	}

	// Set final status
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Copy failed: %v", err)
	} else {
		result.Status = "success"
		if result.FilesCopied > 0 {
			result.Message = fmt.Sprintf("Copied %d file(s), %d bytes", result.FilesCopied, result.BytesCopied)
		} else if result.Skipped > 0 {
			result.Message = fmt.Sprintf("Skipped %d file(s) (already exist)", result.Skipped)
		} else {
			result.Message = "Copy completed"
		}
	}

	// Populate output
	result.Output["source"] = config.Source
	result.Output["destination"] = config.Destination
	result.Output["files_copied"] = result.FilesCopied
	result.Output["bytes_copied"] = result.BytesCopied
	result.Output["skipped"] = result.Skipped

	// Apply mode if specified
	if config.Mode != "" && result.FilesCopied > 0 {
		mode, _ := parseFileMode(config.Mode)
		if err := dstFs.Chmod(dstInfo.Path, mode); err != nil {
			result.Output["mode_warning"] = fmt.Sprintf("Failed to set mode: %v", err)
		} else {
			result.Output["mode_applied"] = config.Mode
		}
	}

	return result
}

// copyFileCrossFS copies a single file between potentially different filesystems
func (e *Executor) copyFileCrossFS(srcFs afero.Fs, srcPath string, dstFs afero.Fs, dstPath string) error {
	// Open source file
	srcFile, err := srcFs.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := dstFs.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	// Copy contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Try to preserve permissions (may fail for some filesystem types)
	srcInfo, err := srcFs.Stat(srcPath)
	if err == nil {
		dstFs.Chmod(dstPath, srcInfo.Mode())
	}

	return nil
}

// copyDirCrossFS recursively copies a directory between potentially different filesystems
func (e *Executor) copyDirCrossFS(srcFs afero.Fs, srcPath string, dstFs afero.Fs, dstPath string, config *CopyConfig, result *CopyResult) error {
	// Create destination directory
	if err := dstFs.MkdirAll(dstPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := afero.ReadDir(srcFs, srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcEntryPath := filepath.Join(srcPath, entry.Name())
		dstEntryPath := filepath.Join(dstPath, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := e.copyDirCrossFS(srcFs, srcEntryPath, dstFs, dstEntryPath, config, result); err != nil {
				return err
			}
		} else {
			// Check if destination exists
			_, destErr := dstFs.Stat(dstEntryPath)
			destExists := destErr == nil

			if destExists && !config.Force {
				result.Skipped++
				continue
			}

			// Copy file
			if err := e.copyFileCrossFS(srcFs, srcEntryPath, dstFs, dstEntryPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", srcEntryPath, err)
			}

			result.FilesCopied++
			result.BytesCopied += entry.Size()
		}
	}

	return nil
}


// copyFile copies a single file using Afero
func (e *Executor) copyFile(fs afero.Fs, src, dst string) error {
	// Open source file
	srcFile, err := fs.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := fs.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	// Copy contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Get source file info to copy permissions
	srcInfo, err := fs.Stat(src)
	if err == nil {
		// Try to preserve permissions
		fs.Chmod(dst, srcInfo.Mode())
	}

	return nil
}

// copyDir recursively copies a directory
func (e *Executor) copyDir(fs afero.Fs, src, dst string, config *CopyConfig, result *CopyResult) error {
	// Create destination directory
	if err := fs.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := afero.ReadDir(fs, src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := e.copyDir(fs, srcPath, dstPath, config, result); err != nil {
				return err
			}
		} else {
			// Check if destination exists
			_, destErr := fs.Stat(dstPath)
			destExists := destErr == nil

			if destExists && !config.Force {
				result.Skipped++
				continue
			}

			// Copy file
			if err := e.copyFile(fs, srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", srcPath, err)
			}

			result.FilesCopied++
			result.BytesCopied += entry.Size()
		}
	}

	return nil
}

// parseFileMode parses a file mode string (e.g., "0644") into os.FileMode
func parseFileMode(mode string) (os.FileMode, error) {
	// Remove any leading "0" for octal notation
	mode = strings.TrimPrefix(mode, "0")

	// Parse as octal
	modeInt, err := fmt.Sscanf(mode, "%o", new(uint32))
	if err != nil || modeInt != 1 {
		return 0, fmt.Errorf("invalid mode format (use octal like '0644' or '644')")
	}

	var modeValue uint32
	fmt.Sscanf(mode, "%o", &modeValue)
	return os.FileMode(modeValue), nil
}
