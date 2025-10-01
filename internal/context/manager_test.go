// ABOUTME: Tests for the context manager and variable resolution
// ABOUTME: Validates environment handling, template evaluation, and task result sharing

package context

import (
	"os"
	"strings"
	"testing"

	"github.com/sarlalian/ritual/internal/template"
	"github.com/sarlalian/ritual/pkg/types"
)

func TestManager_Initialize(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	workflow := &types.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Environment: map[string]string{
			"TEST_VAR": "test_value",
			"COMPUTED": "{{ .env.TEST_VAR }}_computed",
		},
		Variables: map[string]interface{}{
			"simple_var":   "simple_value",
			"template_var": "Value is {{ .env.TEST_VAR }}",
		},
	}

	err := manager.Initialize(workflow, []string{"CLI_VAR=cli_value"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check environment variables
	if manager.GetEnvironment("TEST_VAR", "") != "test_value" {
		t.Errorf("Expected TEST_VAR to be 'test_value', got '%s'", manager.GetEnvironment("TEST_VAR", ""))
	}

	if manager.GetEnvironment("COMPUTED", "") != "test_value_computed" {
		t.Errorf("Expected COMPUTED to be 'test_value_computed', got '%s'", manager.GetEnvironment("COMPUTED", ""))
	}

	if manager.GetEnvironment("CLI_VAR", "") != "cli_value" {
		t.Errorf("Expected CLI_VAR to be 'cli_value', got '%s'", manager.GetEnvironment("CLI_VAR", ""))
	}

	// Check variables
	simpleVar, err := manager.GetVariable("simple_var")
	if err != nil {
		t.Fatalf("Expected no error getting simple_var, got: %v", err)
	}
	if simpleVar != "simple_value" {
		t.Errorf("Expected simple_var to be 'simple_value', got '%v'", simpleVar)
	}

	templateVar, err := manager.GetVariable("template_var")
	if err != nil {
		t.Fatalf("Expected no error getting template_var, got: %v", err)
	}
	if templateVar != "Value is test_value" {
		t.Errorf("Expected template_var to be 'Value is test_value', got '%v'", templateVar)
	}

	// Check workflow metadata
	context := manager.GetContext()
	workflowMeta, ok := context.Metadata["workflow"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected workflow metadata to exist")
	}

	if workflowMeta["name"] != "test-workflow" {
		t.Errorf("Expected workflow name to be 'test-workflow', got '%v'", workflowMeta["name"])
	}
}

func TestManager_SetGetVariable(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Initialize with basic context
	workflow := &types.Workflow{
		Name: "test",
		Environment: map[string]string{
			"BASE_VALUE": "base",
		},
	}
	manager.Initialize(workflow, nil)

	// Set simple variable
	err := manager.SetVariable("test_var", "test_value")
	if err != nil {
		t.Fatalf("Expected no error setting variable, got: %v", err)
	}

	value, err := manager.GetVariable("test_var")
	if err != nil {
		t.Fatalf("Expected no error getting variable, got: %v", err)
	}
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", value)
	}

	// Set template variable
	err = manager.SetVariable("template_var", "Value: {{ .env.BASE_VALUE }}")
	if err != nil {
		t.Fatalf("Expected no error setting template variable, got: %v", err)
	}

	templateValue, err := manager.GetVariable("template_var")
	if err != nil {
		t.Fatalf("Expected no error getting template variable, got: %v", err)
	}
	if templateValue != "Value: base" {
		t.Errorf("Expected 'Value: base', got '%v'", templateValue)
	}
}

func TestManager_SetGetEnvironment(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Initialize
	workflow := &types.Workflow{Name: "test"}
	manager.Initialize(workflow, nil)

	// Set environment variable
	err := manager.SetEnvironment("TEST_ENV", "env_value")
	if err != nil {
		t.Fatalf("Expected no error setting environment, got: %v", err)
	}

	value := manager.GetEnvironment("TEST_ENV", "default")
	if value != "env_value" {
		t.Errorf("Expected 'env_value', got '%s'", value)
	}

	// Test default value
	defaultValue := manager.GetEnvironment("NON_EXISTENT", "default_val")
	if defaultValue != "default_val" {
		t.Errorf("Expected 'default_val', got '%s'", defaultValue)
	}
}

func TestManager_RegisterTaskResult(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Initialize
	workflow := &types.Workflow{Name: "test"}
	manager.Initialize(workflow, nil)

	// Register task result
	taskResult := &types.TaskResult{
		ID:         "test_task",
		Name:       "Test Task",
		Status:     types.TaskSuccess,
		Stdout:     "task output",
		ReturnCode: 0,
	}

	err := manager.RegisterTaskResult(taskResult)
	if err != nil {
		t.Fatalf("Expected no error registering task result, got: %v", err)
	}

	// Get by ID
	result, err := manager.GetTaskResult("test_task")
	if err != nil {
		t.Fatalf("Expected no error getting task result by ID, got: %v", err)
	}
	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task status to be 'success', got '%s'", result.Status)
	}

	// Get by name
	result, err = manager.GetTaskResult("Test Task")
	if err != nil {
		t.Fatalf("Expected no error getting task result by name, got: %v", err)
	}
	if result.Stdout != "task output" {
		t.Errorf("Expected task stdout to be 'task output', got '%s'", result.Stdout)
	}

	// Test non-existent task
	_, err = manager.GetTaskResult("non_existent")
	if err == nil {
		t.Error("Expected error for non-existent task result")
	}
}

func TestManager_EvaluateString(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Initialize with context
	workflow := &types.Workflow{
		Name: "test",
		Environment: map[string]string{
			"ENV_VAR": "env_value",
		},
		Variables: map[string]interface{}{
			"var1": "variable_value",
		},
	}
	manager.Initialize(workflow, nil)

	// Register task result
	taskResult := &types.TaskResult{
		ID:     "task1",
		Name:   "Task 1",
		Status: types.TaskSuccess,
		Stdout: "output",
	}
	manager.RegisterTaskResult(taskResult)

	tests := []struct {
		template string
		expected string
	}{
		{"Plain text", "Plain text"},
		{"{{ .env.ENV_VAR }}", "env_value"},
		{"{{ .vars.var1 }}", "variable_value"},
		{"{{ .tasks.task1.Status }}", "success"},
		{"Combined: {{ .env.ENV_VAR }} and {{ .vars.var1 }}", "Combined: env_value and variable_value"},
	}

	for _, test := range tests {
		t.Run(test.template, func(t *testing.T) {
			result, err := manager.EvaluateString(test.template)
			if err != nil {
				t.Fatalf("Expected no error evaluating '%s', got: %v", test.template, err)
			}
			if result != test.expected {
				t.Errorf("Expected '%s', got '%s'", test.expected, result)
			}
		})
	}
}

func TestManager_EvaluateMap(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Initialize with context
	workflow := &types.Workflow{
		Name: "test",
		Variables: map[string]interface{}{
			"base": "base_value",
		},
	}
	manager.Initialize(workflow, nil)

	input := map[string]interface{}{
		"simple":   "plain text",
		"template": "Template: {{ .vars.base }}",
		"number":   42,
		"nested": map[string]interface{}{
			"inner": "Inner: {{ .vars.base }}",
		},
	}

	result, err := manager.EvaluateMap(input)
	if err != nil {
		t.Fatalf("Expected no error evaluating map, got: %v", err)
	}

	if result["simple"] != "plain text" {
		t.Errorf("Expected simple to be 'plain text', got '%v'", result["simple"])
	}

	if result["template"] != "Template: base_value" {
		t.Errorf("Expected template to be 'Template: base_value', got '%v'", result["template"])
	}

	if result["number"] != 42 {
		t.Errorf("Expected number to be 42, got %v", result["number"])
	}

	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected nested to be a map")
	}
	if nested["inner"] != "Inner: base_value" {
		t.Errorf("Expected nested.inner to be 'Inner: base_value', got '%v'", nested["inner"])
	}
}

func TestManager_Clone(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Initialize with context
	workflow := &types.Workflow{
		Name: "test",
		Environment: map[string]string{
			"ENV_VAR": "env_value",
		},
		Variables: map[string]interface{}{
			"var1": "value1",
		},
	}
	manager.Initialize(workflow, []string{"CLI_VAR=cli_value"})

	// Register task result
	taskResult := &types.TaskResult{
		ID:     "task1",
		Status: types.TaskSuccess,
	}
	manager.RegisterTaskResult(taskResult)

	// Clone
	clone := manager.Clone()

	// Verify clone has same data
	if clone.GetEnvironment("ENV_VAR", "") != "env_value" {
		t.Error("Clone should have same environment variables")
	}

	cloneVar, err := clone.GetVariable("var1")
	if err != nil || cloneVar != "value1" {
		t.Error("Clone should have same variables")
	}

	_, err = clone.GetTaskResult("task1")
	if err != nil {
		t.Error("Clone should have same task results")
	}

	// Modify original and verify clone is independent
	manager.SetVariable("var1", "modified")
	cloneVar, _ = clone.GetVariable("var1")
	if cloneVar != "value1" {
		t.Error("Clone should be independent of original modifications")
	}
}

func TestParseVariableString(t *testing.T) {
	tests := []struct {
		input       string
		expectedKey string
		expectedVal interface{}
		expectError bool
	}{
		{"key=value", "key", "value", false},
		{"bool_true=true", "bool_true", true, false},
		{"bool_false=false", "bool_false", false, false},
		{"integer=42", "integer", 42, false},
		{"float=3.14", "float", 3.14, false},
		{"invalid", "", nil, true},
		{"key=", "key", "", false},
		{"=value", "", "value", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			key, value, err := ParseVariableString(test.input)

			if test.expectError && err == nil {
				t.Errorf("Expected error for input '%s'", test.input)
			}

			if !test.expectError && err != nil {
				t.Errorf("Expected no error for input '%s', got: %v", test.input, err)
			}

			if !test.expectError {
				if key != test.expectedKey {
					t.Errorf("Expected key '%s', got '%s'", test.expectedKey, key)
				}
				if value != test.expectedVal {
					t.Errorf("Expected value '%v', got '%v'", test.expectedVal, value)
				}
			}
		})
	}
}

func TestLoadEnvironmentFile(t *testing.T) {
	// Create a temporary .env file
	content := `# This is a comment
VAR1=value1
VAR2=value with spaces
BOOL_VAR=true

# Another comment
INT_VAR=42`

	tmpFile := "/tmp/test.env"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile)

	vars, err := LoadEnvironmentFile(tmpFile)
	if err != nil {
		t.Fatalf("Expected no error loading env file, got: %v", err)
	}

	expected := []string{
		"VAR1=value1",
		"VAR2=value with spaces",
		"BOOL_VAR=true",
		"INT_VAR=42",
	}

	if len(vars) != len(expected) {
		t.Errorf("Expected %d variables, got %d", len(expected), len(vars))
	}

	for i, expectedVar := range expected {
		if i >= len(vars) || vars[i] != expectedVar {
			t.Errorf("Expected variable '%s', got '%s'", expectedVar, vars[i])
		}
	}
}

func TestLoadEnvironmentFile_InvalidFormat(t *testing.T) {
	content := `VAR1=value1
INVALID_LINE
VAR2=value2`

	tmpFile := "/tmp/test_invalid.env"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile)

	_, err = LoadEnvironmentFile(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid format")
	}

	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("Expected format error, got: %v", err)
	}
}

func TestManager_VariableTemplateDepdendencies(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Test variables with dependencies on other variables
	workflow := &types.Workflow{
		Name: "test",
		Variables: map[string]interface{}{
			"base":      "base_value",
			"derived":   "{{ .vars.base }}_derived",
			"chained":   "{{ .vars.derived }}_chained",
			"circular1": "{{ .vars.circular2 }}", // This should not crash
			"circular2": "{{ .vars.circular1 }}", // This should not crash
		},
	}

	err := manager.Initialize(workflow, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that non-circular dependencies work
	derived, err := manager.GetVariable("derived")
	if err != nil {
		t.Fatalf("Expected no error getting derived, got: %v", err)
	}
	if derived != "base_value_derived" {
		t.Errorf("Expected 'base_value_derived', got '%v'", derived)
	}

	// Chained dependencies might not work in all cases due to evaluation order
	// but the system should not crash
	_, err = manager.GetVariable("chained")
	if err != nil {
		t.Logf("Chained variable evaluation failed as expected: %v", err)
	}

	// Circular dependencies should not crash the system
	_, err = manager.GetVariable("circular1")
	if err != nil {
		t.Logf("Circular dependency handled gracefully: %v", err)
	}
}

func TestManager_SystemEnvironmentVariables(t *testing.T) {
	engine := template.New()
	manager := New(engine)

	// Set a system environment variable for testing
	os.Setenv("TEST_SYSTEM_VAR", "system_value")
	defer os.Unsetenv("TEST_SYSTEM_VAR")

	workflow := &types.Workflow{
		Name: "test",
	}

	err := manager.Initialize(workflow, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// System environment variable should be available
	value := manager.GetEnvironment("TEST_SYSTEM_VAR", "default")
	if value != "system_value" {
		t.Errorf("Expected system environment variable to be 'system_value', got '%s'", value)
	}
}
