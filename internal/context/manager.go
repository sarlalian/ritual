// ABOUTME: Context manager for workflow execution and variable resolution
// ABOUTME: Handles environment variables, workflow variables, and task result sharing

package context

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sarlalian/ritual/internal/variables"
	"github.com/sarlalian/ritual/pkg/types"
)

// Manager handles workflow context and variable resolution
type Manager struct {
	context        *types.WorkflowContext
	templateEngine types.TemplateEngine
	envOverrides   map[string]string
	variableLoader *variables.FileLoader
	workflowDir    string
	mu             sync.RWMutex // Protects concurrent access to context
}

// New creates a new context manager
func New(templateEngine types.TemplateEngine) *Manager {
	cwd, _ := os.Getwd()
	return &Manager{
		context:        types.NewWorkflowContext(),
		templateEngine: templateEngine,
		envOverrides:   make(map[string]string),
		variableLoader: variables.New(cwd),
		workflowDir:    cwd,
	}
}

// NewWithWorkflowDir creates a new context manager with a specific workflow directory
func NewWithWorkflowDir(templateEngine types.TemplateEngine, workflowDir string) *Manager {
	return &Manager{
		context:        types.NewWorkflowContext(),
		templateEngine: templateEngine,
		envOverrides:   make(map[string]string),
		variableLoader: variables.New(workflowDir),
		workflowDir:    workflowDir,
	}
}

// Initialize sets up the initial context from workflow and environment
func (m *Manager) Initialize(workflow *types.Workflow, envVars []string) error {
	// Load environment variables from system
	m.loadSystemEnvironment()

	// Apply environment variable overrides from workflow
	if err := m.loadWorkflowEnvironment(workflow.Environment); err != nil {
		return fmt.Errorf("failed to load workflow environment: %w", err)
	}

	// Apply command-line environment overrides
	if err := m.loadEnvironmentOverrides(envVars); err != nil {
		return fmt.Errorf("failed to load environment overrides: %w", err)
	}

	// Load variables from external files first
	if err := m.loadVariableFiles(workflow.VariableFiles); err != nil {
		return fmt.Errorf("failed to load variable files: %w", err)
	}

	// Load workflow variables with template evaluation (these override file variables)
	if err := m.loadWorkflowVariables(workflow.Variables); err != nil {
		return fmt.Errorf("failed to load workflow variables: %w", err)
	}

	// Set up workflow metadata
	m.setupWorkflowMetadata(workflow)

	return nil
}

// GetContext returns the current workflow context
func (m *Manager) GetContext() *types.WorkflowContext {
	return m.context
}

// GetVariable returns a variable value with template evaluation
func (m *Manager) GetVariable(name string) (interface{}, error) {
	value, exists := m.context.Variables[name]
	if !exists {
		return nil, fmt.Errorf("variable '%s' not found", name)
	}

	// Variables should already be evaluated during initialization
	// Only re-evaluate if this is a dynamic string that looks like a template
	if strValue, ok := value.(string); ok && strings.Contains(strValue, "{{") {
		evaluated, err := m.templateEngine.Evaluate(strValue, m.context)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate variable '%s': %w", name, err)
		}
		return evaluated, nil
	}

	return value, nil
}

// GetEnvironment returns an environment variable with fallback
func (m *Manager) GetEnvironment(name, defaultValue string) string {
	if value, exists := m.context.Environment[name]; exists {
		return value
	}
	if value := os.Getenv(name); value != "" {
		return value
	}
	return defaultValue
}

// SetVariable sets a workflow variable
func (m *Manager) SetVariable(name string, value interface{}) error {
	if m.context.Variables == nil {
		m.context.Variables = make(map[string]interface{})
	}

	// Evaluate templates in string values
	if strValue, ok := value.(string); ok {
		evaluated, err := m.templateEngine.Evaluate(strValue, m.context)
		if err != nil {
			return fmt.Errorf("failed to evaluate variable '%s': %w", name, err)
		}
		m.context.Variables[name] = evaluated
	} else {
		m.context.Variables[name] = value
	}

	return nil
}

// SetEnvironment sets an environment variable
func (m *Manager) SetEnvironment(name, value string) error {
	if m.context.Environment == nil {
		m.context.Environment = make(map[string]string)
	}

	// Evaluate templates in values
	evaluated, err := m.templateEngine.Evaluate(value, m.context)
	if err != nil {
		return fmt.Errorf("failed to evaluate environment variable '%s': %w", name, err)
	}

	m.context.Environment[name] = evaluated
	return nil
}

// RegisterTaskResult registers a task result for use in templates
func (m *Manager) RegisterTaskResult(taskResult *types.TaskResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.context.Tasks == nil {
		m.context.Tasks = make(map[string]*types.TaskResult)
	}

	// Register by both ID and name
	m.context.Tasks[taskResult.ID] = taskResult
	if taskResult.Name != taskResult.ID {
		m.context.Tasks[taskResult.Name] = taskResult
	}

	return nil
}

// GetTaskResult returns a task result by ID or name
func (m *Manager) GetTaskResult(identifier string) (*types.TaskResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.context.Tasks == nil {
		return nil, fmt.Errorf("no task results available")
	}

	result, exists := m.context.Tasks[identifier]
	if !exists {
		return nil, fmt.Errorf("task result '%s' not found", identifier)
	}

	return result, nil
}

// EvaluateString evaluates a string template with the current context
func (m *Manager) EvaluateString(templateStr string) (string, error) {
	return m.templateEngine.Evaluate(templateStr, m.context)
}

// EvaluateMap evaluates all template strings in a map
func (m *Manager) EvaluateMap(data map[string]interface{}) (map[string]interface{}, error) {
	return m.templateEngine.EvaluateAll(data, m.context)
}

// Clone creates a copy of the context manager for isolated execution
func (m *Manager) Clone() types.ContextManager {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &Manager{
		context:        &types.WorkflowContext{},
		templateEngine: m.templateEngine,
		envOverrides:   make(map[string]string),
	}

	// Deep copy environment
	clone.context.Environment = make(map[string]string)
	for k, v := range m.context.Environment {
		clone.context.Environment[k] = v
	}

	// Deep copy variables
	clone.context.Variables = make(map[string]interface{})
	for k, v := range m.context.Variables {
		clone.context.Variables[k] = v
	}

	// Deep copy tasks
	clone.context.Tasks = make(map[string]*types.TaskResult)
	for k, v := range m.context.Tasks {
		clone.context.Tasks[k] = v
	}

	// Deep copy metadata
	clone.context.Metadata = make(map[string]interface{})
	for k, v := range m.context.Metadata {
		clone.context.Metadata[k] = v
	}

	// Copy env overrides
	for k, v := range m.envOverrides {
		clone.envOverrides[k] = v
	}

	return clone
}

// loadSystemEnvironment loads environment variables from the system
func (m *Manager) loadSystemEnvironment() {
	if m.context.Environment == nil {
		m.context.Environment = make(map[string]string)
	}

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			m.context.Environment[parts[0]] = parts[1]
		}
	}
}

// loadWorkflowEnvironment loads environment variables defined in the workflow
func (m *Manager) loadWorkflowEnvironment(workflowEnv map[string]string) error {
	if workflowEnv == nil {
		return nil
	}

	// Process environment variables in multiple passes to handle dependencies
	processed := make(map[string]bool)
	maxPasses := 5 // Prevent infinite loops

	for pass := 0; pass < maxPasses; pass++ {
		progressMade := false

		for key, value := range workflowEnv {
			if processed[key] {
				continue
			}

			// Try to evaluate this environment variable
			evaluated, err := m.templateEngine.Evaluate(value, m.context)
			if err != nil {
				// If evaluation fails, skip this pass and try later
				continue
			}

			m.context.Environment[key] = evaluated
			processed[key] = true
			progressMade = true
		}

		// If no progress was made, store remaining variables without templates
		if !progressMade {
			for key, value := range workflowEnv {
				if !processed[key] {
					m.context.Environment[key] = value
					processed[key] = true
				}
			}
			break
		}

		// Check if all environment variables are processed
		allProcessed := true
		for key := range workflowEnv {
			if !processed[key] {
				allProcessed = false
				break
			}
		}
		if allProcessed {
			break
		}
	}

	return nil
}

// loadEnvironmentOverrides loads environment variables from command line (key=value format)
func (m *Manager) loadEnvironmentOverrides(envVars []string) error {
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid environment variable format '%s' (expected key=value)", envVar)
		}

		key, value := parts[0], parts[1]

		// Store override for reference
		m.envOverrides[key] = value

		// Evaluate template
		evaluated, err := m.templateEngine.Evaluate(value, m.context)
		if err != nil {
			return fmt.Errorf("failed to evaluate environment override '%s': %w", key, err)
		}

		m.context.Environment[key] = evaluated
	}

	return nil
}

// loadWorkflowVariables loads variables defined in the workflow
func (m *Manager) loadWorkflowVariables(workflowVars map[string]interface{}) error {
	if workflowVars == nil {
		return nil
	}

	if m.context.Variables == nil {
		m.context.Variables = make(map[string]interface{})
	}

	// Process variables in multiple passes to handle dependencies
	processed := make(map[string]bool)
	maxPasses := 5 // Prevent infinite loops

	for pass := 0; pass < maxPasses; pass++ {
		progressMade := false

		for key, value := range workflowVars {
			if processed[key] {
				continue
			}

			// Try to process this variable
			if strValue, ok := value.(string); ok {
				evaluated, err := m.templateEngine.Evaluate(strValue, m.context)
				if err != nil {
					// If evaluation fails, skip this pass and try later
					continue
				}
				m.context.Variables[key] = evaluated
			} else {
				m.context.Variables[key] = value
			}

			processed[key] = true
			progressMade = true
		}

		// If no progress was made, try to process remaining variables without templates
		if !progressMade {
			for key, value := range workflowVars {
				if !processed[key] {
					m.context.Variables[key] = value
					processed[key] = true
				}
			}
			break
		}

		// Check if all variables are processed
		allProcessed := true
		for key := range workflowVars {
			if !processed[key] {
				allProcessed = false
				break
			}
		}
		if allProcessed {
			break
		}
	}

	return nil
}

// setupWorkflowMetadata sets up workflow metadata in the context
func (m *Manager) setupWorkflowMetadata(workflow *types.Workflow) {
	if m.context.Metadata == nil {
		m.context.Metadata = make(map[string]interface{})
	}

	workflowMeta := map[string]interface{}{
		"name":        workflow.Name,
		"version":     workflow.Version,
		"description": workflow.Description,
		"mode":        workflow.Mode,
		"task_count":  len(workflow.Tasks),
	}

	m.context.Metadata["workflow"] = workflowMeta
}

// ParseVariableString parses command-line variable strings (key=value format)
func ParseVariableString(varStr string) (string, interface{}, error) {
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

// LoadEnvironmentFile loads environment variables from a .env file
func LoadEnvironmentFile(filename string) ([]string, error) {
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

// loadVariableFiles loads variables from external files
func (m *Manager) loadVariableFiles(variableFiles []string) error {
	if len(variableFiles) == 0 {
		return nil
	}

	if m.context.Variables == nil {
		m.context.Variables = make(map[string]interface{})
	}

	// Process each variable file in order
	for _, filePath := range variableFiles {
		// Evaluate template in file path (allows dynamic file selection)
		evaluatedPath, err := m.templateEngine.Evaluate(filePath, m.context)
		if err != nil {
			return fmt.Errorf("failed to evaluate variable file path '%s': %w", filePath, err)
		}

		// Load variables from file
		fileVars, err := m.variableLoader.LoadVariableFile(evaluatedPath)
		if err != nil {
			return fmt.Errorf("failed to load variable file '%s': %w", evaluatedPath, err)
		}

		// Resolve any file references within the loaded variables
		resolvedVars, err := m.variableLoader.ResolveVariableReferences(fileVars)
		if err != nil {
			return fmt.Errorf("failed to resolve variable references in file '%s': %w", evaluatedPath, err)
		}

		// Merge variables into context (later files override earlier ones)
		for key, value := range resolvedVars {
			// Evaluate string templates in loaded variables
			if strValue, ok := value.(string); ok {
				evaluated, err := m.templateEngine.Evaluate(strValue, m.context)
				if err != nil {
					// If template evaluation fails, use the raw value
					m.context.Variables[key] = strValue
				} else {
					m.context.Variables[key] = evaluated
				}
			} else {
				m.context.Variables[key] = value
			}
		}
	}

	return nil
}
// GetTemplateEngine returns the template engine used by this context manager
func (m *Manager) GetTemplateEngine() types.TemplateEngine {
	return m.templateEngine
}
