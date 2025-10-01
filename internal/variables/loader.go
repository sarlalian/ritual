// ABOUTME: Variable file loader for loading workflow variables from external files
// ABOUTME: Supports YAML, JSON, and .env file formats for flexible variable management

package variables

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileLoader handles loading variables from external files
type FileLoader struct {
	basePath string
}

// New creates a new variable file loader
func New(basePath string) *FileLoader {
	return &FileLoader{
		basePath: basePath,
	}
}

// LoadVariableFile loads variables from a file and returns them as a map
func (fl *FileLoader) LoadVariableFile(filePath string) (map[string]interface{}, error) {
	// Resolve relative paths against base path
	if !filepath.IsAbs(filePath) && fl.basePath != "" {
		filePath = filepath.Join(fl.basePath, filePath)
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("variable file not found: %s", filePath)
	}

	// Determine file format by extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		return fl.loadYAMLFile(filePath)
	case ".json":
		return fl.loadJSONFile(filePath)
	case ".env":
		return fl.loadEnvFile(filePath)
	default:
		// Try to detect format from content
		return fl.loadAutoDetect(filePath)
	}
}

// loadYAMLFile loads variables from a YAML file
func (fl *FileLoader) loadYAMLFile(filePath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file '%s': %w", filePath, err)
	}

	var variables map[string]interface{}
	if err := yaml.Unmarshal(content, &variables); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file '%s': %w", filePath, err)
	}

	return variables, nil
}

// loadJSONFile loads variables from a JSON file
func (fl *FileLoader) loadJSONFile(filePath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file '%s': %w", filePath, err)
	}

	var variables map[string]interface{}
	if err := yaml.Unmarshal(content, &variables); err != nil {
		// Try JSON parsing as fallback
		if jsonErr := yaml.Unmarshal(content, &variables); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse JSON file '%s': %w", filePath, err)
		}
	}

	return variables, nil
}

// loadEnvFile loads variables from a .env file
func (fl *FileLoader) loadEnvFile(filePath string) (map[string]interface{}, error) {
	envVars, err := loadEnvironmentFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load .env file '%s': %w", filePath, err)
	}

	variables := make(map[string]interface{})
	for _, envVar := range envVars {
		key, value, err := parseVariableString(envVar)
		if err != nil {
			return nil, fmt.Errorf("failed to parse variable in file '%s': %w", filePath, err)
		}
		variables[key] = value
	}

	return variables, nil
}

// loadAutoDetect attempts to detect file format and load accordingly
func (fl *FileLoader) loadAutoDetect(filePath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file '%s': %w", filePath, err)
	}

	contentStr := strings.TrimSpace(string(content))

	// Try YAML first (most permissive)
	var variables map[string]interface{}
	if err := yaml.Unmarshal(content, &variables); err == nil {
		return variables, nil
	}

	// Try .env format if it looks like key=value lines
	if strings.Contains(contentStr, "=") && !strings.Contains(contentStr, "{") {
		return fl.loadEnvFile(filePath)
	}

	return nil, fmt.Errorf("unable to determine format of file '%s'", filePath)
}

// LoadVariableFiles loads multiple variable files and merges them
func (fl *FileLoader) LoadVariableFiles(filePaths []string) (map[string]interface{}, error) {
	merged := make(map[string]interface{})

	for _, filePath := range filePaths {
		variables, err := fl.LoadVariableFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load variable file '%s': %w", filePath, err)
		}

		// Merge variables (later files override earlier ones)
		for key, value := range variables {
			merged[key] = value
		}
	}

	return merged, nil
}

// ResolveVariableReferences resolves references to other variable files
func (fl *FileLoader) ResolveVariableReferences(variables map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range variables {
		// Check if value is a string that looks like a file reference
		if strValue, ok := value.(string); ok && strings.HasPrefix(strValue, "@") {
			// This is a file reference, load the file
			filePath := strings.TrimPrefix(strValue, "@")
			fileVars, err := fl.LoadVariableFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve variable reference '%s': %w", strValue, err)
			}

			// If the file contains a single value with the same key, use that value directly
			if singleValue, exists := fileVars[key]; exists && len(fileVars) == 1 {
				result[key] = singleValue
			} else {
				// Otherwise, use the entire file content as a nested object
				result[key] = fileVars
			}
		} else if mapValue, ok := value.(map[string]interface{}); ok {
			// Recursively resolve nested maps
			resolvedMap, err := fl.ResolveVariableReferences(mapValue)
			if err != nil {
				return nil, err
			}
			result[key] = resolvedMap
		} else {
			// Regular value, copy as-is
			result[key] = value
		}
	}

	return result, nil
}

// ValidateVariableFile validates that a variable file can be loaded
func (fl *FileLoader) ValidateVariableFile(filePath string) error {
	_, err := fl.LoadVariableFile(filePath)
	return err
}

// ListVariableFiles lists all variable files in a directory
func (fl *FileLoader) ListVariableFiles(dirPath string) ([]string, error) {
	if !filepath.IsAbs(dirPath) && fl.basePath != "" {
		dirPath = filepath.Join(fl.basePath, dirPath)
	}

	var varFiles []string
	extensions := []string{".yaml", ".yml", ".json", ".env", ".vars"}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, validExt := range extensions {
			if ext == validExt {
				relPath, _ := filepath.Rel(fl.basePath, path)
				varFiles = append(varFiles, relPath)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list variable files in '%s': %w", dirPath, err)
	}

	return varFiles, nil
}

// GetVariableFileInfo returns information about a variable file
type VariableFileInfo struct {
	Path      string                 `json:"path"`
	Format    string                 `json:"format"`
	Size      int64                  `json:"size"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// GetVariableFileInfo gets detailed information about a variable file
func (fl *FileLoader) GetVariableFileInfo(filePath string, loadContent bool) *VariableFileInfo {
	info := &VariableFileInfo{
		Path: filePath,
	}

	// Resolve relative paths
	fullPath := filePath
	if !filepath.IsAbs(filePath) && fl.basePath != "" {
		fullPath = filepath.Join(fl.basePath, filePath)
	}

	// Get file info
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		info.Error = fmt.Sprintf("File not found: %v", err)
		return info
	}

	info.Size = fileInfo.Size()

	// Determine format
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		info.Format = "YAML"
	case ".json":
		info.Format = "JSON"
	case ".env":
		info.Format = "Environment"
	default:
		info.Format = "Auto-detect"
	}

	// Load content if requested
	if loadContent {
		variables, err := fl.LoadVariableFile(filePath)
		if err != nil {
			info.Error = err.Error()
		} else {
			info.Variables = variables
		}
	}

	return info
}

// loadEnvironmentFile loads environment variables from a .env file
func loadEnvironmentFile(filename string) ([]string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment file '%s': %w", filename, err)
	}

	vars := make([]string, 0)
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Validate format
		if !strings.Contains(line, "=") {
			return nil, fmt.Errorf("invalid format in environment file '%s' at line %d: %s", filename, lineNum+1, line)
		}

		vars = append(vars, line)
	}

	return vars, nil
}

// parseVariableString parses command-line variable strings (key=value format)
func parseVariableString(varStr string) (string, interface{}, error) {
	parts := strings.SplitN(varStr, "=", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid variable format '%s' (expected key=value)", varStr)
	}

	key, value := parts[0], parts[1]

	// Try to parse as different types
	parsed := parseValue(value)
	return key, parsed, nil
}

// parseValue attempts to parse a string value as different types
func parseValue(value string) interface{} {
	// Try boolean
	if lower := strings.ToLower(value); lower == "true" || lower == "false" {
		return lower == "true"
	}

	// Try integer
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}

	// Try float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}

	// Return as string
	return value
}