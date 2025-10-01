// ABOUTME: Integration tests for the complete Ritual workflow engine
// ABOUTME: Tests end-to-end functionality with real workflows and file operations

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarlalian/ritual/internal/orchestrator"
	"github.com/sarlalian/ritual/pkg/types"
)

func TestIntegration_SimpleCommandWorkflow(t *testing.T) {
	// Create temporary workflow file
	tmpDir := t.TempDir()
	workflowFile := filepath.Join(tmpDir, "simple.yaml")

	workflowContent := `
name: Integration Test Simple Workflow
description: Test basic command execution

environment:
  TEST_VAR: "integration_test"

vars:
  greeting: "Hello from {{ .env.TEST_VAR }}"

tasks:
  - id: echo_test
    name: Echo Test Variable
    type: command
    command: "echo '{{ .vars.greeting }}'"

  - id: list_files
    name: List Current Directory
    type: command
    command: "ls -la"
    depends_on: [echo_test]
`

	err := os.WriteFile(workflowFile, []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	// Create orchestrator
	orch := orchestrator.New(&orchestrator.Config{
		DryRun:         false,
		MaxConcurrency: 4,
		Verbose:        true,
	})

	// Execute workflow
	ctx := context.Background()
	result, err := orch.ExecuteWorkflowFile(ctx, workflowFile, nil)
	if err != nil {
		t.Errorf("Failed to execute workflow: %v", err)
	}

	// Verify results
	if result.ParseError != nil {
		t.Errorf("Unexpected parse error: %v", result.ParseError)
	}

	if result.DependencyError != nil {
		t.Errorf("Unexpected dependency error: %v", result.DependencyError)
	}

	if len(result.ValidationErrors) > 0 {
		t.Errorf("Unexpected validation errors: %v", result.ValidationErrors)
	}

	if result.ExecutionError != nil {
		t.Errorf("Unexpected execution error: %v", result.ExecutionError)
	}

	if result.WorkflowResult == nil {
		t.Fatal("Expected workflow result")
	}

	if result.WorkflowResult.Status != "success" {
		t.Errorf("Expected workflow success, got: %v", result.WorkflowResult.Status)
	}

	if len(result.WorkflowResult.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got: %d", len(result.WorkflowResult.Tasks))
	}

	// Check individual task results
	echoTask := result.WorkflowResult.Tasks["echo_test"]
	if echoTask == nil {
		t.Error("Expected echo_test task result")
	} else if echoTask.Status != "success" {
		t.Errorf("Expected echo_test success, got: %v", echoTask.Status)
	}

	listTask := result.WorkflowResult.Tasks["list_files"]
	if listTask == nil {
		t.Error("Expected list_files task result")
	} else if listTask.Status != "success" {
		t.Errorf("Expected list_files success, got: %v", listTask.Status)
	}
}

func TestIntegration_FileOperations(t *testing.T) {
	// Create temporary workflow file and work directory
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	workflowFile := filepath.Join(tmpDir, "file-ops.yaml")

	workflowContent := `
name: Integration Test File Operations
description: Test file creation, templating, and management

vars:
  work_dir: "` + workDir + `"
  project_name: "TestProject"
  version: "1.0.0"

tasks:
  - id: create_work_dir
    name: Create Work Directory
    type: file
    path: "{{ .vars.work_dir }}"
    state: directory
    mode: "0755"

  - id: create_config
    name: Create Config File
    type: file
    path: "{{ .vars.work_dir }}/config.json"
    state: file
    content: |
      {
        "name": "{{ .vars.project_name }}",
        "version": "{{ .vars.version }}",
        "created": "{{ now | date "2006-01-02 15:04:05" }}"
      }
    mode: "0644"
    depends_on: [create_work_dir]

  - id: create_readme
    name: Create README
    type: file
    path: "{{ .vars.work_dir }}/README.md"
    state: file
    content: |
      # {{ .vars.project_name }}

      Version: {{ .vars.version }}

      This is a test project created by Ritual integration tests.
    depends_on: [create_work_dir]

  - id: verify_files
    name: Verify Files Created
    type: command
    command: "ls -la {{ .vars.work_dir }}"
    depends_on: [create_config, create_readme]
`

	err := os.WriteFile(workflowFile, []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	// Create orchestrator
	orch := orchestrator.New(&orchestrator.Config{
		DryRun:         false,
		MaxConcurrency: 4,
		Verbose:        true,
	})

	// Execute workflow
	ctx := context.Background()
	result, err := orch.ExecuteWorkflowFile(ctx, workflowFile, nil)
	if err != nil {
		t.Errorf("Failed to execute workflow: %v", err)
	}

	// Verify workflow execution
	if hasIntegrationErrors(result) {
		t.Errorf("Workflow execution failed: %+v", result)
	}

	// Verify files were created
	configFile := filepath.Join(workDir, "config.json")
	readmeFile := filepath.Join(workDir, "README.md")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	if _, err := os.Stat(readmeFile); os.IsNotExist(err) {
		t.Error("README file was not created")
	}

	// Verify file contents
	configContent, err := os.ReadFile(configFile)
	if err != nil {
		t.Errorf("Failed to read config file: %v", err)
	} else {
		if !strings.Contains(string(configContent), "TestProject") {
			t.Error("Config file doesn't contain expected project name")
		}
	}

	readmeContent, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Errorf("Failed to read README file: %v", err)
	} else {
		if !strings.Contains(string(readmeContent), "# TestProject") {
			t.Error("README file doesn't contain expected header")
		}
	}
}

func TestIntegration_DryRunMode(t *testing.T) {
	// Create temporary workflow file
	tmpDir := t.TempDir()
	workflowFile := filepath.Join(tmpDir, "dry-run.yaml")

	workflowContent := `
name: Integration Test Dry Run
description: Test dry run functionality

tasks:
  - id: would_create_file
    name: Would Create File
    type: file
    path: "/tmp/ritual-dry-run-test-file"
    state: file
    content: "This should not be created in dry run mode"

  - id: would_run_command
    name: Would Run Command
    type: command
    command: "echo 'This would run in real mode'"
    depends_on: [would_create_file]
`

	err := os.WriteFile(workflowFile, []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	// Create orchestrator in dry-run mode
	orch := orchestrator.New(&orchestrator.Config{
		DryRun:         true,
		MaxConcurrency: 4,
		Verbose:        true,
	})

	// Execute workflow
	ctx := context.Background()
	result, err := orch.ExecuteWorkflowFile(ctx, workflowFile, nil)
	if err != nil {
		t.Errorf("Failed to execute workflow: %v", err)
	}

	// Verify workflow execution
	if hasIntegrationErrors(result) {
		t.Errorf("Workflow execution failed: %+v", result)
	}

	// Verify all tasks were skipped
	if result.WorkflowResult != nil {
		for taskID, taskResult := range result.WorkflowResult.Tasks {
			if taskResult.Status != "skipped" {
				t.Errorf("Expected task %s to be skipped in dry-run, got: %v", taskID, taskResult.Status)
			}
			if !strings.Contains(taskResult.Message, "Dry run mode") {
				t.Errorf("Expected dry run message for task %s, got: %v", taskID, taskResult.Message)
			}
		}
	}

	// Verify no actual file was created
	if _, err := os.Stat("/tmp/ritual-dry-run-test-file"); !os.IsNotExist(err) {
		t.Error("File should not have been created in dry-run mode")
		// Clean up if it somehow was created
		os.Remove("/tmp/ritual-dry-run-test-file")
	}
}

func TestIntegration_ValidationErrors(t *testing.T) {
	// Create temporary workflow file with validation errors
	tmpDir := t.TempDir()
	workflowFile := filepath.Join(tmpDir, "invalid.yaml")

	workflowContent := `
name: Integration Test Invalid Workflow
description: Test validation error handling

tasks:
  - id: invalid_command
    name: Invalid Command Task
    type: command
    # Missing required command field

  - id: invalid_dependency
    name: Invalid Dependency Task
    type: command
    command: "echo test"
    depends_on: [nonexistent_task]
`

	err := os.WriteFile(workflowFile, []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	// Create orchestrator
	orch := orchestrator.New(&orchestrator.Config{
		DryRun:         true,
		MaxConcurrency: 4,
		Verbose:        true,
	})

	// Execute workflow
	ctx := context.Background()
	result, err := orch.ExecuteWorkflowFile(ctx, workflowFile, nil)
	if err != nil {
		t.Errorf("Failed to execute workflow: %v", err)
	}

	// Verify validation errors were caught (could be parse, validation, or dependency errors)
	if len(result.ValidationErrors) == 0 && result.DependencyError == nil && result.ParseError == nil {
		t.Errorf("Expected parse, validation, or dependency errors for invalid workflow. Got: %+v", result)
	}

	// Should not have a workflow result due to validation errors
	if result.WorkflowResult != nil {
		t.Error("Should not have workflow result when validation fails")
	}
}

func TestIntegration_ParallelExecution(t *testing.T) {
	// Create temporary workflow file with parallel tasks
	tmpDir := t.TempDir()
	workflowFile := filepath.Join(tmpDir, "parallel.yaml")

	workflowContent := `
name: Integration Test Parallel Execution
description: Test parallel task execution

tasks:
  # These should run in parallel (no dependencies)
  - id: parallel_1
    name: Parallel Task 1
    type: command
    command: "echo 'Task 1 completed'"

  - id: parallel_2
    name: Parallel Task 2
    type: command
    command: "echo 'Task 2 completed'"

  - id: parallel_3
    name: Parallel Task 3
    type: command
    command: "echo 'Task 3 completed'"

  # This should run after all parallel tasks
  - id: final_task
    name: Final Task
    type: command
    command: "echo 'All parallel tasks completed'"
    depends_on: [parallel_1, parallel_2, parallel_3]
`

	err := os.WriteFile(workflowFile, []byte(workflowContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	// Create orchestrator
	orch := orchestrator.New(&orchestrator.Config{
		DryRun:         false,
		MaxConcurrency: 4,
		Verbose:        true,
	})

	// Execute workflow
	ctx := context.Background()
	result, err := orch.ExecuteWorkflowFile(ctx, workflowFile, nil)

	if err != nil {
		t.Errorf("Failed to execute workflow: %v", err)
	}

	// Verify workflow execution
	if hasIntegrationErrors(result) {
		t.Errorf("Workflow execution failed: %+v", result)
	}

	// Verify all tasks completed successfully
	if result.WorkflowResult != nil {
		if len(result.WorkflowResult.Tasks) != 4 {
			t.Errorf("Expected 4 tasks, got: %d", len(result.WorkflowResult.Tasks))
		}

		for taskID, taskResult := range result.WorkflowResult.Tasks {
			if taskResult.Status != "success" {
				t.Errorf("Expected task %s to succeed, got: %v", taskID, taskResult.Status)
			}
		}
	}

	t.Logf("Parallel execution test completed successfully")
}

func hasIntegrationErrors(result *types.Result) bool {
	if result.ParseError != nil || result.DependencyError != nil || result.ExecutionError != nil {
		return true
	}

	if len(result.ValidationErrors) > 0 {
		return true
	}

	if result.WorkflowResult != nil && result.WorkflowResult.Status == "failed" {
		return true
	}

	return false
}