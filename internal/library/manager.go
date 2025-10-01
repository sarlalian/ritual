// ABOUTME: Workflow library management for discovering, validating, and organizing workflow libraries
// ABOUTME: Provides functionality to manage reusable workflow components and task libraries

package library

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sarlalian/ritual/pkg/types"
)

// Manager handles workflow library management and discovery
type Manager struct {
	libraryDirs []string
	parser      types.Parser
}

// LibraryInfo represents information about a workflow library
type LibraryInfo struct {
	Name        string                 `json:"name"`
	Path        string                 `json:"path"`
	Version     string                 `json:"version,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tasks       []TaskInfo             `json:"tasks"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Environment map[string]string      `json:"environment,omitempty"`
	Size        int64                  `json:"size"`
	ModTime     time.Time              `json:"mod_time"`
	IsValid     bool                   `json:"is_valid"`
	Error       string                 `json:"error,omitempty"`
}

// TaskInfo represents information about a task in a library
type TaskInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

// LibraryRegistry holds information about all discovered libraries
type LibraryRegistry struct {
	Libraries    []*LibraryInfo          `json:"libraries"`
	TaskIndex    map[string][]*TaskInfo  `json:"task_index"`    // Task type -> list of tasks
	LibraryIndex map[string]*LibraryInfo `json:"library_index"` // Library name -> library info
	LastScanned  time.Time               `json:"last_scanned"`
	ScanDuration time.Duration           `json:"scan_duration"`
}

// New creates a new library manager
func New(libraryDirs []string, parser types.Parser) *Manager {
	return &Manager{
		libraryDirs: libraryDirs,
		parser:      parser,
	}
}

// ScanLibraries scans all configured library directories for workflows
func (m *Manager) ScanLibraries() (*LibraryRegistry, error) {
	startTime := time.Now()
	registry := &LibraryRegistry{
		Libraries:    make([]*LibraryInfo, 0),
		TaskIndex:    make(map[string][]*TaskInfo),
		LibraryIndex: make(map[string]*LibraryInfo),
		LastScanned:  startTime,
	}

	for _, dir := range m.libraryDirs {
		if err := m.scanDirectory(dir, registry); err != nil {
			return nil, fmt.Errorf("failed to scan library directory '%s': %w", dir, err)
		}
	}

	// Build indexes
	m.buildIndexes(registry)

	registry.ScanDuration = time.Since(startTime)
	return registry, nil
}

// scanDirectory recursively scans a directory for workflow libraries
func (m *Manager) scanDirectory(dir string, registry *LibraryRegistry) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Skip non-existent directories
	}

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip non-YAML files
		if d.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		// Create library info
		info := &LibraryInfo{
			Path: path,
		}

		// Get file info
		fileInfo, err := d.Info()
		if err != nil {
			info.Error = fmt.Sprintf("Failed to get file info: %v", err)
			registry.Libraries = append(registry.Libraries, info)
			return nil
		}

		info.Size = fileInfo.Size()
		info.ModTime = fileInfo.ModTime()

		// Parse workflow
		workflow, err := m.parser.ParseFile(path)
		if err != nil {
			info.Error = fmt.Sprintf("Failed to parse workflow: %v", err)
			registry.Libraries = append(registry.Libraries, info)
			return nil
		}

		// Extract library information
		info.Name = workflow.Name
		info.Version = workflow.Version
		info.Description = workflow.Description
		info.Variables = workflow.Variables
		info.Environment = workflow.Environment
		info.IsValid = true

		// Extract task information
		info.Tasks = make([]TaskInfo, len(workflow.Tasks))
		for i, task := range workflow.Tasks {
			info.Tasks[i] = TaskInfo{
				ID:       task.ID,
				Name:     task.Name,
				Type:     task.Type,
				Required: task.IsRequired(),
			}

			// Try to extract description from task config
			if desc, ok := task.Config["description"].(string); ok {
				info.Tasks[i].Description = desc
			}
		}

		registry.Libraries = append(registry.Libraries, info)
		return nil
	})
}

// buildIndexes builds search indexes for the library registry
func (m *Manager) buildIndexes(registry *LibraryRegistry) {
	for _, lib := range registry.Libraries {
		if !lib.IsValid {
			continue
		}

		// Index by library name
		registry.LibraryIndex[lib.Name] = lib

		// Index tasks by type
		for _, task := range lib.Tasks {
			if task.Type == "" {
				continue
			}

			taskInfo := task // Copy for reference
			registry.TaskIndex[task.Type] = append(registry.TaskIndex[task.Type], &taskInfo)
		}
	}

	// Sort task indexes
	for taskType := range registry.TaskIndex {
		sort.Slice(registry.TaskIndex[taskType], func(i, j int) bool {
			return registry.TaskIndex[taskType][i].Name < registry.TaskIndex[taskType][j].Name
		})
	}
}

// FindLibrary finds a library by name
func (m *Manager) FindLibrary(registry *LibraryRegistry, name string) *LibraryInfo {
	return registry.LibraryIndex[name]
}

// FindTasksByType finds all tasks of a specific type across all libraries
func (m *Manager) FindTasksByType(registry *LibraryRegistry, taskType string) []*TaskInfo {
	return registry.TaskIndex[taskType]
}

// GetLibraryStats returns statistics about the library registry
func (m *Manager) GetLibraryStats(registry *LibraryRegistry) map[string]interface{} {
	validLibraries := 0
	invalidLibraries := 0
	totalTasks := 0
	taskTypes := make(map[string]int)

	for _, lib := range registry.Libraries {
		if lib.IsValid {
			validLibraries++
			totalTasks += len(lib.Tasks)

			for _, task := range lib.Tasks {
				if task.Type != "" {
					taskTypes[task.Type]++
				}
			}
		} else {
			invalidLibraries++
		}
	}

	return map[string]interface{}{
		"total_libraries":   len(registry.Libraries),
		"valid_libraries":   validLibraries,
		"invalid_libraries": invalidLibraries,
		"total_tasks":       totalTasks,
		"task_types":        len(taskTypes),
		"task_type_counts":  taskTypes,
		"last_scanned":      registry.LastScanned,
		"scan_duration":     registry.ScanDuration,
	}
}

// SearchLibraries searches libraries by name or description
func (m *Manager) SearchLibraries(registry *LibraryRegistry, query string) []*LibraryInfo {
	var results []*LibraryInfo
	query = strings.ToLower(query)

	for _, lib := range registry.Libraries {
		if !lib.IsValid {
			continue
		}

		// Search in name and description
		if strings.Contains(strings.ToLower(lib.Name), query) ||
			strings.Contains(strings.ToLower(lib.Description), query) {
			results = append(results, lib)
		}
	}

	// Sort by name
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

// SearchTasks searches tasks by name, type, or description
func (m *Manager) SearchTasks(registry *LibraryRegistry, query string) map[string][]*TaskInfo {
	results := make(map[string][]*TaskInfo)
	query = strings.ToLower(query)

	for _, lib := range registry.Libraries {
		if !lib.IsValid {
			continue
		}

		for _, task := range lib.Tasks {
			// Search in task name, type, and description
			if strings.Contains(strings.ToLower(task.Name), query) ||
				strings.Contains(strings.ToLower(task.Type), query) ||
				strings.Contains(strings.ToLower(task.Description), query) {

				taskInfo := task // Copy for reference
				results[lib.Name] = append(results[lib.Name], &taskInfo)
			}
		}
	}

	return results
}

// ValidateLibrary validates a specific library file
func (m *Manager) ValidateLibrary(path string) (*LibraryInfo, error) {
	info := &LibraryInfo{
		Path: path,
	}

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		return info, fmt.Errorf("failed to get file info: %w", err)
	}

	info.Size = fileInfo.Size()
	info.ModTime = fileInfo.ModTime()

	// Parse workflow
	workflow, err := m.parser.ParseFile(path)
	if err != nil {
		info.Error = fmt.Sprintf("Failed to parse workflow: %v", err)
		return info, err
	}

	// Validate workflow structure
	if err := m.parser.Validate(workflow); err != nil {
		info.Error = fmt.Sprintf("Workflow validation failed: %v", err)
		return info, err
	}

	// Extract library information
	info.Name = workflow.Name
	info.Version = workflow.Version
	info.Description = workflow.Description
	info.Variables = workflow.Variables
	info.Environment = workflow.Environment
	info.IsValid = true

	// Extract task information
	info.Tasks = make([]TaskInfo, len(workflow.Tasks))
	for i, task := range workflow.Tasks {
		info.Tasks[i] = TaskInfo{
			ID:       task.ID,
			Name:     task.Name,
			Type:     task.Type,
			Required: task.IsRequired(),
		}

		// Try to extract description from task config
		if desc, ok := task.Config["description"].(string); ok {
			info.Tasks[i].Description = desc
		}
	}

	return info, nil
}

// ListTaskTypes returns all unique task types found in the registry
func (m *Manager) ListTaskTypes(registry *LibraryRegistry) []string {
	types := make([]string, 0, len(registry.TaskIndex))
	for taskType := range registry.TaskIndex {
		types = append(types, taskType)
	}
	sort.Strings(types)
	return types
}

// GetLibraryDependencies analyzes library dependencies (imports)
func (m *Manager) GetLibraryDependencies(registry *LibraryRegistry, libraryName string) ([]string, error) {
	lib := registry.LibraryIndex[libraryName]
	if lib == nil {
		return nil, fmt.Errorf("library '%s' not found", libraryName)
	}

	// Re-parse the workflow to get import information
	workflow, err := m.parser.ParseFile(lib.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to re-parse library: %w", err)
	}

	return workflow.Imports, nil
}
