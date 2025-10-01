// ABOUTME: Tests for the workflow orchestrator integration
// ABOUTME: Validates end-to-end workflow execution, validation, and error handling

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarlalian/ritual/pkg/types"
)

func TestOrchestrator_New(t *testing.T) {
	config := &Config{
		DryRun:         true,
		MaxConcurrency: 5,
		Verbose:        true,
	}

	orchestrator := New(config)
	if orchestrator == nil {
		t.Error("Expected orchestrator to be created")
	}

	if orchestrator.config.DryRun != true {
		t.Error("Expected dry run to be enabled")
	}

	if orchestrator.config.MaxConcurrency != 5 {
		t.Error("Expected max concurrency to be 5")
	}

	// Check that all components were initialized
	if orchestrator.parser == nil {
		t.Error("Expected parser to be initialized")
	}
	if orchestrator.resolver == nil {
		t.Error("Expected resolver to be initialized")
	}
	if orchestrator.contextManager == nil {
		t.Error("Expected context manager to be initialized")
	}
	if orchestrator.executor == nil {
		t.Error("Expected executor to be initialized")
	}
	if orchestrator.taskRegistry == nil {
		t.Error("Expected task registry to be initialized")
	}
}

func TestOrchestrator_ValidateWorkflow(t *testing.T) {
	orchestrator := New(nil)

	// Create a valid workflow
	validWorkflow := &types.Workflow{
		Name: "Test Workflow",
		Tasks: []types.TaskConfig{
			{
				ID:   "task1",
				Name: "Test Command",
				Type: "command",
				Config: map[string]interface{}{
					"command": "echo hello",
				},
			},
		},
	}

	result, err := orchestrator.ValidateWorkflow(validWorkflow)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(result.ValidationErrors) > 0 {
		t.Errorf("Expected no validation errors, got: %v", result.ValidationErrors)
	}

	if result.ParseError != nil {
		t.Errorf("Expected no parse error, got: %v", result.ParseError)
	}

	if result.DependencyError != nil {
		t.Errorf("Expected no dependency error, got: %v", result.DependencyError)
	}
}

func TestOrchestrator_ValidateWorkflow_InvalidTask(t *testing.T) {
	orchestrator := New(nil)

	// Create a workflow with invalid task
	invalidWorkflow := &types.Workflow{
		Name: "Invalid Workflow",
		Tasks: []types.TaskConfig{
			{
				ID:   "task1",
				Name: "Invalid Command",
				Type: "command",
				Config: map[string]interface{}{
					// Missing command
				},
			},
		},
	}

	result, err := orchestrator.ValidateWorkflow(invalidWorkflow)
	if err != nil {
		t.Errorf("Expected no error from validate method, got: %v", err)
	}

	if len(result.ValidationErrors) == 0 {
		t.Error("Expected validation errors for invalid task")
	}

	// Check that we got a command validation error
	foundCommandError := false
	for _, err := range result.ValidationErrors {
		if strings.Contains(err.Error(), "command task must specify") {
			foundCommandError = true
			break
		}
	}

	if !foundCommandError {
		t.Error("Expected command validation error")
	}
}

func TestOrchestrator_ValidateWorkflow_CircularDependency(t *testing.T) {
	orchestrator := New(nil)

	// Create a workflow with circular dependencies
	circularWorkflow := &types.Workflow{
		Name: "Circular Workflow",
		Tasks: []types.TaskConfig{
			{
				ID:   "task1",
				Name: "Task 1",
				Type: "command",
				Config: map[string]interface{}{
					"command": "echo task1",
				},
				DependsOn: []string{"task2"},
			},
			{
				ID:   "task2",
				Name: "Task 2",
				Type: "command",
				Config: map[string]interface{}{
					"command": "echo task2",
				},
				DependsOn: []string{"task1"},
			},
		},
	}

	result, err := orchestrator.ValidateWorkflow(circularWorkflow)
	if err != nil {
		t.Errorf("Expected no error from validate method, got: %v", err)
	}

	if result.DependencyError == nil {
		t.Error("Expected dependency error for circular dependencies")
	}

	if !strings.Contains(result.DependencyError.Error(), "circular dependency") {
		t.Errorf("Expected circular dependency error, got: %v", result.DependencyError)
	}
}

func TestOrchestrator_ExecuteWorkflowYAML(t *testing.T) {
	orchestrator := New(&Config{DryRun: true})

	yamlContent := []byte(`
name: Test Workflow
tasks:
  - id: task1
    name: Echo Hello
    type: command
    command: echo "Hello World"
`)

	result, err := orchestrator.ExecuteWorkflowYAML(context.Background(), yamlContent, nil)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result.ParseError != nil {
		t.Errorf("Expected no parse error, got: %v", result.ParseError)
	}

	if len(result.ValidationErrors) > 0 {
		t.Errorf("Expected no validation errors, got: %v", result.ValidationErrors)
	}

	if result.WorkflowResult == nil {
		t.Error("Expected workflow result")
		return // Avoid panic
	}

	if result.WorkflowResult.Status != types.WorkflowSuccess {
		t.Errorf("Expected workflow success, got: %v", result.WorkflowResult.Status)
	}
}

func TestOrchestrator_ExecuteWorkflowYAML_ParseError(t *testing.T) {
	orchestrator := New(nil)

	invalidYaml := []byte(`
name: Test Workflow
tasks:
  - id: task1
    name: Echo Hello
    type: command
    command: echo "Hello World"
  invalid_yaml_here
`)

	result, err := orchestrator.ExecuteWorkflowYAML(context.Background(), invalidYaml, nil)
	if err != nil {
		t.Errorf("Expected no error from method, got: %v", err)
	}

	if result.ParseError == nil {
		t.Error("Expected parse error for invalid YAML")
	}

	if !strings.Contains(result.ParseError.Error(), "failed to parse workflow YAML") {
		t.Errorf("Expected YAML parse error, got: %v", result.ParseError)
	}
}

func TestOrchestrator_ExecuteWorkflowFile(t *testing.T) {
	orchestrator := New(&Config{DryRun: true})

	// Create a temporary workflow file
	tmpDir := t.TempDir()
	workflowFile := filepath.Join(tmpDir, "test-workflow.yaml")

	workflowContent := `
name: File Test Workflow
tasks:
  - id: task1
    name: Echo Hello
    type: command
    command: echo "Hello from file"
`

	err := os.WriteFile(workflowFile, []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := orchestrator.ExecuteWorkflowFile(context.Background(), workflowFile, nil)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result.ParseError != nil {
		t.Errorf("Expected no parse error, got: %v", result.ParseError)
	}

	if result.WorkflowResult == nil {
		t.Error("Expected workflow result")
	}

	if result.WorkflowResult.Name != "File Test Workflow" {
		t.Errorf("Expected workflow name 'File Test Workflow', got: %v", result.WorkflowResult.Name)
	}
}

func TestOrchestrator_ExecuteWorkflowFile_NotFound(t *testing.T) {
	orchestrator := New(nil)

	result, err := orchestrator.ExecuteWorkflowFile(context.Background(), "/nonexistent/file.yaml", nil)
	if err != nil {
		t.Errorf("Expected no error from method, got: %v", err)
	}

	if result.ParseError == nil {
		t.Error("Expected parse error for nonexistent file")
	}

	if !strings.Contains(result.ParseError.Error(), "failed to parse workflow file") {
		t.Errorf("Expected file parse error, got: %v", result.ParseError)
	}
}

func TestOrchestrator_ExecuteWorkflow_WithVariables(t *testing.T) {
	orchestrator := New(&Config{DryRun: true})

	workflow := &types.Workflow{
		Name: "Variable Test Workflow",
		Variables: map[string]interface{}{
			"greeting": "Hello",
			"name":     "World",
		},
		Tasks: []types.TaskConfig{
			{
				ID:   "task1",
				Name: "Echo Greeting",
				Type: "command",
				Config: map[string]interface{}{
					"command": "echo '{{ .vars.greeting }} {{ .vars.name }}'",
				},
			},
		},
	}

	result, err := orchestrator.ExecuteWorkflow(context.Background(), workflow, nil)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result.WorkflowResult == nil {
		t.Error("Expected workflow result")
	}

	if result.WorkflowResult.Status != types.WorkflowSuccess {
		t.Errorf("Expected workflow success, got: %v", result.WorkflowResult.Status)
	}

	// Check that the task was executed
	if len(result.WorkflowResult.Tasks) != 1 {
		t.Errorf("Expected 1 task result, got: %d", len(result.WorkflowResult.Tasks))
	}

	taskResult := result.WorkflowResult.Tasks["task1"]
	if taskResult == nil {
		t.Error("Expected task1 result")
	}

	if taskResult.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got: %v", taskResult.Status)
	}
}

func TestOrchestrator_ExecuteWorkflow_WithEnvironment(t *testing.T) {
	orchestrator := New(&Config{DryRun: true})

	workflow := &types.Workflow{
		Name: "Environment Test Workflow",
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
		Tasks: []types.TaskConfig{
			{
				ID:   "task1",
				Name: "Echo Environment",
				Type: "command",
				Config: map[string]interface{}{
					"command": "echo $TEST_VAR",
				},
			},
		},
	}

	envVars := []string{"EXTRA_VAR=extra_value"}

	result, err := orchestrator.ExecuteWorkflow(context.Background(), workflow, envVars)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result.WorkflowResult == nil {
		t.Error("Expected workflow result")
	}

	if result.WorkflowResult.Status != types.WorkflowSuccess {
		t.Errorf("Expected workflow success, got: %v", result.WorkflowResult.Status)
	}
}

func TestOrchestrator_GetExecutionPlan(t *testing.T) {
	orchestrator := New(nil)

	workflow := &types.Workflow{
		Name: "Plan Test Workflow",
		Tasks: []types.TaskConfig{
			{
				ID:   "task1",
				Name: "Independent Task",
				Type: "command",
				Config: map[string]interface{}{
					"command": "echo task1",
				},
			},
			{
				ID:   "task2",
				Name: "Dependent Task",
				Type: "command",
				Config: map[string]interface{}{
					"command": "echo task2",
				},
				DependsOn: []string{"task1"},
			},
		},
	}

	plan, err := orchestrator.GetExecutionPlan(workflow)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if plan == nil {
		t.Error("Expected execution plan")
	}

	if plan.Workflow != workflow {
		t.Error("Expected plan to reference the workflow")
	}

	if len(plan.Layers) == 0 {
		t.Error("Expected execution layers")
	}

	// Should have 2 layers: task1 in layer 0, task2 in layer 1
	if len(plan.Layers) != 2 {
		t.Errorf("Expected 2 execution layers, got: %d", len(plan.Layers))
	}

	// Check layer 0 has task1
	layer0 := plan.Layers[0]
	if len(layer0.Tasks) != 1 {
		t.Errorf("Expected 1 task in layer 0, got: %d", len(layer0.Tasks))
	}

	if layer0.Tasks[0].Task.ID != "task1" {
		t.Errorf("Expected task1 in layer 0, got: %s", layer0.Tasks[0].Task.ID)
	}

	// Check layer 1 has task2
	layer1 := plan.Layers[1]
	if len(layer1.Tasks) != 1 {
		t.Errorf("Expected 1 task in layer 1, got: %d", len(layer1.Tasks))
	}

	if layer1.Tasks[0].Task.ID != "task2" {
		t.Errorf("Expected task2 in layer 1, got: %s", layer1.Tasks[0].Task.ID)
	}
}

func TestOrchestrator_GetTaskRegistry(t *testing.T) {
	orchestrator := New(nil)

	registry := orchestrator.GetTaskRegistry()
	if registry == nil {
		t.Error("Expected task registry")
	}

	// Check that built-in tasks are available
	availableTypes := registry.GetAvailableTypes()
	expectedTypes := []string{"command", "file", "compress"}

	for _, expectedType := range expectedTypes {
		found := false
		for _, availableType := range availableTypes {
			if availableType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected task type '%s' to be available", expectedType)
		}
	}
}

func TestOrchestrator_GetContextManager(t *testing.T) {
	orchestrator := New(nil)

	contextManager := orchestrator.GetContextManager()
	if contextManager == nil {
		t.Error("Expected context manager")
	}
}

func TestOrchestrator_ValidateWorkflowFile(t *testing.T) {
	orchestrator := New(nil)

	// Create a temporary workflow file
	tmpDir := t.TempDir()
	workflowFile := filepath.Join(tmpDir, "validate-test.yaml")

	workflowContent := `
name: Validation Test Workflow
tasks:
  - id: task1
    name: Valid Task
    type: command
    command: echo "valid"
  - id: task2
    name: Invalid Task
    type: command
    # Missing command
`

	err := os.WriteFile(workflowFile, []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := orchestrator.ValidateWorkflowFile(workflowFile)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result.ParseError != nil {
		t.Errorf("Expected no parse error, got: %v", result.ParseError)
	}

	// Should have validation error for task2
	if len(result.ValidationErrors) == 0 {
		t.Error("Expected validation errors")
	}

	foundInvalidTaskError := false
	for _, validationErr := range result.ValidationErrors {
		if strings.Contains(validationErr.Error(), "command task must specify") {
			foundInvalidTaskError = true
			break
		}
	}

	if !foundInvalidTaskError {
		t.Error("Expected validation error for invalid task")
	}
}

func TestOrchestrator_ExecuteWorkflow_Timeout(t *testing.T) {
	orchestrator := New(&Config{DryRun: false})

	workflow := &types.Workflow{
		Name: "Timeout Test Workflow",
		Tasks: []types.TaskConfig{
			{
				ID:   "task1",
				Name: "Long Running Task",
				Type: "command",
				Config: map[string]interface{}{
					"command": "sleep 2",
					"timeout": "100ms",
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := orchestrator.ExecuteWorkflow(ctx, workflow, nil)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result.WorkflowResult == nil {
		t.Error("Expected workflow result")
	}

	// Task should have failed due to timeout
	taskResult := result.WorkflowResult.Tasks["task1"]
	if taskResult == nil {
		t.Error("Expected task1 result")
	}

	if taskResult.Status != types.TaskFailed {
		t.Errorf("Expected task to fail due to timeout, got: %v", taskResult.Status)
	}

	if !strings.Contains(taskResult.Error, "timeout") {
		t.Errorf("Expected timeout error, got: %v", taskResult.Error)
	}
}