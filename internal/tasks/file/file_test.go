// ABOUTME: Tests for the file task executor with various file system operations
// ABOUTME: Validates file creation, modification, deletion, and permission handling

package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

// MockContextManager for testing
type MockContextManager struct{}

func (m *MockContextManager) EvaluateMap(data map[string]interface{}) (map[string]interface{}, error) {
	return data, nil
}

func (m *MockContextManager) EvaluateString(templateStr string) (string, error) {
	// Simple template evaluation - replace {{ .vars.name }} with "test_value"
	if strings.Contains(templateStr, "{{ .vars.name }}") {
		return strings.ReplaceAll(templateStr, "{{ .vars.name }}", "test_value"), nil
	}
	return templateStr, nil
}

func (m *MockContextManager) Initialize(workflow *types.Workflow, envVars []string) error { return nil }
func (m *MockContextManager) GetContext() *types.WorkflowContext                         { return nil }
func (m *MockContextManager) GetVariable(name string) (interface{}, error)              { return nil, nil }
func (m *MockContextManager) SetVariable(name string, value interface{}) error          { return nil }
func (m *MockContextManager) GetEnvironment(name, defaultValue string) string           { return defaultValue }
func (m *MockContextManager) SetEnvironment(name, value string) error                   { return nil }
func (m *MockContextManager) RegisterTaskResult(taskResult *types.TaskResult) error     { return nil }
func (m *MockContextManager) GetTaskResult(identifier string) (*types.TaskResult, error) {
	return nil, nil
}

func (m *MockContextManager) GetTemplateEngine() types.TemplateEngine {
	return &MockTemplateEngine{}
}

func (m *MockContextManager) Clone() types.ContextManager { return &MockContextManager{} }

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
		t.Error("Expected file executor to support dry run")
	}
}

func TestExecutor_ValidateConfig_Valid(t *testing.T) {
	executor := New()

	validConfigs := []map[string]interface{}{
		{
			"path": "/tmp/test.txt",
		},
		{
			"path":    "/tmp/test.txt",
			"state":   "present",
			"content": "test content",
		},
		{
			"path":  "/tmp/testdir",
			"state": "directory",
			"mode":  "755",
		},
		{
			"path":   "/tmp/test.txt",
			"source": "/etc/hosts",
		},
	}

	for i, config := range validConfigs {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			task := &types.TaskConfig{
				ID:     "test",
				Name:   "Test Task",
				Type:   "file",
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
			reason: "missing path",
		},
		{
			config: map[string]interface{}{
				"path":  "/tmp/test.txt",
				"state": "invalid",
			},
			reason: "invalid state",
		},
		{
			config: map[string]interface{}{
				"path": "/tmp/test.txt",
				"mode": "invalid",
			},
			reason: "invalid mode",
		},
		{
			config: map[string]interface{}{
				"path":    "/tmp/test.txt",
				"content": "test",
				"source":  "/etc/hosts",
			},
			reason: "both content and source",
		},
	}

	for i, test := range invalidConfigs {
		t.Run(test.reason, func(t *testing.T) {
			task := &types.TaskConfig{
				ID:     "test",
				Name:   "Test Task",
				Type:   "file",
				Config: test.config,
			}

			err := executor.Validate(task)
			if err == nil {
				t.Errorf("Expected config %d to fail validation (%s)", i, test.reason)
			}
		})
	}
}

func TestExecutor_Execute_CreateFile(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Create Test File",
		Type: "file",
		Config: map[string]interface{}{
			"path":    testFile,
			"content": "test content",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Expected file to be created")
	}

	// Check content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("Expected 'test content', got '%s'", string(content))
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}
}

func TestExecutor_Execute_UpdateFile(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create initial file
	err := os.WriteFile(testFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Update Test File",
		Type: "file",
		Config: map[string]interface{}{
			"path":    testFile,
			"content": "updated content",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != "updated content" {
		t.Errorf("Expected 'updated content', got '%s'", string(content))
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}
}

func TestExecutor_Execute_FileUpToDate(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with target content
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Check File Up To Date",
		Type: "file",
		Config: map[string]interface{}{
			"path":    testFile,
			"content": "test content",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != false {
		t.Error("Expected changed=false in output")
	}

	if !strings.Contains(result.Message, "up to date") {
		t.Errorf("Expected 'up to date' message, got: %s", result.Message)
	}
}

func TestExecutor_Execute_CreateDirectory(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Create Test Directory",
		Type: "file",
		Config: map[string]interface{}{
			"path":  testDir,
			"state": "directory",
			"mode":  "755",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if directory was created
	info, err := os.Stat(testDir)
	if os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	} else if !info.IsDir() {
		t.Error("Expected path to be a directory")
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}
}

func TestExecutor_Execute_RemoveFile(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file to be removed
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Remove Test File",
		Type: "file",
		Config: map[string]interface{}{
			"path":  testFile,
			"state": "absent",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if file was removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected file to be removed")
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}
}

func TestExecutor_Execute_TouchFile(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Touch Test File",
		Type: "file",
		Config: map[string]interface{}{
			"path":  testFile,
			"state": "touch",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if file was created
	info, err := os.Stat(testFile)
	if os.IsNotExist(err) {
		t.Error("Expected file to be created")
	} else if info.IsDir() {
		t.Error("Expected path to be a file")
	}

	// Check file is empty
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if len(content) != 0 {
		t.Error("Expected empty file")
	}
}

func TestExecutor_Execute_CopyFromSource(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	err := os.WriteFile(sourceFile, []byte("source content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Copy From Source",
		Type: "file",
		Config: map[string]interface{}{
			"path":   destFile,
			"source": sourceFile,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if file was created with correct content
	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(content) != "source content" {
		t.Errorf("Expected 'source content', got '%s'", string(content))
	}
}

func TestExecutor_Execute_CreateParentDirs(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "deep", "nested", "test.txt")

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Create File with Parent Dirs",
		Type: "file",
		Config: map[string]interface{}{
			"path":        testFile,
			"content":     "test content",
			"create_dirs": true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Expected file to be created")
	}

	// Check if parent directories were created
	parentDir := filepath.Dir(testFile)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Error("Expected parent directories to be created")
	}
}

func TestExecutor_Execute_TemplateContent(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Create File with Template",
		Type: "file",
		Config: map[string]interface{}{
			"path":     testFile,
			"content":  "Hello {{ .vars.name }}!",
			"template": true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check content was templated
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != "Hello test_value!" {
		t.Errorf("Expected 'Hello test_value!', got '%s'", string(content))
	}
}

func TestExecutor_Execute_BackupFile(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	backupFile := testFile + ".bak"

	// Create initial file
	err := os.WriteFile(testFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Update File with Backup",
		Type: "file",
		Config: map[string]interface{}{
			"path":    testFile,
			"content": "updated content",
			"backup":  true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check backup was created
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("Expected backup file to be created")
	}

	// Check backup content
	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}
	if string(backupContent) != "initial content" {
		t.Errorf("Expected backup to contain 'initial content', got '%s'", string(backupContent))
	}

	// Check main file was updated
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read main file: %v", err)
	}
	if string(content) != "updated content" {
		t.Errorf("Expected main file to contain 'updated content', got '%s'", string(content))
	}

	// Check backup file path in output
	if backupPath, exists := result.Output["backup_file"]; !exists || backupPath != backupFile {
		t.Errorf("Expected backup_file in output to be '%s', got %v", backupFile, backupPath)
	}
}