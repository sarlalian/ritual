// ABOUTME: Tests for the template engine with Sprig integration
// ABOUTME: Validates template evaluation, context handling, and error cases

package template

import (
	"os"
	"strings"
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

func TestEngine_Evaluate_SimpleString(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{
		Variables: map[string]interface{}{
			"name": "world",
		},
	}

	result, err := engine.Evaluate("Hello {{ .vars.name }}!", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "Hello world!" {
		t.Errorf("Expected 'Hello world!', got '%s'", result)
	}
}

func TestEngine_Evaluate_NoTemplate(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{}

	result, err := engine.Evaluate("plain text", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "plain text" {
		t.Errorf("Expected 'plain text', got '%s'", result)
	}
}

func TestEngine_Evaluate_EmptyString(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{}

	result, err := engine.Evaluate("", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestEngine_Evaluate_Environment(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	result, err := engine.Evaluate("Value: {{ .env.TEST_VAR }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "Value: test_value" {
		t.Errorf("Expected 'Value: test_value', got '%s'", result)
	}
}

func TestEngine_Evaluate_TaskResults(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{
		Tasks: map[string]*types.TaskResult{
			"test_task": {
				ID:     "test_task",
				Name:   "test task",
				Status: types.TaskSuccess,
				Stdout: "task output",
			},
		},
	}

	result, err := engine.Evaluate("Task {{ .tasks.test_task.Name }} status: {{ .tasks.test_task.Status }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "Task test task status: success" {
		t.Errorf("Expected 'Task test task status: success', got '%s'", result)
	}
}

func TestEngine_Evaluate_SprigFunctions(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{}

	// Test upper function from Sprig
	result, err := engine.Evaluate("{{ \"hello\" | upper }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "HELLO" {
		t.Errorf("Expected 'HELLO', got '%s'", result)
	}

	// Test date function with now
	result, err = engine.Evaluate("{{ now | date \"2006\" }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should be current year (just check it's 4 digits starting with 20)
	if len(result) != 4 || !strings.HasPrefix(result, "20") {
		t.Errorf("Expected current year format, got '%s'", result)
	}
}

func TestEngine_Evaluate_CustomFunctions(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{}

	// Test env function
	_ = os.Setenv("TEST_TEMPLATE_VAR", "test_env_value")
	defer func() { _ = os.Unsetenv("TEST_TEMPLATE_VAR") }()

	result, err := engine.Evaluate("{{ env \"TEST_TEMPLATE_VAR\" }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "test_env_value" {
		t.Errorf("Expected 'test_env_value', got '%s'", result)
	}

	// Test env function with default
	result, err = engine.Evaluate("{{ env \"NON_EXISTENT_VAR\" \"default_value\" }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "default_value" {
		t.Errorf("Expected 'default_value', got '%s'", result)
	}

	// Test hostname function
	result, err = engine.Evaluate("Host: {{ hostname }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.HasPrefix(result, "Host: ") {
		t.Errorf("Expected result to start with 'Host: ', got '%s'", result)
	}
}

func TestEngine_Evaluate_InvalidTemplate(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{}

	// This should cause a parse error due to invalid syntax
	_, err := engine.Evaluate("{{ .vars.name | invalidFunction }}", ctx)

	if err == nil {
		t.Fatal("Expected error for invalid template")
	}

	if templateErr, ok := err.(*types.TemplateError); !ok {
		t.Errorf("Expected TemplateError, got %T", err)
	} else if !strings.Contains(templateErr.Message, "failed to parse template") {
		t.Errorf("Expected parse error message, got '%s'", templateErr.Message)
	}
}

func TestEngine_EvaluateAll_SimpleMap(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{
		Variables: map[string]interface{}{
			"name": "test",
		},
	}

	input := map[string]interface{}{
		"greeting": "Hello {{ .vars.name }}!",
		"number":   42,
		"boolean":  true,
	}

	result, err := engine.EvaluateAll(input, ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result["greeting"] != "Hello test!" {
		t.Errorf("Expected 'Hello test!', got '%v'", result["greeting"])
	}

	if result["number"] != 42 {
		t.Errorf("Expected 42, got %v", result["number"])
	}

	if result["boolean"] != true {
		t.Errorf("Expected true, got %v", result["boolean"])
	}
}

func TestEngine_EvaluateAll_NestedMap(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{
		Variables: map[string]interface{}{
			"name": "nested",
		},
	}

	input := map[string]interface{}{
		"config": map[string]interface{}{
			"name":  "Config for {{ .vars.name }}",
			"value": 123,
		},
	}

	result, err := engine.EvaluateAll(input, ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config, ok := result["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected config to be a map, got %T", result["config"])
	}

	if config["name"] != "Config for nested" {
		t.Errorf("Expected 'Config for nested', got '%v'", config["name"])
	}

	if config["value"] != 123 {
		t.Errorf("Expected 123, got %v", config["value"])
	}
}

func TestEngine_EvaluateAll_Array(t *testing.T) {
	engine := New()
	ctx := &types.WorkflowContext{
		Variables: map[string]interface{}{
			"index": "1",
		},
	}

	input := map[string]interface{}{
		"items": []interface{}{
			"Item {{ .vars.index }}",
			"Static item",
			42,
		},
	}

	result, err := engine.EvaluateAll(input, ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	items, ok := result["items"].([]interface{})
	if !ok {
		t.Fatalf("Expected items to be an array, got %T", result["items"])
	}

	if len(items) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(items))
	}

	if items[0] != "Item 1" {
		t.Errorf("Expected 'Item 1', got '%v'", items[0])
	}

	if items[1] != "Static item" {
		t.Errorf("Expected 'Static item', got '%v'", items[1])
	}

	if items[2] != 42 {
		t.Errorf("Expected 42, got %v", items[2])
	}
}

func TestValidateTemplate_Valid(t *testing.T) {
	tests := []string{
		"plain text",
		"",
		"{{ .vars.name }}",
		"Hello {{ .vars.name | upper }}!",
		"{{ env \"HOME\" }}",
		"{{ now | date \"2006-01-02\" }}",
	}

	for _, template := range tests {
		t.Run(template, func(t *testing.T) {
			err := ValidateTemplate(template)
			if err != nil {
				t.Errorf("Expected template '%s' to be valid, got error: %v", template, err)
			}
		})
	}
}

func TestValidateTemplate_Invalid(t *testing.T) {
	tests := []string{
		"{{ .vars.name | unknown_function }}",
		"{{ range }}",
		"{{ if }}",
	}

	for _, template := range tests {
		t.Run(template, func(t *testing.T) {
			err := ValidateTemplate(template)
			if err == nil {
				t.Errorf("Expected template '%s' to be invalid", template)
			}

			if templateErr, ok := err.(*types.TemplateError); !ok {
				t.Errorf("Expected TemplateError for invalid template '%s', got %T", template, err)
			} else if !strings.Contains(templateErr.Message, "invalid template syntax") {
				t.Errorf("Expected invalid syntax message for template '%s', got '%s'", template, templateErr.Message)
			}
		})
	}
}

func TestEvaluateString_Convenience(t *testing.T) {
	ctx := &types.WorkflowContext{
		Variables: map[string]interface{}{
			"test": "value",
		},
	}

	result, err := EvaluateString("Test: {{ .vars.test }}", ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "Test: value" {
		t.Errorf("Expected 'Test: value', got '%s'", result)
	}
}

func TestEngine_CreateTemplateData(t *testing.T) {
	engine := &Engine{}
	ctx := &types.WorkflowContext{
		Environment: map[string]string{
			"TEST_ENV": "env_value",
		},
		Variables: map[string]interface{}{
			"test_var": "var_value",
		},
		Tasks: map[string]*types.TaskResult{
			"task1": {ID: "task1", Status: types.TaskSuccess},
		},
		Metadata: map[string]interface{}{
			"workflow": map[string]interface{}{
				"name": "test-workflow",
			},
		},
	}

	data := engine.createTemplateData(ctx)

	// Check environment
	if env, ok := data["environment"].(map[string]string); !ok {
		t.Error("Expected environment to be a map[string]string")
	} else if env["TEST_ENV"] != "env_value" {
		t.Errorf("Expected TEST_ENV to be 'env_value', got '%s'", env["TEST_ENV"])
	}

	// Check variables
	if vars, ok := data["variables"].(map[string]interface{}); !ok {
		t.Error("Expected variables to be a map[string]interface{}")
	} else if vars["test_var"] != "var_value" {
		t.Errorf("Expected test_var to be 'var_value', got '%v'", vars["test_var"])
	}

	// Check shorthand aliases exist
	if data["env"] == nil {
		t.Error("Expected 'env' alias to exist")
	}

	if data["vars"] == nil {
		t.Error("Expected 'vars' alias to exist")
	}

	// Check tasks
	if tasks, ok := data["tasks"].(map[string]*types.TaskResult); !ok {
		t.Error("Expected tasks to be a map[string]*types.TaskResult")
	} else if tasks["task1"].Status != types.TaskSuccess {
		t.Errorf("Expected task1 Status to be 'success', got '%s'", tasks["task1"].Status)
	}

	// Check workflow metadata
	if workflow, ok := data["workflow"].(map[string]interface{}); !ok {
		t.Error("Expected workflow to be a map[string]interface{}")
	} else if workflow["name"] != "test-workflow" {
		t.Errorf("Expected workflow name to be 'test-workflow', got '%v'", workflow["name"])
	}
}
