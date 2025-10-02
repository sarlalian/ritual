// ABOUTME: Tests for copy task executor
// ABOUTME: Validates file and directory copying across filesystems

package copy

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

func TestExecutor_New(t *testing.T) {
	executor := New()
	if executor == nil {
		t.Error("Expected executor to be created")
	}
}

func TestExecutor_SupportsDryRun(t *testing.T) {
	executor := New()
	if !executor.SupportsDryRun() {
		t.Error("Expected copy executor to support dry run")
	}
}

func TestExecutor_Validate_Valid(t *testing.T) {
	executor := New()

	validConfigs := []map[string]interface{}{
		{
			"src":  "/tmp/source.txt",
			"dest": "/tmp/dest.txt",
		},
		{
			"source":      "/tmp/source",
			"destination": "/tmp/dest",
			"recursive":   true,
		},
		{
			"src":        "/tmp/file.txt",
			"dest":       "/tmp/copy.txt",
			"force":      true,
			"backup":     true,
			"backup_ext": ".backup",
		},
		{
			"src":  "/tmp/file.txt",
			"dest": "/tmp/copy.txt",
			"mode": "0644",
		},
	}

	for i, config := range validConfigs {
		task := &types.TaskConfig{
			ID:     "test",
			Name:   "Test Copy",
			Type:   "copy",
			Config: config,
		}

		err := executor.Validate(task)
		if err != nil {
			t.Errorf("Config %d: Expected valid config to pass validation, got: %v", i, err)
		}
	}
}

func TestExecutor_Validate_Invalid(t *testing.T) {
	executor := New()

	invalidConfigs := []struct {
		config map[string]interface{}
		reason string
	}{
		{
			config: map[string]interface{}{
				"dest": "/tmp/dest.txt",
			},
			reason: "missing source",
		},
		{
			config: map[string]interface{}{
				"src": "/tmp/source.txt",
			},
			reason: "missing destination",
		},
		{
			config: map[string]interface{}{},
			reason: "missing both source and destination",
		},
		{
			config: map[string]interface{}{
				"src":  "/tmp/source.txt",
				"dest": "/tmp/dest.txt",
				"mode": "invalid",
			},
			reason: "invalid mode format",
		},
	}

	for i, test := range invalidConfigs {
		task := &types.TaskConfig{
			ID:     "test",
			Name:   "Test Copy",
			Type:   "copy",
			Config: test.config,
		}

		err := executor.Validate(task)
		if err == nil {
			t.Errorf("Config %d (%s): Expected validation error", i, test.reason)
		}
	}
}

func TestExecutor_Execute_CopyFile(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	// Create temporary source file
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	content := []byte("Hello, World!")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test-copy",
		Name: "Test Copy File",
		Type: "copy",
		Config: map[string]interface{}{
			"src":  srcFile,
			"dest": dstFile,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Verify destination file exists
	if _, err := os.Stat(dstFile); err != nil {
		t.Errorf("Destination file not created: %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Errorf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("Content mismatch: expected %s, got %s", content, dstContent)
	}

	// Check output
	if result.Output["files_copied"] != 1 {
		t.Errorf("Expected files_copied=1, got %v", result.Output["files_copied"])
	}
}

func TestExecutor_Execute_CopyDirectory(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	// Create temporary source directory with files
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "source")
	dstDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create some files in source
	files := []string{"file1.txt", "file2.txt"}
	for _, file := range files {
		path := filepath.Join(srcDir, file)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	// Create subdirectory
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "file3.txt")
	if err := os.WriteFile(subFile, []byte("subdir content"), 0644); err != nil {
		t.Fatalf("Failed to create subdirectory file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test-copy-dir",
		Name: "Test Copy Directory",
		Type: "copy",
		Config: map[string]interface{}{
			"src":       srcDir,
			"dest":      dstDir,
			"recursive": true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Verify destination directory exists
	if _, err := os.Stat(dstDir); err != nil {
		t.Errorf("Destination directory not created: %v", err)
	}

	// Verify all files were copied
	for _, file := range files {
		path := filepath.Join(dstDir, file)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("File %s not copied: %v", file, err)
		}
	}

	// Verify subdirectory and its file
	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file3.txt")); err != nil {
		t.Errorf("Subdirectory file not copied: %v", err)
	}

	// Check that 3 files were copied
	if result.Output["files_copied"] != 3 {
		t.Errorf("Expected files_copied=3, got %v", result.Output["files_copied"])
	}
}

func TestExecutor_Execute_DirectoryWithoutRecursive(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "source")
	dstDir := filepath.Join(tmpDir, "dest")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test-no-recursive",
		Name: "Test Copy Directory Without Recursive",
		Type: "copy",
		Config: map[string]interface{}{
			"src":       srcDir,
			"dest":      dstDir,
			"recursive": false,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for directory without recursive, got %s", result.Status)
	}
}

func TestExecutor_Execute_ForceOverwrite(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	// Create source and destination files
	if err := os.WriteFile(srcFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	if err := os.WriteFile(dstFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create destination file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test-force",
		Name: "Test Force Overwrite",
		Type: "copy",
		Config: map[string]interface{}{
			"src":   srcFile,
			"dest":  dstFile,
			"force": true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Verify content was overwritten
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != "new content" {
		t.Errorf("Expected 'new content', got '%s'", content)
	}
}

func TestExecutor_Execute_WithBackup(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")
	backupFile := dstFile + ".bak"

	// Create source and destination files
	if err := os.WriteFile(srcFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	if err := os.WriteFile(dstFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create destination file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test-backup",
		Name: "Test Backup",
		Type: "copy",
		Config: map[string]interface{}{
			"src":    srcFile,
			"dest":   dstFile,
			"force":  true,
			"backup": true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Verify backup was created
	if _, err := os.Stat(backupFile); err != nil {
		t.Errorf("Backup file not created: %v", err)
	}

	// Verify backup has old content
	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}

	if string(backupContent) != "old content" {
		t.Errorf("Expected backup to contain 'old content', got '%s'", backupContent)
	}

	// Verify destination has new content
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != "new content" {
		t.Errorf("Expected destination to contain 'new content', got '%s'", dstContent)
	}
}

func TestExecutor_Execute_SkipExisting(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	// Create source and destination files
	if err := os.WriteFile(srcFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	if err := os.WriteFile(dstFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create destination file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test-skip",
		Name: "Test Skip Existing",
		Type: "copy",
		Config: map[string]interface{}{
			"src":   srcFile,
			"dest":  dstFile,
			"force": false, // Don't overwrite
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Verify content was NOT overwritten
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != "old content" {
		t.Errorf("Expected 'old content' (not overwritten), got '%s'", content)
	}

	// Check skipped count
	if result.Output["skipped"] != 1 {
		t.Errorf("Expected skipped=1, got %v", result.Output["skipped"])
	}
}

func TestExecutor_Execute_NonExistentSource(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	task := &types.TaskConfig{
		ID:   "test-nonexistent",
		Name: "Test Non-existent Source",
		Type: "copy",
		Config: map[string]interface{}{
			"src":  "/nonexistent/file.txt",
			"dest": "/tmp/dest.txt",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for non-existent source, got %s", result.Status)
	}
}

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		input    string
		expected os.FileMode
		hasError bool
	}{
		{"0644", 0644, false},
		{"644", 0644, false},
		{"0755", 0755, false},
		{"755", 0755, false},
		{"invalid", 0, true},
		{"999", 0, true}, // Invalid octal
	}

	for _, test := range tests {
		result, err := parseFileMode(test.input)

		if test.hasError {
			if err == nil {
				t.Errorf("Input '%s': Expected error, got nil", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Input '%s': Expected no error, got %v", test.input, err)
			}
			if result != test.expected {
				t.Errorf("Input '%s': Expected mode %o, got %o", test.input, test.expected, result)
			}
		}
	}
}
