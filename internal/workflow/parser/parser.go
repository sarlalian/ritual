// ABOUTME: YAML parser for workflow definitions (incantations)
// ABOUTME: Handles parsing, validation, and type conversion of workflow files

package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	"github.com/sarlalian/ritual/pkg/types"
)

// Parser implements the workflow parser interface
type Parser struct {
	fs afero.Fs
}

// New creates a new workflow parser
func New(fs afero.Fs) types.Parser {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return &Parser{fs: fs}
}

// Parse parses a workflow from YAML bytes
func (p *Parser) Parse(data []byte) (*types.Workflow, error) {
	var workflow types.Workflow

	// Parse YAML with strict mode to catch typos
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)

	if err := decoder.Decode(&workflow); err != nil {
		return nil, types.NewParseError("", 0, 0, "failed to parse YAML", err)
	}

	// Set defaults
	if err := p.setDefaults(&workflow); err != nil {
		return nil, err
	}

	// Validate the workflow
	if err := p.Validate(&workflow); err != nil {
		return nil, err
	}

	return &workflow, nil
}

// ParseFile parses a workflow from a file
func (p *Parser) ParseFile(filename string) (*types.Workflow, error) {
	// Check if file exists
	exists, err := afero.Exists(p.fs, filename)
	if err != nil {
		return nil, types.NewParseError(filename, 0, 0, "failed to check file existence", err)
	}
	if !exists {
		return nil, types.NewParseError(filename, 0, 0, "workflow file does not exist", nil)
	}

	// Read file
	data, err := afero.ReadFile(p.fs, filename)
	if err != nil {
		return nil, types.NewParseError(filename, 0, 0, "failed to read file", err)
	}

	// Parse content
	workflow, err := p.Parse(data)
	if err != nil {
		// Add filename context to parse errors
		if parseErr, ok := err.(*types.ParseError); ok {
			parseErr.File = filename
			return nil, parseErr
		}
		return nil, types.NewParseError(filename, 0, 0, "failed to parse workflow", err)
	}

	return workflow, nil
}

// Validate validates a workflow definition
func (p *Parser) Validate(workflow *types.Workflow) error {
	if workflow.Name == "" {
		return types.NewValidationError("name", workflow.Name, "workflow name is required")
	}

	if len(workflow.Tasks) == 0 {
		return types.NewValidationError("tasks", workflow.Tasks, "workflow must have at least one task")
	}

	// Validate execution mode
	if workflow.Mode != "" && workflow.Mode != types.ParallelMode && workflow.Mode != types.SequentialMode {
		return types.NewValidationError("mode", workflow.Mode, "mode must be 'parallel' or 'sequential'")
	}

	// Create task ID map for dependency validation
	taskIDs := make(map[string]bool)
	taskNames := make(map[string]bool)

	// First pass: collect task IDs and names, validate basic task config
	for i := range workflow.Tasks {
		task := &workflow.Tasks[i]
		if err := p.validateTask(task, i); err != nil {
			return err
		}

		// Generate ID if not provided
		if task.ID == "" {
			task.ID = p.generateTaskID(task.Name, i)
		}

		// Check for duplicate IDs
		if taskIDs[task.ID] {
			return types.NewValidationError("tasks", task.ID, fmt.Sprintf("duplicate task ID: %s", task.ID))
		}
		taskIDs[task.ID] = true

		// Check for duplicate names
		if taskNames[task.Name] {
			return types.NewValidationError("tasks", task.Name, fmt.Sprintf("duplicate task name: %s", task.Name))
		}
		taskNames[task.Name] = true
	}

	// Second pass: validate dependencies
	for _, task := range workflow.Tasks {
		if err := p.validateTaskDependencies(&task, taskIDs, taskNames); err != nil {
			return err
		}
	}

	// Validate success/failure tasks
	for i, task := range workflow.OnSuccess {
		if err := p.validateTask(&task, i); err != nil {
			return fmt.Errorf("on_success[%d]: %w", i, err)
		}
	}

	for i, task := range workflow.OnFailure {
		if err := p.validateTask(&task, i); err != nil {
			return fmt.Errorf("on_failure[%d]: %w", i, err)
		}
	}

	return nil
}

// setDefaults sets default values for workflow fields
func (p *Parser) setDefaults(workflow *types.Workflow) error {
	// Set default execution mode
	if workflow.Mode == "" {
		workflow.Mode = types.ParallelMode
	}

	// Initialize maps if nil
	if workflow.Environment == nil {
		workflow.Environment = make(map[string]string)
	}
	if workflow.Variables == nil {
		workflow.Variables = make(map[string]interface{})
	}

	// Set task defaults
	for i := range workflow.Tasks {
		p.setTaskDefaults(&workflow.Tasks[i], i)
	}

	return nil
}

// validateTask validates a single task configuration
func (p *Parser) validateTask(task *types.TaskConfig, index int) error {
	if task.Name == "" {
		return types.NewValidationError("name", task.Name, fmt.Sprintf("task[%d] name is required", index))
	}

	// Determine task type from config keys
	taskType := p.inferTaskType(task)
	if taskType == "" {
		return types.NewValidationError("type", task.Config, fmt.Sprintf("task[%d] '%s' must specify a task type", index, task.Name))
	}

	task.Type = taskType

	// Validate retry configuration
	if task.RetryCount < 0 {
		return types.NewValidationError("retry_count", task.RetryCount, fmt.Sprintf("task[%d] '%s' retry_count cannot be negative", index, task.Name))
	}

	return nil
}

// validateTaskDependencies validates task dependency references
func (p *Parser) validateTaskDependencies(task *types.TaskConfig, taskIDs, taskNames map[string]bool) error {
	for _, dep := range task.DependsOn {
		if !taskIDs[dep] && !taskNames[dep] {
			return types.NewDependencyError(task.ID, task.DependsOn, fmt.Sprintf("dependency '%s' not found", dep))
		}
	}
	return nil
}

// setTaskDefaults sets default values for a task
func (p *Parser) setTaskDefaults(task *types.TaskConfig, index int) {
	// Generate ID if not provided
	if task.ID == "" {
		task.ID = p.generateTaskID(task.Name, index)
	}

	// Set required to true by default
	if task.Required == nil {
		defaultRequired := true
		task.Required = &defaultRequired
	}

	// Initialize config map if nil
	if task.Config == nil {
		task.Config = make(map[string]interface{})
	}
}

// generateTaskID generates a unique task ID from name and index
func (p *Parser) generateTaskID(name string, index int) string {
	// Convert name to snake_case and use as ID
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")

	// Remove non-alphanumeric characters except underscores
	var result strings.Builder
	for _, char := range id {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			result.WriteRune(char)
		}
	}

	finalID := result.String()
	if finalID == "" {
		finalID = fmt.Sprintf("task_%d", index)
	}

	return finalID
}

// inferTaskType infers the task type from the configuration
func (p *Parser) inferTaskType(task *types.TaskConfig) string {
	// Check for explicit type field
	if task.Type != "" {
		return task.Type
	}

	// Infer from top-level task keys (Ansible-style)
	for key := range task.Config {
		switch key {
		case "command":
			return "command"
		case "file":
			return "file"
		case "compress":
			return "compress"
		case "checksum":
			return "checksum"
		case "email":
			return "email"
		case "slack":
			return "slack"
		case "ssh":
			return "ssh"
		}
	}

	return ""
}

// ParseString is a convenience function for parsing YAML strings
func ParseString(yamlContent string) (*types.Workflow, error) {
	parser := New(nil)
	return parser.Parse([]byte(yamlContent))
}

// ParseFileFromPath is a convenience function for parsing files
func ParseFileFromPath(filename string) (*types.Workflow, error) {
	parser := New(nil)
	return parser.ParseFile(filename)
}

// ValidateFileStructure checks if a file appears to be a valid workflow
func ValidateFileStructure(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".yaml" && ext != ".yml" {
		return fmt.Errorf("workflow files must have .yaml or .yml extension")
	}

	// Check if file is readable
	if _, err := os.Stat(filename); err != nil {
		return fmt.Errorf("cannot access workflow file: %w", err)
	}

	return nil
}
