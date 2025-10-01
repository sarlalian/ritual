// ABOUTME: Import resolver for workflow composition and reusable workflow libraries
// ABOUTME: Handles importing external workflows and merging them into the main workflow

package imports

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/sarlalian/ritual/pkg/types"
)

// Resolver handles importing and resolving external workflows
type Resolver struct {
	fs       afero.Fs
	parser   types.Parser
	cache    map[string]*types.Workflow
	basePath string
	maxDepth int
}

// Config holds configuration for the import resolver
type Config struct {
	FileSystem afero.Fs
	Parser     types.Parser
	BasePath   string
	MaxDepth   int // Maximum import depth to prevent circular imports
}

// New creates a new import resolver
func New(config *Config) *Resolver {
	if config == nil {
		config = &Config{}
	}

	if config.FileSystem == nil {
		config.FileSystem = afero.NewOsFs()
	}

	if config.MaxDepth == 0 {
		config.MaxDepth = 10 // Default maximum import depth
	}

	return &Resolver{
		fs:       config.FileSystem,
		parser:   config.Parser,
		cache:    make(map[string]*types.Workflow),
		basePath: config.BasePath,
		maxDepth: config.MaxDepth,
	}
}

// ResolveImports processes all imports in a workflow and returns the resolved workflow
func (r *Resolver) ResolveImports(ctx context.Context, workflow *types.Workflow, workflowPath string) (*types.Workflow, error) {
	if len(workflow.Imports) == 0 {
		return workflow, nil // No imports to resolve
	}

	// Set base path from workflow location if not set
	if r.basePath == "" && workflowPath != "" {
		r.basePath = filepath.Dir(workflowPath)
	}

	// Resolve all imports
	importedWorkflows := make(map[string]*types.Workflow)
	for _, importPath := range workflow.Imports {
		resolvedPath := r.resolveImportPath(importPath, workflowPath)

		imported, err := r.resolveImport(ctx, resolvedPath, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve import '%s': %w", importPath, err)
		}

		importName := r.getImportName(importPath)
		importedWorkflows[importName] = imported
	}

	// Merge imports into the main workflow
	mergedWorkflow, err := r.mergeWorkflows(workflow, importedWorkflows)
	if err != nil {
		return nil, fmt.Errorf("failed to merge imported workflows: %w", err)
	}

	return mergedWorkflow, nil
}

// Resolve resolves a single import path to a workflow
func (r *Resolver) Resolve(ctx context.Context, importPath string) (*types.Workflow, error) {
	return r.resolveImport(ctx, importPath, 0)
}

// ResolveAll resolves multiple import paths
func (r *Resolver) ResolveAll(ctx context.Context, importPaths []string) (map[string]*types.Workflow, error) {
	resolved := make(map[string]*types.Workflow)

	for _, importPath := range importPaths {
		workflow, err := r.resolveImport(ctx, importPath, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve import '%s': %w", importPath, err)
		}

		importName := r.getImportName(importPath)
		resolved[importName] = workflow
	}

	return resolved, nil
}

// resolveImport recursively resolves an import with depth tracking
func (r *Resolver) resolveImport(ctx context.Context, importPath string, depth int) (*types.Workflow, error) {
	if depth > r.maxDepth {
		return nil, fmt.Errorf("maximum import depth (%d) exceeded, possible circular import: %s", r.maxDepth, importPath)
	}

	// Check cache first
	if cached, exists := r.cache[importPath]; exists {
		return cached, nil
	}

	// Parse the imported workflow
	workflow, err := r.parser.ParseFile(importPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imported workflow '%s': %w", importPath, err)
	}

	// Recursively resolve imports in the imported workflow
	if len(workflow.Imports) > 0 {
		resolvedWorkflow, err := r.ResolveImports(ctx, workflow, importPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve nested imports in '%s': %w", importPath, err)
		}
		workflow = resolvedWorkflow
	}

	// Cache the resolved workflow
	r.cache[importPath] = workflow

	return workflow, nil
}

// resolveImportPath converts a relative import path to an absolute path
func (r *Resolver) resolveImportPath(importPath, workflowPath string) string {
	if filepath.IsAbs(importPath) {
		return importPath
	}

	// If we have a workflow path, resolve relative to it
	if workflowPath != "" {
		return filepath.Join(filepath.Dir(workflowPath), importPath)
	}

	// Otherwise resolve relative to base path
	if r.basePath != "" {
		return filepath.Join(r.basePath, importPath)
	}

	return importPath
}

// getImportName extracts a name from an import path for referencing
func (r *Resolver) getImportName(importPath string) string {
	base := filepath.Base(importPath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// mergeWorkflows merges imported workflows into the main workflow
func (r *Resolver) mergeWorkflows(main *types.Workflow, imports map[string]*types.Workflow) (*types.Workflow, error) {
	merged := *main // Create a copy

	// Merge environment variables (main workflow takes precedence)
	if merged.Environment == nil {
		merged.Environment = make(map[string]string)
	}

	for importName, imported := range imports {
		if imported.Environment != nil {
			for key, value := range imported.Environment {
				// Prefix imported environment variables with import name
				prefixedKey := fmt.Sprintf("%s_%s", strings.ToUpper(importName), key)
				if _, exists := merged.Environment[prefixedKey]; !exists {
					merged.Environment[prefixedKey] = value
				}
			}
		}
	}

	// Merge workflow variables (main workflow takes precedence)
	if merged.Variables == nil {
		merged.Variables = make(map[string]interface{})
	}

	for importName, imported := range imports {
		if imported.Variables != nil {
			// Create a nested structure for imported variables
			merged.Variables[importName] = imported.Variables
		}
	}

	// Merge tasks (imported tasks get prefixed with import name)
	originalTaskCount := len(merged.Tasks)
	for importName, imported := range imports {
		for _, task := range imported.Tasks {
			importedTask := r.prefixTaskIDs(task, importName)
			merged.Tasks = append(merged.Tasks, importedTask)
		}
	}

	// Update dependencies to handle prefixed task IDs
	for i := originalTaskCount; i < len(merged.Tasks); i++ {
		task := &merged.Tasks[i]
		for j, dep := range task.DependsOn {
			// Check if this dependency refers to an imported task
			if !r.taskExistsInMain(dep, main.Tasks[:originalTaskCount]) {
				// Try to find it in the same import
				importName := r.getTaskImportName(task.ID)
				if importName != "" {
					task.DependsOn[j] = fmt.Sprintf("%s_%s", importName, dep)
				}
			}
		}
	}

	return &merged, nil
}

// prefixTaskIDs prefixes task IDs and names with the import name
func (r *Resolver) prefixTaskIDs(task types.TaskConfig, importName string) types.TaskConfig {
	prefixed := task // Copy the task

	// Prefix the task ID
	if prefixed.ID != "" {
		prefixed.ID = fmt.Sprintf("%s_%s", importName, prefixed.ID)
	}

	// Prefix dependencies (will be resolved later in mergeWorkflows)
	// Dependencies within the same import will be handled by the dependency update logic

	return prefixed
}

// taskExistsInMain checks if a task ID exists in the main workflow's original tasks
func (r *Resolver) taskExistsInMain(taskID string, mainTasks []types.TaskConfig) bool {
	for _, task := range mainTasks {
		if task.ID == taskID {
			return true
		}
	}
	return false
}

// getTaskImportName extracts the import name from a prefixed task ID
func (r *Resolver) getTaskImportName(taskID string) string {
	parts := strings.Split(taskID, "_")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

// Clear clears the import cache
func (r *Resolver) Clear() {
	r.cache = make(map[string]*types.Workflow)
}
