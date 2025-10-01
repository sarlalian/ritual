// ABOUTME: Workflow orchestrator that coordinates all components for end-to-end execution
// ABOUTME: Integrates parser, resolver, context manager, executor, and task registry

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/afero"

	contextManager "github.com/sarlalian/ritual/internal/context"
	"github.com/sarlalian/ritual/internal/executor"
	"github.com/sarlalian/ritual/internal/history"
	"github.com/sarlalian/ritual/internal/tasks"
	"github.com/sarlalian/ritual/internal/template"
	"github.com/sarlalian/ritual/internal/workflow/imports"
	"github.com/sarlalian/ritual/internal/workflow/parser"
	"github.com/sarlalian/ritual/internal/workflow/resolver"
	"github.com/sarlalian/ritual/pkg/types"
)

// Orchestrator coordinates workflow execution across all system components
type Orchestrator struct {
	parser         types.Parser
	resolver       *resolver.DependencyResolver
	contextManager types.ContextManager
	executor       *executor.Executor
	taskRegistry   *tasks.Registry
	importResolver *imports.Resolver
	historyStore   *history.Store
	logger         types.Logger
	config         *Config
}

// Config holds orchestrator configuration
type Config struct {
	DryRun         bool
	MaxConcurrency int
	Logger         types.Logger
	Verbose        bool
	HistoryDir     string
}

// New creates a new workflow orchestrator
func New(config *Config) (*Orchestrator, error) {
	if config == nil {
		config = &Config{
			MaxConcurrency: types.DefaultConcurrency,
			HistoryDir:     "./history",
		}
	}

	// Validate MaxConcurrency
	maxConcurrency, err := types.ValidateConcurrency(config.MaxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("invalid orchestrator configuration: %w", err)
	}
	config.MaxConcurrency = maxConcurrency

	// Set default history directory if not specified
	if config.HistoryDir == "" {
		config.HistoryDir = "./history"
	}

	// Initialize template engine
	templateEngine := template.New()

	// Initialize context manager
	ctxManager := contextManager.New(templateEngine)

	// Initialize task registry
	taskRegistry := tasks.New()

	// Initialize executor
	executorConfig := &executor.Config{
		DryRun:         config.DryRun,
		MaxConcurrency: config.MaxConcurrency,
		Logger:         config.Logger,
	}
	exec, err := executor.New(ctxManager, executorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	// Register all tasks to executor
	taskRegistry.RegisterToExecutor(exec)

	// Initialize import resolver
	parserInstance := parser.New(afero.NewOsFs())
	importResolver := imports.New(&imports.Config{
		FileSystem: afero.NewOsFs(),
		Parser:     parserInstance,
		MaxDepth:   10,
	})

	// Initialize history store
	historyStore := history.New(config.HistoryDir, 10000) // Keep up to 10k records
	historyStore.Initialize()                             // Create directory if needed

	return &Orchestrator{
		parser:         parserInstance,
		resolver:       resolver.New(),
		contextManager: ctxManager,
		executor:       exec,
		taskRegistry:   taskRegistry,
		importResolver: importResolver,
		historyStore:   historyStore,
		logger:         config.Logger,
		config:         config,
	}, nil
}

// ExecuteWorkflowFile executes a workflow from a YAML file
func (o *Orchestrator) ExecuteWorkflowFile(ctx context.Context, filename string, envVars []string) (*types.Result, error) {
	o.logf("Loading workflow from file: %s", filename)

	// Parse workflow from file
	workflow, err := o.parser.ParseFile(filename)
	if err != nil {
		return &types.Result{
			ParseError: fmt.Errorf("failed to parse workflow file '%s': %w", filename, err),
		}, nil
	}

	// Update context manager with workflow directory for variable file loading
	workflowDir := filepath.Dir(filename)
	if workflowDir != "." {
		if mgr, ok := o.contextManager.(*contextManager.Manager); ok {
			mgr.SetWorkflowDir(workflowDir)
		}
	}

	return o.ExecuteWorkflowWithPath(ctx, workflow, envVars, filename)
}

// ExecuteWorkflowWithPath executes a workflow with file path context for imports
func (o *Orchestrator) ExecuteWorkflowWithPath(ctx context.Context, workflow *types.Workflow, envVars []string, workflowPath string) (*types.Result, error) {
	result := &types.Result{}

	o.logf("Starting workflow execution: %s", workflow.Name)
	startTime := time.Now()

	// Resolve imports if present
	if len(workflow.Imports) > 0 {
		o.logf("Resolving %d workflow imports", len(workflow.Imports))
		resolvedWorkflow, err := o.importResolver.ResolveImports(ctx, workflow, workflowPath)
		if err != nil {
			result.ParseError = fmt.Errorf("failed to resolve imports: %w", err)
			return result, nil
		}
		workflow = resolvedWorkflow
		o.logf("Successfully resolved imports, workflow now has %d tasks", len(workflow.Tasks))
	}

	// Continue with the rest of the execution logic
	return o.executeResolvedWorkflow(ctx, workflow, envVars, workflowPath, startTime, result)
}

// ExecuteWorkflowYAML executes a workflow from YAML content
func (o *Orchestrator) ExecuteWorkflowYAML(ctx context.Context, yamlContent []byte, envVars []string) (*types.Result, error) {
	o.logf("Parsing workflow from YAML content")

	// Parse workflow from YAML
	workflow, err := o.parser.Parse(yamlContent)
	if err != nil {
		return &types.Result{
			ParseError: fmt.Errorf("failed to parse workflow YAML: %w", err),
		}, nil
	}

	return o.ExecuteWorkflow(ctx, workflow, envVars)
}

// ExecuteWorkflow executes a parsed workflow
func (o *Orchestrator) ExecuteWorkflow(ctx context.Context, workflow *types.Workflow, envVars []string) (*types.Result, error) {
	return o.ExecuteWorkflowWithPath(ctx, workflow, envVars, "")
}

// executeResolvedWorkflow handles the actual workflow execution after imports are resolved
func (o *Orchestrator) executeResolvedWorkflow(ctx context.Context, workflow *types.Workflow, envVars []string, workflowPath string, startTime time.Time, result *types.Result) (*types.Result, error) {
	// Validate workflow
	o.logf("Validating workflow and tasks")
	if err := o.parser.Validate(workflow); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, fmt.Errorf("workflow validation failed: %w", err))
	}

	// Validate all tasks
	taskErrors := o.taskRegistry.ValidateAll(workflow.Tasks)
	result.ValidationErrors = append(result.ValidationErrors, taskErrors...)

	// Validate templates in task configurations
	templateEngine := o.contextManager.GetTemplateEngine()
	templateErrors := template.ValidateTaskTemplates(workflow.Tasks, templateEngine)
	result.ValidationErrors = append(result.ValidationErrors, templateErrors...)

	// Stop if we have validation errors
	if len(result.ValidationErrors) > 0 {
		o.logf("Workflow validation failed with %d errors", len(result.ValidationErrors))
		return result, nil
	}

	// Initialize context
	o.logf("Initializing workflow context")
	if err := o.contextManager.Initialize(workflow, envVars); err != nil {
		result.ExecutionError = fmt.Errorf("failed to initialize context: %w", err)
		return result, nil
	}

	// Build dependency graph
	o.logf("Building dependency graph with %d tasks", len(workflow.Tasks))
	if err := o.resolver.BuildGraph(workflow.Tasks); err != nil {
		result.DependencyError = fmt.Errorf("failed to build dependency graph: %w", err)
		return result, nil
	}

	// Validate dependency graph
	if err := o.resolver.ValidateGraph(); err != nil {
		result.DependencyError = fmt.Errorf("dependency validation failed: %w", err)
		return result, nil
	}

	// Get execution statistics
	stats := o.resolver.GetStats()
	o.logf("Dependency graph: %v", stats)

	// Execute workflow
	o.logf("Executing workflow")
	if o.config.DryRun {
		o.logf("DRY RUN MODE - No actual changes will be made")
	}

	workflowResult, err := o.executor.ExecuteWorkflow(ctx, workflow, o.resolver)
	if err != nil {
		result.ExecutionError = fmt.Errorf("workflow execution failed: %w", err)
		result.WorkflowResult = workflowResult // Include partial results
	} else {
		result.WorkflowResult = workflowResult
	}

	// Record execution history (regardless of success or failure)
	if o.historyStore != nil {
		triggerData := map[string]interface{}{
			"env_vars": envVars,
		}
		if historyErr := o.historyStore.RecordExecution(result, workflow.Name, workflowPath, "manual", triggerData); historyErr != nil {
			o.logf("Failed to record execution history: %v", historyErr)
		}
	}

	// Return early if execution failed
	if err != nil {
		return result, nil
	}

	// Log completion
	duration := time.Since(startTime)
	o.logf("Workflow '%s' completed with status %s in %v",
		workflow.Name, workflowResult.Status, duration)

	if o.config.Verbose {
		o.logWorkflowSummary(workflowResult)
	}

	return result, nil
}

// GetTaskRegistry returns the task registry for custom task registration
func (o *Orchestrator) GetTaskRegistry() *tasks.Registry {
	return o.taskRegistry
}

// GetContextManager returns the context manager for inspection
func (o *Orchestrator) GetContextManager() types.ContextManager {
	return o.contextManager
}

// ValidateWorkflowFile validates a workflow file without executing it
func (o *Orchestrator) ValidateWorkflowFile(filename string) (*types.Result, error) {
	o.logf("Validating workflow file: %s", filename)

	// Parse workflow from file
	workflow, err := o.parser.ParseFile(filename)
	if err != nil {
		return &types.Result{
			ParseError: fmt.Errorf("failed to parse workflow file '%s': %w", filename, err),
		}, nil
	}

	return o.ValidateWorkflow(workflow)
}

// ValidateWorkflow validates a parsed workflow without executing it
func (o *Orchestrator) ValidateWorkflow(workflow *types.Workflow) (*types.Result, error) {
	result := &types.Result{}

	o.logf("Validating workflow: %s", workflow.Name)

	// Validate workflow structure
	if err := o.parser.Validate(workflow); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, fmt.Errorf("workflow validation failed: %w", err))
	}

	// Validate all tasks
	taskErrors := o.taskRegistry.ValidateAll(workflow.Tasks)
	result.ValidationErrors = append(result.ValidationErrors, taskErrors...)

	// Validate templates in task configurations
	templateEngine := o.contextManager.GetTemplateEngine()
	templateErrors := template.ValidateTaskTemplates(workflow.Tasks, templateEngine)
	result.ValidationErrors = append(result.ValidationErrors, templateErrors...)

	// Validate dependencies
	if err := o.resolver.BuildGraph(workflow.Tasks); err != nil {
		result.DependencyError = fmt.Errorf("failed to build dependency graph: %w", err)
		return result, nil
	}

	if err := o.resolver.ValidateGraph(); err != nil {
		result.DependencyError = fmt.Errorf("dependency validation failed: %w", err)
		return result, nil
	}

	if len(result.ValidationErrors) == 0 {
		o.logf("Workflow validation passed")
	} else {
		o.logf("Workflow validation failed with %d errors", len(result.ValidationErrors))
	}

	return result, nil
}

// GetExecutionPlan returns the execution plan without running the workflow
func (o *Orchestrator) GetExecutionPlan(workflow *types.Workflow) (*ExecutionPlan, error) {
	// Build dependency graph
	if err := o.resolver.BuildGraph(workflow.Tasks); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Get execution layers
	layers, err := o.resolver.GetExecutionLayers()
	if err != nil {
		return nil, fmt.Errorf("failed to get execution layers: %w", err)
	}

	// Get statistics
	stats := o.resolver.GetStats()

	return &ExecutionPlan{
		Workflow: workflow,
		Layers:   layers,
		Stats:    stats,
	}, nil
}

// ExecutionPlan represents a workflow execution plan
type ExecutionPlan struct {
	Workflow *types.Workflow
	Layers   []*resolver.ExecutionLayer
	Stats    map[string]interface{}
}

// logf logs a formatted message if logger is available
func (o *Orchestrator) logf(format string, args ...interface{}) {
	if o.logger != nil {
		o.logger.Info().Msgf(format, args...)
	}
}

// logWorkflowSummary logs a summary of workflow execution results
func (o *Orchestrator) logWorkflowSummary(result *types.WorkflowResult) {
	if o.logger == nil {
		return
	}

	successCount := 0
	failedCount := 0
	skippedCount := 0

	for _, taskResult := range result.Tasks {
		switch taskResult.Status {
		case types.TaskSuccess:
			successCount++
		case types.TaskFailed:
			failedCount++
		case types.TaskSkipped:
			skippedCount++
		}
	}

	o.logger.Info().
		Str("workflow", result.Name).
		Str("status", string(result.Status)).
		Dur("duration", result.Duration).
		Int("total_tasks", len(result.Tasks)).
		Int("successful", successCount).
		Int("failed", failedCount).
		Int("skipped", skippedCount).
		Msg("Workflow execution summary")

	// Log failed tasks
	if failedCount > 0 {
		for taskID, taskResult := range result.Tasks {
			if taskResult.Status == types.TaskFailed {
				o.logger.Error().
					Str("task_id", taskID).
					Str("task_name", taskResult.Name).
					Str("task_type", taskResult.Type).
					Str("error", taskResult.Message).
					Msg("Task failed")
			}
		}
	}
}
