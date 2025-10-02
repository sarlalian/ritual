// ABOUTME: Tests for the YAML workflow parser
// ABOUTME: Validates parsing logic, error handling, and edge cases

package parser

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
	"github.com/spf13/afero"
)

func TestParser_Parse_ValidWorkflow(t *testing.T) {
	yamlContent := `
name: test-workflow
version: "1.0"
description: "Test workflow"
mode: parallel

environment:
  TEST_VAR: "test_value"

vars:
  test_var: "test"

tasks:
  - name: hello-world
    command:
      cmd: "echo hello"
    register: hello_result

  - name: dependent-task
    command:
      cmd: "echo world"
    depends_on: [hello-world]
`

	parser := New(nil)
	workflow, err := parser.Parse([]byte(yamlContent))

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if workflow.Name != "test-workflow" {
		t.Errorf("Expected name 'test-workflow', got '%s'", workflow.Name)
	}

	if workflow.Mode != types.ParallelMode {
		t.Errorf("Expected mode 'parallel', got '%s'", workflow.Mode)
	}

	if len(workflow.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(workflow.Tasks))
	}

	// Check first task
	task1 := workflow.Tasks[0]
	if task1.Name != "hello-world" {
		t.Errorf("Expected first task name 'hello-world', got '%s'", task1.Name)
	}
	if task1.Type != "command" {
		t.Errorf("Expected first task type 'command', got '%s'", task1.Type)
	}
	if task1.ID == "" {
		t.Error("Expected task ID to be generated")
	}

	// Check second task
	task2 := workflow.Tasks[1]
	if len(task2.DependsOn) != 1 || task2.DependsOn[0] != "hello-world" {
		t.Errorf("Expected second task to depend on 'hello-world', got %v", task2.DependsOn)
	}
}

func TestParser_Parse_InvalidYAML(t *testing.T) {
	yamlContent := `
name: test-workflow
tasks:
  - name: test
    invalid_yaml: [
`

	parser := New(nil)
	_, err := parser.Parse([]byte(yamlContent))

	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}

	if parseErr, ok := err.(*types.ParseError); !ok {
		t.Errorf("Expected ParseError, got %T", err)
	} else if parseErr.Message != "failed to parse YAML" {
		t.Errorf("Expected parse error message, got '%s'", parseErr.Message)
	}
}

func TestParser_Validate_MissingName(t *testing.T) {
	workflow := &types.Workflow{
		Tasks: []types.TaskConfig{
			{Name: "test", Config: map[string]interface{}{"command": map[string]interface{}{"cmd": "echo test"}}},
		},
	}

	parser := New(nil)
	err := parser.Validate(workflow)

	if err == nil {
		t.Fatal("Expected validation error for missing name")
	}

	if validationErr, ok := err.(*types.ValidationError); !ok {
		t.Errorf("Expected ValidationError, got %T", err)
	} else if validationErr.Field != "name" {
		t.Errorf("Expected field 'name', got '%s'", validationErr.Field)
	}
}

func TestParser_Validate_NoTasks(t *testing.T) {
	workflow := &types.Workflow{
		Name:  "test",
		Tasks: []types.TaskConfig{},
	}

	parser := New(nil)
	err := parser.Validate(workflow)

	if err == nil {
		t.Fatal("Expected validation error for no tasks")
	}

	if validationErr, ok := err.(*types.ValidationError); !ok {
		t.Errorf("Expected ValidationError, got %T", err)
	} else if validationErr.Field != "tasks" {
		t.Errorf("Expected field 'tasks', got '%s'", validationErr.Field)
	}
}

func TestParser_Validate_InvalidDependency(t *testing.T) {
	workflow := &types.Workflow{
		Name: "test",
		Tasks: []types.TaskConfig{
			{
				Name:      "test-task",
				Config:    map[string]interface{}{"command": map[string]interface{}{"cmd": "echo test"}},
				DependsOn: []string{"non-existent-task"},
			},
		},
	}

	parser := New(nil)
	err := parser.Validate(workflow)

	if err == nil {
		t.Fatal("Expected validation error for invalid dependency")
	}

	if depErr, ok := err.(*types.DependencyError); !ok {
		t.Errorf("Expected DependencyError, got %T", err)
	} else if depErr.TaskID != "test_task" {
		t.Errorf("Expected task ID 'test_task', got '%s'", depErr.TaskID)
	}
}

func TestParser_InferTaskType(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		expectedType string
	}{
		{
			name:         "command task",
			config:       map[string]interface{}{"command": map[string]interface{}{"cmd": "echo hello"}},
			expectedType: "command",
		},
		{
			name:         "file task",
			config:       map[string]interface{}{"file": map[string]interface{}{"path": "/tmp/test", "state": "present"}},
			expectedType: "file",
		},
		{
			name:         "compress task",
			config:       map[string]interface{}{"compress": map[string]interface{}{"src": "file.txt", "format": "gzip"}},
			expectedType: "compress",
		},
		{
			name:         "checksum task",
			config:       map[string]interface{}{"checksum": map[string]interface{}{"file": "test.txt", "algorithm": "sha256"}},
			expectedType: "checksum",
		},
		{
			name:         "unknown task",
			config:       map[string]interface{}{"unknown_field": "value"},
			expectedType: "",
		},
	}

	parser := &Parser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &types.TaskConfig{Config: tt.config}
			result := parser.inferTaskType(task)
			if result != tt.expectedType {
				t.Errorf("Expected type '%s', got '%s'", tt.expectedType, result)
			}
		})
	}
}

func TestParser_GenerateTaskID(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		name       string
		index      int
		expectedID string
	}{
		{"hello world", 0, "hello_world"},
		{"Test-Task", 1, "test_task"},
		{"UPPERCASE", 2, "uppercase"},
		{"", 3, "task_3"},
		{"123numbers", 4, "123numbers"},
		{"special!@#chars", 5, "specialchars"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.generateTaskID(tt.name, tt.index)
			if result != tt.expectedID {
				t.Errorf("Expected ID '%s', got '%s'", tt.expectedID, result)
			}
		})
	}
}

func TestParser_ParseFile_NotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	parser := New(fs)

	_, err := parser.ParseFile("non-existent.yaml")

	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}

	if parseErr, ok := err.(*types.ParseError); !ok {
		t.Errorf("Expected ParseError, got %T", err)
	} else if parseErr.File != "non-existent.yaml" {
		t.Errorf("Expected filename in error, got '%s'", parseErr.File)
	}
}

func TestParser_ParseFile_Success(t *testing.T) {
	yamlContent := `
name: file-test
tasks:
  - name: test-task
    command:
      cmd: "echo test"
`

	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "test.yaml", []byte(yamlContent), 0644)

	parser := New(fs)
	workflow, err := parser.ParseFile("test.yaml")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if workflow.Name != "file-test" {
		t.Errorf("Expected name 'file-test', got '%s'", workflow.Name)
	}
}

func TestValidateFileStructure(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{"invalid extension", "test.txt", true},
		{"no extension", "test", true},
		{"valid yaml extension", "test.yaml", false},
		{"valid yml extension", "test.yml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test just the extension validation logic
			ext := strings.ToLower(filepath.Ext(tt.filename))
			hasValidExt := ext == ".yaml" || ext == ".yml"

			if hasValidExt == tt.expectError {
				if tt.expectError {
					t.Errorf("Expected error for filename '%s', but extension '%s' is valid", tt.filename, ext)
				} else {
					t.Errorf("Expected no error for filename '%s', but extension '%s' is invalid", tt.filename, ext)
				}
			}
		})
	}
}
