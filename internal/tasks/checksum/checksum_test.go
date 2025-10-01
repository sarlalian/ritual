// ABOUTME: Tests for checksum task executor
// ABOUTME: Validates hash calculation and verification across multiple algorithms

package checksum

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

// MockContextManager for testing
type MockContextManager struct{}

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

func (m *MockContextManager) EvaluateString(templateStr string) (string, error) {
	return templateStr, nil
}

func (m *MockContextManager) EvaluateMap(data map[string]interface{}) (map[string]interface{}, error) {
	return data, nil
}

func (m *MockContextManager) GetTemplateEngine() types.TemplateEngine {
	return &MockTemplateEngine{}
}

func (m *MockContextManager) Clone() types.ContextManager {
	return &MockContextManager{}
}

// MockTemplateEngine for testing
type MockTemplateEngine struct{}

func (m *MockTemplateEngine) Evaluate(template string, ctx *types.WorkflowContext) (string, error) {
	return template, nil
}

func (m *MockTemplateEngine) EvaluateAll(data map[string]interface{}, ctx *types.WorkflowContext) (map[string]interface{}, error) {
	return data, nil
}

func TestChecksum_Calculate_SHA256(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	executor := New()
	task := &types.TaskConfig{
		ID:   "test-checksum",
		Name: "Test Checksum",
		Type: "checksum",
		Config: map[string]interface{}{
			"path":      testFile,
			"algorithm": "sha256",
			"action":    "calculate",
		},
	}

	ctx := context.Background()
	contextManager := &MockContextManager{}
	result := executor.Execute(ctx, task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if result.Output["checksum"] == "" {
		t.Error("Expected checksum in output")
	}

	// SHA256 of "Hello, World!" is dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f
	expectedChecksum := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	if result.Output["checksum"] != expectedChecksum {
		t.Errorf("Expected checksum %s, got %s", expectedChecksum, result.Output["checksum"])
	}
}

func TestChecksum_Verify_Success(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	executor := New()
	task := &types.TaskConfig{
		ID:   "test-checksum",
		Name: "Test Checksum",
		Type: "checksum",
		Config: map[string]interface{}{
			"path":      testFile,
			"algorithm": "sha256",
			"action":    "verify",
			"expected":  "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
		},
	}

	ctx := context.Background()
	contextManager := &MockContextManager{}
	result := executor.Execute(ctx, task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	if verified, ok := result.Output["verified"].(bool); !ok || !verified {
		t.Error("Expected verification to pass")
	}
}

func TestChecksum_Verify_Failure(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	executor := New()
	task := &types.TaskConfig{
		ID:   "test-checksum",
		Name: "Test Checksum",
		Type: "checksum",
		Config: map[string]interface{}{
			"path":      testFile,
			"algorithm": "sha256",
			"action":    "verify",
			"expected":  "wrongchecksum",
		},
	}

	ctx := context.Background()
	contextManager := &MockContextManager{}
	result := executor.Execute(ctx, task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure, got %s", result.Status)
	}

	if verified, ok := result.Output["verified"].(bool); !ok || verified {
		t.Error("Expected verification to fail")
	}
}

func TestChecksum_MD5(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	executor := New()
	task := &types.TaskConfig{
		ID:   "test-checksum",
		Name: "Test Checksum",
		Type: "checksum",
		Config: map[string]interface{}{
			"path":      testFile,
			"algorithm": "md5",
		},
	}

	ctx := context.Background()
	contextManager := &MockContextManager{}
	result := executor.Execute(ctx, task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// MD5 of "Hello, World!" is 65a8e27d8879283831b664bd8b7f0ad4
	expectedChecksum := "65a8e27d8879283831b664bd8b7f0ad4"
	if result.Output["checksum"] != expectedChecksum {
		t.Errorf("Expected checksum %s, got %s", expectedChecksum, result.Output["checksum"])
	}
}

func TestChecksum_Validate(t *testing.T) {
	executor := New()

	tests := []struct {
		name      string
		task      *types.TaskConfig
		shouldErr bool
	}{
		{
			name: "Valid SHA256",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"path":      "/tmp/test.txt",
					"algorithm": "sha256",
				},
			},
			shouldErr: false,
		},
		{
			name: "Missing path",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"algorithm": "sha256",
				},
			},
			shouldErr: true,
		},
		{
			name: "Invalid algorithm",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"path":      "/tmp/test.txt",
					"algorithm": "invalid",
				},
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.Validate(tt.task)
			if tt.shouldErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestChecksum_SupportsDryRun(t *testing.T) {
	executor := New()
	if !executor.SupportsDryRun() {
		t.Error("Expected checksum executor to support dry run")
	}
}
