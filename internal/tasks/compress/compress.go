// ABOUTME: Compress task executor for archive operations (tar, gzip, bzip2, zip)
// ABOUTME: Supports creating and extracting archives with various compression formats

package compress

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	bzip2w "github.com/dsnet/compress/bzip2"
	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles compress task execution
type Executor struct{}

// CompressConfig represents the configuration for a compress task
type CompressConfig struct {
	Path        string   `yaml:"path" json:"path"`
	State       string   `yaml:"state,omitempty" json:"state,omitempty"`
	Format      string   `yaml:"format,omitempty" json:"format,omitempty"`
	Sources     []string `yaml:"sources,omitempty" json:"sources,omitempty"`
	Destination string   `yaml:"destination,omitempty" json:"destination,omitempty"`
	Exclude     []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	Include     []string `yaml:"include,omitempty" json:"include,omitempty"`
	BaseDir     string   `yaml:"base_dir,omitempty" json:"base_dir,omitempty"`
	Overwrite   bool     `yaml:"overwrite,omitempty" json:"overwrite,omitempty"`
	PreserveDir bool     `yaml:"preserve_dir,omitempty" json:"preserve_dir,omitempty"`
}

// Archive states
const (
	StateCreate  = "create"
	StateExtract = "extract"
	StatePresent = "present"
	StateAbsent  = "absent"
)

// Archive formats
const (
	FormatTarGz  = "tar.gz"
	FormatTgz    = "tgz"
	FormatTarBz2 = "tar.bz2"
	FormatTbz2   = "tbz2"
	FormatTar    = "tar"
	FormatZip    = "zip"
	FormatGzip   = "gzip"
	FormatGz     = "gz"
	FormatBzip2  = "bzip2"
	FormatBz2    = "bz2"
)

// New creates a new compress executor
func New() *Executor {
	return &Executor{}
}

// Execute runs a compress task
func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
		Status:    types.TaskRunning,
		Output:    make(map[string]interface{}),
	}

	// Parse compress configuration
	config, err := e.parseConfig(task, contextManager)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to parse configuration: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Execute the compress operation
	execResult := e.executeCompressOperation(ctx, config)

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
		return fmt.Errorf("invalid compress configuration: %w", err)
	}

	// Path is required
	if config.Path == "" {
		return fmt.Errorf("compress task must specify 'path'")
	}

	// Validate state
	validStates := []string{StateCreate, StateExtract, StatePresent, StateAbsent}
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

	// Validate format
	if config.Format != "" {
		validFormats := []string{FormatTarGz, FormatTgz, FormatTarBz2, FormatTbz2, FormatTar, FormatZip, FormatGzip, FormatGz, FormatBzip2, FormatBz2}
		valid := false
		for _, format := range validFormats {
			if config.Format == format {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid format '%s', must be one of: %s", config.Format, strings.Join(validFormats, ", "))
		}
	}

	// Sources required for create operation
	if (config.State == StateCreate || config.State == StatePresent) && len(config.Sources) == 0 {
		return fmt.Errorf("sources required for create/present state")
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig parses and evaluates the task configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*CompressConfig, error) {
	// First evaluate all templates in the config
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate task configuration: %w", err)
	}

	return e.parseConfigRaw(evaluatedConfig)
}

// parseConfigRaw parses raw configuration without template evaluation
func (e *Executor) parseConfigRaw(configMap map[string]interface{}) (*CompressConfig, error) {
	config := &CompressConfig{
		State: StatePresent, // Default to present
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

		case "format":
			if str, ok := value.(string); ok {
				config.Format = str
			} else {
				return nil, fmt.Errorf("format must be a string")
			}

		case "sources":
			if sources, ok := value.([]interface{}); ok {
				config.Sources = make([]string, len(sources))
				for i, source := range sources {
					if str, ok := source.(string); ok {
						config.Sources[i] = str
					} else {
						return nil, fmt.Errorf("all sources must be strings")
					}
				}
			} else {
				return nil, fmt.Errorf("sources must be an array of strings")
			}

		case "destination":
			if str, ok := value.(string); ok {
				config.Destination = str
			} else {
				return nil, fmt.Errorf("destination must be a string")
			}

		case "exclude":
			if exclude, ok := value.([]interface{}); ok {
				config.Exclude = make([]string, len(exclude))
				for i, item := range exclude {
					if str, ok := item.(string); ok {
						config.Exclude[i] = str
					} else {
						return nil, fmt.Errorf("all exclude patterns must be strings")
					}
				}
			} else {
				return nil, fmt.Errorf("exclude must be an array of strings")
			}

		case "include":
			if include, ok := value.([]interface{}); ok {
				config.Include = make([]string, len(include))
				for i, item := range include {
					if str, ok := item.(string); ok {
						config.Include[i] = str
					} else {
						return nil, fmt.Errorf("all include patterns must be strings")
					}
				}
			} else {
				return nil, fmt.Errorf("include must be an array of strings")
			}

		case "base_dir":
			if str, ok := value.(string); ok {
				config.BaseDir = str
			} else {
				return nil, fmt.Errorf("base_dir must be a string")
			}

		case "overwrite":
			if b, ok := value.(bool); ok {
				config.Overwrite = b
			} else {
				return nil, fmt.Errorf("overwrite must be a boolean")
			}

		case "preserve_dir":
			if b, ok := value.(bool); ok {
				config.PreserveDir = b
			} else {
				return nil, fmt.Errorf("preserve_dir must be a boolean")
			}
		}
	}

	// Auto-detect format from file extension if not specified
	if config.Format == "" {
		config.Format = e.detectFormat(config.Path)
	}

	return config, nil
}

// detectFormat auto-detects archive format from file extension
func (e *Executor) detectFormat(path string) string {
	lowerPath := strings.ToLower(path)
	if strings.HasSuffix(lowerPath, ".tar.gz") || strings.HasSuffix(lowerPath, ".tgz") {
		return FormatTarGz
	} else if strings.HasSuffix(lowerPath, ".tar.bz2") || strings.HasSuffix(lowerPath, ".tbz2") {
		return FormatTarBz2
	} else if strings.HasSuffix(lowerPath, ".tar") {
		return FormatTar
	} else if strings.HasSuffix(lowerPath, ".zip") {
		return FormatZip
	} else if strings.HasSuffix(lowerPath, ".gz") {
		return FormatGzip
	} else if strings.HasSuffix(lowerPath, ".bz2") {
		return FormatBzip2
	}
	return FormatTarGz // Default
}

// executeCompressOperation executes the actual compress operation
func (e *Executor) executeCompressOperation(ctx context.Context, config *CompressConfig) *types.TaskResult {
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

	result.Output["path"] = path
	result.Output["format"] = config.Format

	switch config.State {
	case StateAbsent:
		return e.handleAbsent(path, result)

	case StateExtract:
		return e.handleExtract(path, config, result)

	case StateCreate, StatePresent:
		return e.handleCreate(path, config, result)

	default:
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Unsupported state: %s", config.State)
		return result
	}
}

// handleAbsent handles absent state (remove archive)
func (e *Executor) handleAbsent(path string, result *types.TaskResult) *types.TaskResult {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		result.Status = types.TaskSuccess
		result.Message = "Archive is already absent"
		result.Output["changed"] = false
		return result
	}

	if err := os.Remove(path); err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to remove archive: %v", err)
		return result
	}

	result.Status = types.TaskSuccess
	result.Message = "Archive removed successfully"
	result.Output["changed"] = true
	return result
}

// handleExtract handles extract state
func (e *Executor) handleExtract(path string, config *CompressConfig, result *types.TaskResult) *types.TaskResult {
	// Check if archive exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		result.Status = types.TaskFailed
		result.Message = "Archive file does not exist"
		return result
	}

	// Determine extraction destination
	destDir := config.Destination
	if destDir == "" {
		destDir = filepath.Dir(path)
	}

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to create destination directory: %v", err)
		return result
	}

	// Extract based on format
	var extractedFiles []string
	var err error

	switch config.Format {
	case FormatTarGz, FormatTgz:
		extractedFiles, err = e.extractTarGz(path, destDir, config)
	case FormatTarBz2, FormatTbz2:
		extractedFiles, err = e.extractTarBz2(path, destDir, config)
	case FormatTar:
		extractedFiles, err = e.extractTar(path, destDir, config)
	case FormatZip:
		extractedFiles, err = e.extractZip(path, destDir, config)
	default:
		err = fmt.Errorf("extraction not supported for format: %s", config.Format)
	}

	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to extract archive: %v", err)
		return result
	}

	result.Status = types.TaskSuccess
	result.Message = fmt.Sprintf("Archive extracted successfully (%d files)", len(extractedFiles))
	result.Output["changed"] = true
	result.Output["extracted_files"] = len(extractedFiles)
	result.Output["destination"] = destDir

	return result
}

// handleCreate handles create state
func (e *Executor) handleCreate(path string, config *CompressConfig, result *types.TaskResult) *types.TaskResult {
	// Check if archive already exists
	if _, err := os.Stat(path); err == nil && !config.Overwrite {
		result.Status = types.TaskSuccess
		result.Message = "Archive already exists"
		result.Output["changed"] = false
		return result
	}

	// Create parent directory if needed
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to create parent directory: %v", err)
		return result
	}

	// Create archive based on format
	var archivedFiles []string
	var err error

	switch config.Format {
	case FormatTarGz, FormatTgz:
		archivedFiles, err = e.createTarGz(path, config)
	case FormatTarBz2, FormatTbz2:
		archivedFiles, err = e.createTarBz2(path, config)
	case FormatTar:
		archivedFiles, err = e.createTar(path, config)
	case FormatZip:
		archivedFiles, err = e.createZip(path, config)
	default:
		err = fmt.Errorf("creation not supported for format: %s", config.Format)
	}

	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to create archive: %v", err)
		return result
	}

	result.Status = types.TaskSuccess
	result.Message = fmt.Sprintf("Archive created successfully (%d files)", len(archivedFiles))
	result.Output["changed"] = true
	result.Output["archived_files"] = len(archivedFiles)

	return result
}

// extractTarGz extracts a tar.gz archive
func (e *Executor) extractTarGz(archivePath, destDir string, config *CompressConfig) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gzr.Close() }()

	return e.extractTarReader(tar.NewReader(gzr), destDir, config)
}

// extractTarBz2 extracts a tar.bz2 archive
func (e *Executor) extractTarBz2(archivePath, destDir string, config *CompressConfig) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	bzr := bzip2.NewReader(file)

	return e.extractTarReader(tar.NewReader(bzr), destDir, config)
}

// extractTar extracts a tar archive
func (e *Executor) extractTar(archivePath, destDir string, config *CompressConfig) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return e.extractTarReader(tar.NewReader(file), destDir, config)
}

// extractTarReader extracts files from a tar reader
func (e *Executor) extractTarReader(tr *tar.Reader, destDir string, config *CompressConfig) ([]string, error) {
	var extractedFiles []string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Skip if file should be excluded
		if e.shouldExclude(header.Name, config) {
			continue
		}

		target := filepath.Join(destDir, header.Name)

		// Ensure target is within destination directory (security check)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue // Skip files that would extract outside destination
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return nil, err
			}

		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, err
			}

			// Create file
			outFile, err := os.Create(target)
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				_ = outFile.Close()
				return nil, err
			}
			_ = outFile.Close()

			// Set file permissions
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return nil, err
			}

			extractedFiles = append(extractedFiles, target)
		}
	}

	return extractedFiles, nil
}

// extractZip extracts a zip archive
func (e *Executor) extractZip(archivePath, destDir string, config *CompressConfig) ([]string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()

	var extractedFiles []string

	for _, file := range reader.File {
		// Skip if file should be excluded
		if e.shouldExclude(file.Name, config) {
			continue
		}

		target := filepath.Join(destDir, file.Name)

		// Ensure target is within destination directory (security check)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue // Skip files that would extract outside destination
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.FileInfo().Mode()); err != nil {
				return nil, err
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, err
		}

		// Extract file
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}

		outFile, err := os.Create(target)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}

		_, err = io.Copy(outFile, rc)
		_ = outFile.Close()
		_ = rc.Close()

		if err != nil {
			return nil, err
		}

		// Set file permissions
		if err := os.Chmod(target, file.FileInfo().Mode()); err != nil {
			return nil, err
		}

		extractedFiles = append(extractedFiles, target)
	}

	return extractedFiles, nil
}

// createTarGz creates a tar.gz archive
func (e *Executor) createTarGz(archivePath string, config *CompressConfig) ([]string, error) {
	file, err := os.Create(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	gzw := gzip.NewWriter(file)
	defer func() { _ = gzw.Close() }()

	return e.createTarWriter(tar.NewWriter(gzw), config)
}

// createTarBz2 creates a tar.bz2 archive
func (e *Executor) createTarBz2(archivePath string, config *CompressConfig) ([]string, error) {
	file, err := os.Create(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	bzwConfig := &bzip2w.WriterConfig{Level: bzip2w.DefaultCompression}
	bzw, err := bzip2w.NewWriter(file, bzwConfig)
	if err != nil {
		return nil, err
	}
	defer func() { _ = bzw.Close() }()

	return e.createTarWriter(tar.NewWriter(bzw), config)
}

// createTar creates a tar archive
func (e *Executor) createTar(archivePath string, config *CompressConfig) ([]string, error) {
	file, err := os.Create(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return e.createTarWriter(tar.NewWriter(file), config)
}

// createTarWriter creates archive files using a tar writer
func (e *Executor) createTarWriter(tw *tar.Writer, config *CompressConfig) ([]string, error) {
	defer func() { _ = tw.Close() }()

	var archivedFiles []string

	for _, source := range config.Sources {
		sourcePath, err := filepath.Abs(source)
		if err != nil {
			return nil, err
		}

		err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip if file should be excluded
			if e.shouldExclude(path, config) {
				return nil
			}

			// Create archive path
			var archivePath string
			if config.BaseDir != "" {
				relPath, err := filepath.Rel(config.BaseDir, path)
				if err != nil {
					return err
				}
				archivePath = relPath
			} else {
				archivePath = path
			}

			// Create tar header
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = archivePath

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if !info.IsDir() {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer func() { _ = file.Close() }()

				if _, err := io.Copy(tw, file); err != nil {
					return err
				}

				archivedFiles = append(archivedFiles, path)
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return archivedFiles, nil
}

// createZip creates a zip archive
func (e *Executor) createZip(archivePath string, config *CompressConfig) ([]string, error) {
	file, err := os.Create(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	writer := zip.NewWriter(file)
	defer func() { _ = writer.Close() }()

	var archivedFiles []string

	for _, source := range config.Sources {
		sourcePath, err := filepath.Abs(source)
		if err != nil {
			return nil, err
		}

		err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip if file should be excluded
			if e.shouldExclude(path, config) {
				return nil
			}

			// Skip directories for zip archives (they're created implicitly)
			if info.IsDir() {
				return nil
			}

			// Create archive path
			var archivePath string
			if config.BaseDir != "" {
				relPath, err := filepath.Rel(config.BaseDir, path)
				if err != nil {
					return err
				}
				archivePath = relPath
			} else {
				archivePath = path
			}

			// Create zip file entry
			zipFile, err := writer.Create(archivePath)
			if err != nil {
				return err
			}

			// Copy file content
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = srcFile.Close() }()

			if _, err := io.Copy(zipFile, srcFile); err != nil {
				return err
			}

			archivedFiles = append(archivedFiles, path)
			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return archivedFiles, nil
}

// shouldExclude checks if a file should be excluded based on include/exclude patterns
func (e *Executor) shouldExclude(path string, config *CompressConfig) bool {
	// Check exclude patterns first
	for _, excludePattern := range config.Exclude {
		if e.matchPattern(path, excludePattern) {
			return true
		}
	}

	// If include patterns are specified, only include matching files
	if len(config.Include) > 0 {
		for _, includePattern := range config.Include {
			if e.matchPattern(path, includePattern) {
				return false
			}
		}
		return true
	}

	return false
}

// matchPattern performs basic pattern matching supporting * wildcards
func (e *Executor) matchPattern(path, pattern string) bool {
	// Simple pattern matching for common cases
	if pattern == path {
		return true
	}

	// Handle simple wildcard patterns
	if strings.HasPrefix(pattern, "*.") {
		extension := pattern[1:]
		return strings.HasSuffix(path, extension)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(path, prefix)
	}

	// Default to substring matching for patterns without wildcards
	return strings.Contains(path, pattern)
}
