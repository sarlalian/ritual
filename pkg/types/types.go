// ABOUTME: Core types and interfaces for the Ritual workflow engine
// ABOUTME: Defines fundamental data structures used throughout the application

package types

import (
	"context"
	"fmt"
	"time"
)

// ExecutionMode defines how tasks are executed within a workflow
type ExecutionMode string

const (
	// ParallelMode executes tasks concurrently when dependencies allow (default)
	ParallelMode ExecutionMode = "parallel"
	// SequentialMode executes all tasks one after another
	SequentialMode ExecutionMode = "sequential"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	// TaskPending indicates the task hasn't started yet
	TaskPending TaskStatus = "pending"
	// TaskRunning indicates the task is currently executing
	TaskRunning TaskStatus = "running"
	// TaskSuccess indicates the task completed successfully
	TaskSuccess TaskStatus = "success"
	// TaskWarning indicates the task completed with warnings
	TaskWarning TaskStatus = "warning"
	// TaskFailed indicates the task failed to complete
	TaskFailed TaskStatus = "failed"
	// TaskSkipped indicates the task was skipped due to conditions
	TaskSkipped TaskStatus = "skipped"
)

// WorkflowStatus represents the overall state of a workflow
type WorkflowStatus string

const (
	// WorkflowPending indicates the workflow hasn't started
	WorkflowPending WorkflowStatus = "pending"
	// WorkflowRunning indicates the workflow is currently executing
	WorkflowRunning WorkflowStatus = "running"
	// WorkflowSuccess indicates all required tasks completed successfully
	WorkflowSuccess WorkflowStatus = "success"
	// WorkflowPartialSuccess indicates some non-required tasks failed
	WorkflowPartialSuccess WorkflowStatus = "partial_success"
	// WorkflowFailed indicates one or more required tasks failed
	WorkflowFailed WorkflowStatus = "failed"
)

// Concurrency constraints for workflow execution
const (
	// MinConcurrency is the minimum allowed concurrent task execution
	MinConcurrency = 1
	// MaxConcurrency is the maximum allowed concurrent task execution
	MaxConcurrency = 256
	// DefaultConcurrency is the default number of concurrent tasks
	DefaultConcurrency = 10
)

// Workflow represents a complete workflow definition
type Workflow struct {
	Name          string                 `yaml:"name" json:"name"`
	Version       string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Description   string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Mode          ExecutionMode          `yaml:"mode,omitempty" json:"mode,omitempty"`
	Environment   map[string]string      `yaml:"environment,omitempty" json:"environment,omitempty"`
	Imports       []string               `yaml:"imports,omitempty" json:"imports,omitempty"`
	VariableFiles []string               `yaml:"variable_files,omitempty" json:"variable_files,omitempty"`
	Variables     map[string]interface{} `yaml:"vars,omitempty" json:"vars,omitempty"`
	Tasks         []TaskConfig           `yaml:"tasks" json:"tasks"`
	OnSuccess     []TaskConfig           `yaml:"on_success,omitempty" json:"on_success,omitempty"`
	OnFailure     []TaskConfig           `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`
}

// TaskConfig represents a task definition in the workflow
type TaskConfig struct {
	ID         string                 `yaml:"id,omitempty" json:"id,omitempty"`
	Name       string                 `yaml:"name" json:"name"`
	Type       string                 `yaml:"type,omitempty" json:"type,omitempty"`
	Config     map[string]interface{} `yaml:",inline" json:"config"`
	DependsOn  []string               `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	When       string                 `yaml:"when,omitempty" json:"when,omitempty"`
	Required   *bool                  `yaml:"required,omitempty" json:"required,omitempty"`
	AlwaysRun  bool                   `yaml:"always_run,omitempty" json:"always_run,omitempty"`
	Register   string                 `yaml:"register,omitempty" json:"register,omitempty"`
	RetryCount int                    `yaml:"retry_count,omitempty" json:"retry_count,omitempty"`
	RetryDelay time.Duration          `yaml:"retry_delay,omitempty" json:"retry_delay,omitempty"`
}

// IsRequired returns whether this task is required for workflow success
func (tc *TaskConfig) IsRequired() bool {
	if tc.Required == nil {
		return true // Tasks are required by default
	}
	return *tc.Required
}

// TaskResult represents the result of executing a task
type TaskResult struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Status       TaskStatus             `json:"status"`
	Message      string                 `json:"message,omitempty"`
	Output       map[string]interface{} `json:"output,omitempty"`
	Stdout       string                 `json:"stdout,omitempty"`
	Stderr       string                 `json:"stderr,omitempty"`
	ReturnCode   int                    `json:"return_code,omitempty"`
	Error        string                 `json:"error,omitempty"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Duration     time.Duration          `json:"duration"`
	AttemptCount int                    `json:"attempt_count"`
}

// WorkflowResult represents the overall result of executing a workflow
type WorkflowResult struct {
	Name      string                 `json:"name"`
	Status    WorkflowStatus         `json:"status"`
	Tasks     map[string]*TaskResult `json:"tasks"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  time.Duration          `json:"duration"`
	Error     string                 `json:"error,omitempty"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowContext holds the execution context for a workflow
type WorkflowContext struct {
	Environment map[string]string      // Environment variables
	Variables   map[string]interface{} // Workflow variables
	Tasks       map[string]*TaskResult // Task results by ID/name
	Imports     map[string]*Workflow   // Imported workflows by name
	Metadata    map[string]interface{} // Additional metadata
}

// NewWorkflowContext creates a new workflow context
func NewWorkflowContext() *WorkflowContext {
	return &WorkflowContext{
		Environment: make(map[string]string),
		Variables:   make(map[string]interface{}),
		Tasks:       make(map[string]*TaskResult),
		Imports:     make(map[string]*Workflow),
		Metadata:    make(map[string]interface{}),
	}
}

// Task represents an executable task
type Task interface {
	// ID returns the unique identifier for this task
	ID() string

	// Name returns the human-readable name for this task
	Name() string

	// Type returns the task type identifier
	Type() string

	// Execute runs the task with the given context
	Execute(ctx context.Context, wCtx *WorkflowContext) (*TaskResult, error)

	// Validate checks if the task configuration is valid
	Validate() error
}

// Executor executes workflows
type Executor interface {
	// Execute runs a workflow and returns the result
	Execute(ctx context.Context, workflow *Workflow, wCtx *WorkflowContext) (*WorkflowResult, error)
}

// Parser parses workflow definitions from YAML
type Parser interface {
	// Parse parses a workflow from YAML bytes
	Parse(data []byte) (*Workflow, error)

	// ParseFile parses a workflow from a file
	ParseFile(filename string) (*Workflow, error)

	// Validate validates a workflow definition
	Validate(workflow *Workflow) error
}

// TemplateEngine evaluates templates within workflow configurations
type TemplateEngine interface {
	// Evaluate evaluates a template string with the given context
	Evaluate(template string, ctx *WorkflowContext) (string, error)

	// EvaluateAll evaluates all template strings in a map
	EvaluateAll(data map[string]interface{}, ctx *WorkflowContext) (map[string]interface{}, error)
}

// ImportResolver resolves and loads imported workflows
type ImportResolver interface {
	// Resolve resolves an import path to a workflow
	Resolve(ctx context.Context, importPath string) (*Workflow, error)

	// ResolveAll resolves multiple import paths
	ResolveAll(ctx context.Context, importPaths []string) (map[string]*Workflow, error)
}

// EventFilter defines filtering rules for events
type EventFilter struct {
	Include []string `yaml:"include,omitempty" json:"include,omitempty"` // Patterns that must match
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"` // Patterns that must not match
}

// EventListener listens for external events and triggers workflows
type EventListener interface {
	// Start starts listening for events
	Start(ctx context.Context) error

	// Stop stops listening for events
	Stop() error

	// SetWorkflowTrigger sets the function to call when an event should trigger a workflow
	SetWorkflowTrigger(trigger func(workflowPath string, context map[string]interface{}) error)
}

// Result represents the complete result of workflow execution
type Result struct {
	WorkflowResult   *WorkflowResult `json:"workflow_result,omitempty"`
	ValidationErrors []error         `json:"validation_errors,omitempty"`
	ParseError       error           `json:"parse_error,omitempty"`
	DependencyError  error           `json:"dependency_error,omitempty"`
	ExecutionError   error           `json:"execution_error,omitempty"`
}

// OutputHandler handles workflow output
type OutputHandler interface {
	// Handle processes a workflow result and outputs it to the configured destination
	Handle(ctx context.Context, result *WorkflowResult) error
}

// TaskExecutor executes individual tasks
type TaskExecutor interface {
	// Execute runs a single task and returns the result
	Execute(ctx context.Context, task *TaskConfig, contextManager ContextManager) *TaskResult

	// Validate checks if the task configuration is valid
	Validate(task *TaskConfig) error

	// SupportsDryRun indicates if this executor supports dry-run mode
	SupportsDryRun() bool
}

// ContextManager manages workflow execution context and variable resolution
type ContextManager interface {
	// Initialize sets up the initial context from workflow and environment
	Initialize(workflow *Workflow, envVars []string) error

	// GetContext returns the current workflow context
	GetContext() *WorkflowContext

	// GetVariable returns a variable value with template evaluation
	GetVariable(name string) (interface{}, error)

	// SetVariable sets a workflow variable
	SetVariable(name string, value interface{}) error

	// GetEnvironment returns an environment variable with fallback
	GetEnvironment(name, defaultValue string) string

	// SetEnvironment sets an environment variable
	SetEnvironment(name, value string) error

	// RegisterTaskResult registers a task result for use in templates
	RegisterTaskResult(taskResult *TaskResult) error

	// GetTaskResult returns a task result by ID or name
	GetTaskResult(identifier string) (*TaskResult, error)

	// EvaluateString evaluates a string template with the current context
	EvaluateString(templateStr string) (string, error)

	// EvaluateMap evaluates all template strings in a map
	EvaluateMap(data map[string]interface{}) (map[string]interface{}, error)

	// GetTemplateEngine returns the template engine used by this context manager
	GetTemplateEngine() TemplateEngine

	// Clone creates a copy of the context manager for isolated execution
	Clone() ContextManager
}

// ExecutionLayer represents a group of tasks that can be executed in parallel
type ExecutionLayer struct {
	Tasks       []*TaskNode
	LayerNumber int
}

// TaskNode represents a task with its dependencies and metadata
type TaskNode struct {
	Task         *TaskConfig
	Dependencies []*TaskNode
	Dependents   []*TaskNode
	InDegree     int // Number of dependencies
	Layer        int // Execution layer (0 = first layer)
}

// DependencyResolver handles dependency resolution and execution planning
type DependencyResolver interface {
	// BuildGraph builds the dependency graph from workflow tasks
	BuildGraph(tasks []TaskConfig) error

	// GetExecutionLayers returns tasks organized into parallel execution layers
	GetExecutionLayers() ([]*ExecutionLayer, error)

	// GetTaskOrder returns tasks in topological order for sequential execution
	GetTaskOrder() ([]*TaskNode, error)

	// GetTasksByLayer returns all tasks in a specific execution layer
	GetTasksByLayer(layerNum int) ([]*TaskNode, error)

	// GetDependenciesFor returns all direct dependencies for a task
	GetDependenciesFor(taskID string) ([]*TaskNode, error)

	// GetDependentsFor returns all direct dependents for a task
	GetDependentsFor(taskID string) ([]*TaskNode, error)

	// ValidateGraph performs additional validation on the dependency graph
	ValidateGraph() error

	// GetStats returns statistics about the dependency graph
	GetStats() map[string]interface{}

	// Clear resets the resolver state
	Clear()
}

// Logger provides structured logging interface
type Logger interface {
	// Debug logs a debug message
	Debug() LogEvent

	// Info logs an info message
	Info() LogEvent

	// Warn logs a warning message
	Warn() LogEvent

	// Error logs an error message
	Error() LogEvent

	// With returns a logger with additional context
	With() LogContext
}

// LogEvent represents a log event being constructed
type LogEvent interface {
	// Str adds a string field
	Str(key, val string) LogEvent

	// Int adds an integer field
	Int(key string, val int) LogEvent

	// Dur adds a duration field
	Dur(key string, val time.Duration) LogEvent

	// Err adds an error field
	Err(err error) LogEvent

	// Bool adds a boolean field
	Bool(key string, val bool) LogEvent

	// Any adds an arbitrary field
	Any(key string, val interface{}) LogEvent

	// Msg logs the event with a message
	Msg(msg string)

	// Msgf logs the event with a formatted message
	Msgf(format string, args ...interface{})
}

// LogContext represents a logger context being constructed
type LogContext interface {
	// Str adds a string field to the context
	Str(key, val string) LogContext

	// Logger returns the logger with the built context
	Logger() Logger
}

// ValidateConcurrency validates a concurrency value and returns a valid value or an error.
// If value is 0, returns DefaultConcurrency.
// If value is negative or exceeds MaxConcurrency, returns an error.
func ValidateConcurrency(value int) (int, error) {
	if value == 0 {
		return DefaultConcurrency, nil
	}
	if value < MinConcurrency {
		return 0, fmt.Errorf("max_concurrency must be at least %d, got %d", MinConcurrency, value)
	}
	if value > MaxConcurrency {
		return 0, fmt.Errorf("max_concurrency cannot exceed %d, got %d", MaxConcurrency, value)
	}
	return value, nil
}
