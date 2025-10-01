// ABOUTME: Tests for the task execution engine with parallel and sequential modes
// ABOUTME: Validates task lifecycle, dependency handling, and result collection

package executor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sarlalian/ritual/internal/workflow/resolver"
	"github.com/sarlalian/ritual/pkg/types"
)

// MockTaskExecutor implements TaskExecutor for testing
type MockTaskExecutor struct {
	name         string
	shouldFail   bool
	shouldSkip   bool
	executionLog []string
	delay        time.Duration
}

func (m *MockTaskExecutor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
	}

	// Add delay if specified (for testing parallel execution)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.shouldSkip {
		result.Status = types.TaskSkipped
		result.Message = "Mock task skipped"
	} else if m.shouldFail {
		result.Status = types.TaskFailed
		result.Message = "Mock task failed"
		result.ReturnCode = 1
	} else {
		result.Status = types.TaskSuccess
		result.Message = "Mock task completed"
		result.Stdout = "mock output"
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Log execution for order verification
	if m.executionLog != nil {
		m.executionLog = append(m.executionLog, task.ID)
	}

	return result
}

func (m *MockTaskExecutor) Validate(task *types.TaskConfig) error {
	if task.Type == "invalid" {
		return errors.New("invalid task type")
	}
	return nil
}

func (m *MockTaskExecutor) SupportsDryRun() bool {
	return true
}

// MockContextManager implements ContextManager for testing
type MockContextManager struct {
	variables   map[string]interface{}
	environment map[string]string
	taskResults map[string]*types.TaskResult
}

func NewMockContextManager() *MockContextManager {
	return &MockContextManager{
		variables:   make(map[string]interface{}),
		environment: make(map[string]string),
		taskResults: make(map[string]*types.TaskResult),
	}
}

func (m *MockContextManager) Initialize(workflow *types.Workflow, envVars []string) error {
	return nil
}

func (m *MockContextManager) GetContext() *types.WorkflowContext {
	return &types.WorkflowContext{
		Variables:   m.variables,
		Environment: m.environment,
		Tasks:       m.taskResults,
	}
}

func (m *MockContextManager) GetVariable(name string) (interface{}, error) {
	if val, exists := m.variables[name]; exists {
		return val, nil
	}
	return nil, errors.New("variable not found")
}

func (m *MockContextManager) SetVariable(name string, value interface{}) error {
	m.variables[name] = value
	return nil
}

func (m *MockContextManager) GetEnvironment(name, defaultValue string) string {
	if val, exists := m.environment[name]; exists {
		return val
	}
	return defaultValue
}

func (m *MockContextManager) SetEnvironment(name, value string) error {
	m.environment[name] = value
	return nil
}

func (m *MockContextManager) RegisterTaskResult(taskResult *types.TaskResult) error {
	m.taskResults[taskResult.ID] = taskResult
	if taskResult.Name != taskResult.ID {
		m.taskResults[taskResult.Name] = taskResult
	}
	return nil
}

func (m *MockContextManager) GetTaskResult(identifier string) (*types.TaskResult, error) {
	if result, exists := m.taskResults[identifier]; exists {
		return result, nil
	}
	return nil, errors.New("task result not found")
}

func (m *MockContextManager) EvaluateString(templateStr string) (string, error) {
	// Simple mock evaluation - just return the template as-is for most cases
	if templateStr == "false" || templateStr == "" {
		return "false", nil
	}
	return "true", nil
}

func (m *MockContextManager) EvaluateMap(data map[string]interface{}) (map[string]interface{}, error) {
	return data, nil
}

func (m *MockContextManager) GetTemplateEngine() types.TemplateEngine {
	return &MockTemplateEngine{}
}

func (m *MockContextManager) Clone() types.ContextManager {
	clone := NewMockContextManager()
	for k, v := range m.variables {
		clone.variables[k] = v
	}
	for k, v := range m.environment {
		clone.environment[k] = v
	}
	for k, v := range m.taskResults {
		clone.taskResults[k] = v
	}
	return clone
}

// MockTemplateEngine implements TemplateEngine for testing
type MockTemplateEngine struct{}

func (m *MockTemplateEngine) Evaluate(template string, ctx *types.WorkflowContext) (string, error) {
	return template, nil
}

func (m *MockTemplateEngine) EvaluateAll(data map[string]interface{}, ctx *types.WorkflowContext) (map[string]interface{}, error) {
	return data, nil
}

// MockDependencyResolver implements DependencyResolver for testing
type MockDependencyResolver struct {
	layers []*resolver.ExecutionLayer
}

func NewMockResolver(tasks []types.TaskConfig) *resolver.DependencyResolver {
	// Use the real resolver to build the graph for tests
	r := resolver.New()
	if err := r.BuildGraph(tasks); err != nil {
		// If graph building fails, return empty resolver
		return r
	}
	return r
}

func TestExecutor_New(t *testing.T) {
	contextManager := NewMockContextManager()

	// Test with default config
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if executor.contextManager != contextManager {
		t.Error("Expected context manager to be set")
	}
	if executor.maxConcurrency != 10 {
		t.Errorf("Expected default concurrency to be 10, got %d", executor.maxConcurrency)
	}
	if executor.dryRun != false {
		t.Error("Expected default dry run to be false")
	}

	// Test with custom config
	config := &Config{
		DryRun:         true,
		MaxConcurrency: 5,
	}
	executor, err = New(contextManager, config)
	if err != nil {
		t.Fatalf("Failed to create executor with custom config: %v", err)
	}
	if executor.dryRun != true {
		t.Error("Expected dry run to be true")
	}
	if executor.maxConcurrency != 5 {
		t.Errorf("Expected concurrency to be 5, got %d", executor.maxConcurrency)
	}
}

func TestExecutor_RegisterTask(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{}
	executor.RegisterTask("test", mockExecutor)

	if len(executor.taskRegistry) != 1 {
		t.Errorf("Expected 1 registered task, got %d", len(executor.taskRegistry))
	}

	if executor.taskRegistry["test"] != mockExecutor {
		t.Error("Expected registered task executor to match")
	}
}

func TestExecutor_ExecuteTask_Success(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{}
	executor.RegisterTask("test", mockExecutor)

	task := &types.TaskConfig{
		ID:   "task1",
		Name: "Test Task",
		Type: "test",
	}

	result, err := executor.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s", result.Status)
	}

	if result.ID != "task1" {
		t.Errorf("Expected task ID 'task1', got '%s'", result.ID)
	}

	// Check that result was registered in context
	registeredResult, err := contextManager.GetTaskResult("task1")
	if err != nil {
		t.Errorf("Expected task result to be registered: %v", err)
	}
	if registeredResult.Status != types.TaskSuccess {
		t.Error("Expected registered task result to have success status")
	}
}

func TestExecutor_ExecuteTask_Failure(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{shouldFail: true}
	executor.RegisterTask("test", mockExecutor)

	task := &types.TaskConfig{
		ID:   "task1",
		Name: "Test Task",
		Type: "test",
	}

	result, err := executor.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure, got %s", result.Status)
	}

	if result.ReturnCode != 1 {
		t.Errorf("Expected return code 1, got %d", result.ReturnCode)
	}
}

func TestExecutor_ExecuteTask_UnknownType(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "task1",
		Name: "Test Task",
		Type: "unknown",
	}

	_, execErr := executor.ExecuteTask(context.Background(), task)
	if execErr == nil {
		t.Error("Expected error for unknown task type")
	}

	if !strings.Contains(execErr.Error(), "no executor registered") {
		t.Errorf("Expected registration error, got: %v", execErr)
	}
}

func TestExecutor_ExecuteTask_DryRun(t *testing.T) {
	contextManager := NewMockContextManager()
	config := &Config{DryRun: true}
	executor, err := New(contextManager, config)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{}
	executor.RegisterTask("test", mockExecutor)

	task := &types.TaskConfig{
		ID:   "task1",
		Name: "Test Task",
		Type: "test",
	}

	result, err := executor.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != types.TaskSkipped {
		t.Errorf("Expected task skipped in dry run, got %s", result.Status)
	}

	if !strings.Contains(result.Message, "Dry run mode") {
		t.Errorf("Expected dry run message, got: %s", result.Message)
	}
}

func TestExecutor_ExecuteTask_Condition_Skip(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{}
	executor.RegisterTask("test", mockExecutor)

	task := &types.TaskConfig{
		ID:   "task1",
		Name: "Test Task",
		Type: "test",
		When: "false", // Mock evaluates this to false
	}

	result, err := executor.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != types.TaskSkipped {
		t.Errorf("Expected task skipped due to condition, got %s", result.Status)
	}

	if !strings.Contains(result.Message, "condition") {
		t.Errorf("Expected condition message, got: %s", result.Message)
	}
}

func TestExecutor_ExecuteWorkflow_Parallel(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{}
	executor.RegisterTask("test", mockExecutor)

	workflow := &types.Workflow{
		Name: "Test Workflow",
		Mode: types.ParallelMode,
		Tasks: []types.TaskConfig{
			{ID: "task1", Name: "Task 1", Type: "test"},
			{ID: "task2", Name: "Task 2", Type: "test"},
		},
	}

	resolver := NewMockResolver(workflow.Tasks)
	result, err := executor.ExecuteWorkflow(context.Background(), workflow, resolver)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != types.WorkflowSuccess {
		t.Errorf("Expected workflow success, got %s", result.Status)
	}

	if len(result.Tasks) != 2 {
		t.Errorf("Expected 2 task results, got %d", len(result.Tasks))
	}

	// Check that both tasks completed successfully
	for taskID, taskResult := range result.Tasks {
		if taskResult.Status != types.TaskSuccess {
			t.Errorf("Expected task %s to succeed, got %s", taskID, taskResult.Status)
		}
	}
}

func TestExecutor_ExecuteWorkflow_Sequential(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{}
	executor.RegisterTask("test", mockExecutor)

	workflow := &types.Workflow{
		Name: "Test Workflow",
		Mode: types.SequentialMode,
		Tasks: []types.TaskConfig{
			{ID: "task1", Name: "Task 1", Type: "test"},
			{ID: "task2", Name: "Task 2", Type: "test"},
		},
	}

	resolver := NewMockResolver(workflow.Tasks)
	result, err := executor.ExecuteWorkflow(context.Background(), workflow, resolver)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != types.WorkflowSuccess {
		t.Errorf("Expected workflow success, got %s", result.Status)
	}

	if len(result.Tasks) != 2 {
		t.Errorf("Expected 2 task results, got %d", len(result.Tasks))
	}
}

func TestExecutor_ExecuteWorkflow_RequiredTaskFailure(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	mockExecutor := &MockTaskExecutor{shouldFail: true}
	executor.RegisterTask("test", mockExecutor)

	required := true
	workflow := &types.Workflow{
		Name: "Test Workflow",
		Mode: types.ParallelMode,
		Tasks: []types.TaskConfig{
			{ID: "task1", Name: "Task 1", Type: "test", Required: &required},
		},
	}

	resolver := NewMockResolver(workflow.Tasks)
	result, err := executor.ExecuteWorkflow(context.Background(), workflow, resolver)

	if err == nil {
		t.Error("Expected error for required task failure")
	}

	if result.Status != types.WorkflowFailed {
		t.Errorf("Expected workflow failed, got %s", result.Status)
	}

	if !strings.Contains(err.Error(), "required task") {
		t.Errorf("Expected required task error message, got: %v", err)
	}
}

func TestExecutor_ExecuteWorkflow_OptionalTaskFailure(t *testing.T) {
	contextManager := NewMockContextManager()
	executor, err := New(contextManager, nil)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	// First task fails (optional), second task succeeds
	executor.RegisterTask("fail", &MockTaskExecutor{shouldFail: true})
	executor.RegisterTask("success", &MockTaskExecutor{})

	optional := false
	workflow := &types.Workflow{
		Name: "Test Workflow",
		Mode: types.ParallelMode,
		Tasks: []types.TaskConfig{
			{ID: "task1", Name: "Task 1", Type: "fail", Required: &optional},
			{ID: "task2", Name: "Task 2", Type: "success"},
		},
	}

	resolver := NewMockResolver(workflow.Tasks)
	result, err := executor.ExecuteWorkflow(context.Background(), workflow, resolver)

	if err != nil {
		t.Fatalf("Expected no error for optional task failure, got: %v", err)
	}

	if result.Status != types.WorkflowFailed {
		t.Errorf("Expected workflow failed due to failed task, got %s", result.Status)
	}
}

func TestExecutor_IsTruthy(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"", false},
		{"false", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"true", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"anything else", true},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			result := isTruthy(test.value)
			if result != test.expected {
				t.Errorf("Expected isTruthy('%s') to be %v, got %v", test.value, test.expected, result)
			}
		})
	}
}
