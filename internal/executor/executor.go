// ABOUTME: Core task execution engine with parallel and sequential execution modes
// ABOUTME: Manages task lifecycle, dependency resolution, and result collection

package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sarlalian/ritual/internal/workflow/resolver"
	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles task execution with dependency resolution
type Executor struct {
	contextManager types.ContextManager
	taskRegistry   map[string]types.TaskExecutor
	logger         types.Logger
	dryRun         bool
	maxConcurrency int
}

// Config holds executor configuration
type Config struct {
	DryRun         bool
	MaxConcurrency int
	Logger         types.Logger
}

// New creates a new executor with the given context manager
func New(contextManager types.ContextManager, config *Config) *Executor {
	if config == nil {
		config = &Config{
			DryRun:         false,
			MaxConcurrency: 10, // Default parallelism
		}
	}

	return &Executor{
		contextManager: contextManager,
		taskRegistry:   make(map[string]types.TaskExecutor),
		logger:         config.Logger,
		dryRun:         config.DryRun,
		maxConcurrency: config.MaxConcurrency,
	}
}

// RegisterTask registers a task executor for a specific task type
func (e *Executor) RegisterTask(taskType string, executor types.TaskExecutor) {
	e.taskRegistry[taskType] = executor
}

// ExecuteWorkflow executes a complete workflow with dependency resolution
func (e *Executor) ExecuteWorkflow(ctx context.Context, workflow *types.Workflow, resolverImpl *resolver.DependencyResolver) (*types.WorkflowResult, error) {
	startTime := time.Now()

	// Get execution layers from resolver
	layers, err := resolverImpl.GetExecutionLayers()
	if err != nil {
		return nil, fmt.Errorf("failed to get execution layers: %w", err)
	}

	result := &types.WorkflowResult{
		Name:      workflow.Name,
		StartTime: startTime,
		Tasks:     make(map[string]*types.TaskResult),
		Status:    types.WorkflowRunning,
	}

	// Execute layers sequentially, tasks within layers in parallel
	for layerNum, layer := range layers {
		e.logf("Executing layer %d with %d tasks", layerNum, len(layer.Tasks))

		if workflow.Mode == types.SequentialMode {
			// Execute tasks sequentially within the layer
			if err := e.executeLayerSequential(ctx, layer, result); err != nil {
				result.Status = types.WorkflowFailed
				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(startTime)
				return result, err
			}
		} else {
			// Execute tasks in parallel within the layer
			if err := e.executeLayerParallel(ctx, layer, result); err != nil {
				result.Status = types.WorkflowFailed
				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(startTime)
				return result, err
			}
		}
	}

	// Determine final status
	result.Status = types.WorkflowSuccess
	for _, taskResult := range result.Tasks {
		if taskResult.Status == types.TaskFailed {
			result.Status = types.WorkflowFailed
			break
		} else if taskResult.Status == types.TaskSkipped {
			result.Status = types.WorkflowPartialSuccess
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	return result, nil
}

// ExecuteTask executes a single task
func (e *Executor) ExecuteTask(ctx context.Context, task *types.TaskConfig) (*types.TaskResult, error) {
	executor, exists := e.taskRegistry[task.Type]
	if !exists {
		return nil, fmt.Errorf("no executor registered for task type '%s'", task.Type)
	}

	// Create task result
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
		Status:    types.TaskRunning,
	}

	e.logf("Executing task '%s' (%s)", task.Name, task.Type)

	// Check if task should be skipped based on conditions
	if shouldSkip, reason := e.shouldSkipTask(task); shouldSkip {
		result.Status = types.TaskSkipped
		result.Message = reason
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		e.logf("Task '%s' skipped: %s", task.Name, reason)
		return result, nil
	}

	// Execute the task
	if e.dryRun {
		result.Status = types.TaskSkipped
		result.Message = "Dry run mode - task would be executed"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		e.logf("Task '%s' dry run completed", task.Name)
	} else {
		// Execute the actual task
		execResult := executor.Execute(ctx, task, e.contextManager)

		// Update result with execution details
		result.Status = execResult.Status
		result.Message = execResult.Message
		result.Stdout = execResult.Stdout
		result.Stderr = execResult.Stderr
		result.ReturnCode = execResult.ReturnCode
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		// Log completion
		if result.Status == types.TaskSuccess {
			e.logf("Task '%s' completed successfully", task.Name)
		} else {
			e.logf("Task '%s' failed: %s", task.Name, result.Message)
		}
	}

	// Register task result in context for other tasks to use
	if err := e.contextManager.RegisterTaskResult(result); err != nil {
		e.logf("Warning: failed to register task result for '%s': %v", task.ID, err)
	}

	return result, nil
}

// executeLayerSequential executes all tasks in a layer sequentially
func (e *Executor) executeLayerSequential(ctx context.Context, layer *resolver.ExecutionLayer, workflowResult *types.WorkflowResult) error {
	for _, taskNode := range layer.Tasks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		result, err := e.ExecuteTask(ctx, taskNode.Task)
		if err != nil {
			return fmt.Errorf("task '%s' execution failed: %w", taskNode.Task.ID, err)
		}

		workflowResult.Tasks[taskNode.Task.ID] = result

		// Stop execution if task failed and it's required
		if result.Status == types.TaskFailed && taskNode.Task.IsRequired() {
			return fmt.Errorf("required task '%s' failed: %s", taskNode.Task.Name, result.Message)
		}
	}

	return nil
}

// executeLayerParallel executes all tasks in a layer in parallel
func (e *Executor) executeLayerParallel(ctx context.Context, layer *resolver.ExecutionLayer, workflowResult *types.WorkflowResult) error {
	// Limit concurrency
	semaphore := make(chan struct{}, e.maxConcurrency)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	for _, taskNode := range layer.Tasks {
		wg.Add(1)

		go func(node *resolver.TaskNode) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			select {
			case <-ctx.Done():
				mu.Lock()
				if firstError == nil {
					firstError = ctx.Err()
				}
				mu.Unlock()
				return
			default:
			}

			result, err := e.ExecuteTask(ctx, node.Task)

			mu.Lock()
			defer mu.Unlock()

			if err != nil && firstError == nil {
				firstError = fmt.Errorf("task '%s' execution failed: %w", node.Task.ID, err)
				return
			}

			workflowResult.Tasks[node.Task.ID] = result

			// Check if required task failed
			if result.Status == types.TaskFailed && node.Task.IsRequired() && firstError == nil {
				firstError = fmt.Errorf("required task '%s' failed: %s", node.Task.Name, result.Message)
			}
		}(taskNode)
	}

	wg.Wait()
	return firstError
}

// shouldSkipTask determines if a task should be skipped based on conditions
func (e *Executor) shouldSkipTask(task *types.TaskConfig) (bool, string) {
	// Check if task has a condition
	if task.When != "" {
		// Evaluate the condition using the template engine
		result, err := e.contextManager.EvaluateString(task.When)
		if err != nil {
			return true, fmt.Sprintf("failed to evaluate condition: %v", err)
		}

		// Check if result is truthy
		if !isTruthy(result) {
			return true, fmt.Sprintf("condition '%s' evaluated to false", task.When)
		}
	}

	return false, ""
}

// isTruthy determines if a string represents a truthy value
func isTruthy(value string) bool {
	switch value {
	case "", "false", "0", "no", "off":
		return false
	default:
		return true
	}
}

// logf logs a formatted message if logger is available
func (e *Executor) logf(format string, args ...interface{}) {
	if e.logger != nil {
		e.logger.Info().Msgf(format, args...)
	}
}