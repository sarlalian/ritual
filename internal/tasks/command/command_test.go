// ABOUTME: Tests for the command task executor with various execution scenarios
// ABOUTME: Validates command execution, error handling, and output capture

package command

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

// MockContextManager for testing
type MockContextManager struct {
	variables map[string]interface{}
}

func NewMockContextManager() *MockContextManager {
	return &MockContextManager{
		variables: make(map[string]interface{}),
	}
}

func (m *MockContextManager) EvaluateMap(data map[string]interface{}) (map[string]interface{}, error) {
	// Simple mock - just return the data as-is
	return data, nil
}

func (m *MockContextManager) EvaluateString(templateStr string) (string, error) {
	return templateStr, nil
}

func (m *MockContextManager) Initialize(workflow *types.Workflow, envVars []string) error {
	return nil
}

func (m *MockContextManager) GetContext() *types.WorkflowContext {
	return &types.WorkflowContext{}
}

func (m *MockContextManager) GetVariable(name string) (interface{}, error) {
	return nil, nil
}

func (m *MockContextManager) SetVariable(name string, value interface{}) error {
	return nil
}

func (m *MockContextManager) GetEnvironment(name, defaultValue string) string {
	return defaultValue
}

func (m *MockContextManager) SetEnvironment(name, value string) error {
	return nil
}

func (m *MockContextManager) RegisterTaskResult(taskResult *types.TaskResult) error {
	return nil
}

func (m *MockContextManager) GetTaskResult(identifier string) (*types.TaskResult, error) {
	return nil, nil
}

func (m *MockContextManager) GetTemplateEngine() types.TemplateEngine {
	return &MockTemplateEngine{}
}

func (m *MockContextManager) Clone() types.ContextManager {
	return NewMockContextManager()
}

// MockTemplateEngine for testing
type MockTemplateEngine struct{}

func (m *MockTemplateEngine) Evaluate(template string, ctx *types.WorkflowContext) (string, error) {
	return template, nil
}

func (m *MockTemplateEngine) EvaluateAll(data map[string]interface{}, ctx *types.WorkflowContext) (map[string]interface{}, error) {
	return data, nil
}

func TestExecutor_New(t *testing.T) {
	executor := New()
	if executor == nil {
		t.Error("Expected executor to be created")
	}
}

func TestExecutor_SupportsDryRun(t *testing.T) {
	executor := New()
	if !executor.SupportsDryRun() {
		t.Error("Expected command executor to support dry run")
	}
}

func TestExecutor_ValidateConfig_Valid(t *testing.T) {
	executor := New()

	validConfigs := []map[string]interface{}{
		{
			"command": "echo hello",
		},
		{
			"script": "echo hello world",
		},
		{
			"command": "ls",
			"args":    []interface{}{"-la"},
		},
		{
			"command":     "sleep 1",
			"timeout":     "5s",
			"working_dir": "/tmp",
		},
	}

	for i, config := range validConfigs {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			task := &types.TaskConfig{
				ID:     "test",
				Name:   "Test Task",
				Type:   "command",
				Config: config,
			}

			err := executor.Validate(task)
			if err != nil {
				t.Errorf("Expected valid config to pass validation, got: %v", err)
			}
		})
	}
}

func TestExecutor_ValidateConfig_Invalid(t *testing.T) {
	executor := New()

	invalidConfigs := []struct {
		config map[string]interface{}
		reason string
	}{
		{
			config: map[string]interface{}{},
			reason: "missing command and script",
		},
		{
			config: map[string]interface{}{
				"command": "echo hello",
				"script":  "echo world",
			},
			reason: "both command and script specified",
		},
		{
			config: map[string]interface{}{
				"command": "echo hello",
				"timeout": "invalid",
			},
			reason: "invalid timeout format",
		},
		{
			config: map[string]interface{}{
				"command": 123,
			},
			reason: "non-string command",
		},
	}

	for i, test := range invalidConfigs {
		t.Run(test.reason, func(t *testing.T) {
			task := &types.TaskConfig{
				ID:     "test",
				Name:   "Test Task",
				Type:   "command",
				Config: test.config,
			}

			err := executor.Validate(task)
			if err == nil {
				t.Errorf("Expected config %d to fail validation (%s)", i, test.reason)
			}
		})
	}
}

func TestExecutor_Execute_SimpleCommand(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Command",
		Type: "command",
		Config: map[string]interface{}{
			"command": getTestCommand("echo", "hello world"),
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("Expected stdout to contain 'hello world', got: %s", result.Stdout)
	}

	if result.ReturnCode != 0 {
		t.Errorf("Expected return code 0, got %d", result.ReturnCode)
	}
}

func TestExecutor_Execute_Script(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Script",
		Type: "command",
		Config: map[string]interface{}{
			"script": getTestScript("echo 'script output'"),
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if !strings.Contains(result.Stdout, "script output") {
		t.Errorf("Expected stdout to contain 'script output', got: %s", result.Stdout)
	}
}

func TestExecutor_Execute_WithArgs(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Command with Args",
		Type: "command",
		Config: map[string]interface{}{
			"command": getTestCommandName("echo"),
			"args":    []interface{}{"hello", "from", "args"},
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if !strings.Contains(result.Stdout, "hello from args") {
		t.Errorf("Expected stdout to contain 'hello from args', got: %s", result.Stdout)
	}
}

func TestExecutor_Execute_WorkingDirectory(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	// Create a temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Working Directory",
		Type: "command",
		Config: map[string]interface{}{
			"command":     getTestCommand("ls", "testfile.txt"),
			"working_dir": tmpDir,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if !strings.Contains(result.Stdout, "testfile.txt") {
		t.Errorf("Expected stdout to contain 'testfile.txt', got: %s", result.Stdout)
	}
}

func TestExecutor_Execute_Environment(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Environment",
		Type: "command",
		Config: map[string]interface{}{
			"script": getTestScript("echo $TEST_VAR"),
			"environment": map[string]interface{}{
				"TEST_VAR": "test_value",
			},
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if !strings.Contains(result.Stdout, "test_value") {
		t.Errorf("Expected stdout to contain 'test_value', got: %s", result.Stdout)
	}
}

func TestExecutor_Execute_Failure(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Failure",
		Type: "command",
		Config: map[string]interface{}{
			"command": getTestCommand("false"), // Command that always fails
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure, got %s", result.Status)
	}

	if result.ReturnCode == 0 {
		t.Error("Expected non-zero return code for failed command")
	}
}

func TestExecutor_Execute_FailureIgnored(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Failure Ignored",
		Type: "command",
		Config: map[string]interface{}{
			"command":       getTestCommand("false"), // Command that always fails
			"fail_on_error": false,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success (failure ignored), got %s", result.Status)
	}

	if result.ReturnCode == 0 {
		t.Error("Expected non-zero return code even when failure is ignored")
	}

	if !strings.Contains(result.Message, "ignored") {
		t.Errorf("Expected message to indicate failure was ignored, got: %s", result.Message)
	}
}

func TestExecutor_Execute_Timeout(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Timeout",
		Type: "command",
		Config: map[string]interface{}{
			"command": getTestCommand("sleep", "2"),
			"timeout": "500ms",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure due to timeout, got %s", result.Status)
	}

	if !strings.Contains(result.Message, "timed out") {
		t.Errorf("Expected timeout message, got: %s", result.Message)
	}
}

func TestExecutor_Execute_InvalidWorkingDir(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Invalid Working Dir",
		Type: "command",
		Config: map[string]interface{}{
			"command":     getTestCommand("echo", "hello"),
			"working_dir": "/nonexistent/directory",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for invalid working dir, got %s", result.Status)
	}

	if !strings.Contains(result.Message, "does not exist") {
		t.Errorf("Expected working directory error message, got: %s", result.Message)
	}
}

func TestExecutor_Execute_CaptureConfig(t *testing.T) {
	executor := New()
	contextManager := NewMockContextManager()

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Test Capture Config",
		Type: "command",
		Config: map[string]interface{}{
			"script": getTestScript("echo 'stdout message'; echo 'stderr message' >&2"),
			"capture": map[string]interface{}{
				"stdout": true,
				"stderr": true,
			},
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if !strings.Contains(result.Stdout, "stdout message") {
		t.Errorf("Expected stdout to be captured, got: %s", result.Stdout)
	}

	if !strings.Contains(result.Stderr, "stderr message") {
		t.Errorf("Expected stderr to be captured, got: %s", result.Stderr)
	}
}

// Helper functions for cross-platform testing

func getTestCommand(cmd string, args ...string) string {
	if runtime.GOOS == "windows" {
		switch cmd {
		case "echo":
			return "cmd /c echo " + strings.Join(args, " ")
		case "false":
			return "cmd /c exit 1"
		case "sleep":
			if len(args) > 0 {
				return "cmd /c timeout /t " + args[0] + " >nul"
			}
			return "cmd /c timeout /t 1 >nul"
		case "ls":
			return "cmd /c dir /b " + strings.Join(args, " ")
		}
		return "cmd /c " + cmd + " " + strings.Join(args, " ")
	}
	return cmd + " " + strings.Join(args, " ")
}

func getTestCommandName(cmd string) string {
	if runtime.GOOS == "windows" {
		switch cmd {
		case "echo":
			return "cmd"
		}
	}
	return cmd
}

func getTestScript(script string) string {
	if runtime.GOOS == "windows" {
		// Convert Unix-style script to Windows batch
		script = strings.ReplaceAll(script, "echo '", "echo ")
		script = strings.ReplaceAll(script, "'", "")
		script = strings.ReplaceAll(script, "$TEST_VAR", "%TEST_VAR%")
		script = strings.ReplaceAll(script, " >&2", " 1>&2")
	}
	return script
}
