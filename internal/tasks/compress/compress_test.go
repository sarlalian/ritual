// ABOUTME: Tests for the compress task executor with archive operations
// ABOUTME: Validates archive creation, extraction, and format handling

package compress

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
		t.Error("Expected compress executor to support dry run")
	}
}

func TestExecutor_DetectFormat(t *testing.T) {
	executor := New()

	tests := []struct {
		path     string
		expected string
	}{
		{"archive.tar.gz", FormatTarGz},
		{"archive.tgz", FormatTarGz},
		{"archive.tar", FormatTar},
		{"archive.zip", FormatZip},
		{"file.gz", FormatGzip},
		{"unknown.ext", FormatTarGz}, // Default
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			result := executor.detectFormat(test.path)
			if result != test.expected {
				t.Errorf("Expected format %s for %s, got %s", test.expected, test.path, result)
			}
		})
	}
}

func TestExecutor_ValidateConfig_Valid(t *testing.T) {
	executor := New()

	validConfigs := []map[string]interface{}{
		{
			"path":    "/tmp/archive.tar.gz",
			"state":   "create",
			"sources": []interface{}{"/tmp/source1", "/tmp/source2"},
		},
		{
			"path":  "/tmp/archive.zip",
			"state": "extract",
		},
		{
			"path":    "/tmp/archive.tar",
			"format":  "tar",
			"sources": []interface{}{"/tmp/source"},
		},
	}

	for i, config := range validConfigs {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			task := &types.TaskConfig{
				ID:     "test",
				Name:   "Test Task",
				Type:   "compress",
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
				"path":  "/tmp/archive.tar.gz",
				"state": "invalid",
			},
			reason: "invalid state",
		},
		{
			config: map[string]interface{}{
				"path":   "/tmp/archive.tar.gz",
				"format": "invalid",
			},
			reason: "invalid format",
		},
		{
			config: map[string]interface{}{
				"path":  "/tmp/archive.tar.gz",
				"state": "create",
				// missing sources
			},
			reason: "missing sources for create",
		},
	}

	for i, test := range invalidConfigs {
		t.Run(test.reason, func(t *testing.T) {
			task := &types.TaskConfig{
				ID:     "test",
				Name:   "Test Task",
				Type:   "compress",
				Config: test.config,
			}

			err := executor.Validate(task)
			if err == nil {
				t.Errorf("Expected config %d to fail validation (%s)", i, test.reason)
			}
		})
	}
}

func TestExecutor_Execute_CreateTarGz(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()

	// Create test files
	testFile1 := filepath.Join(tmpDir, "file1.txt")
	testFile2 := filepath.Join(tmpDir, "file2.txt")
	err := os.WriteFile(testFile1, []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	err = os.WriteFile(testFile2, []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create archive
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	task := &types.TaskConfig{
		ID:   "test",
		Name: "Create Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":    archivePath,
			"state":   "create",
			"format":  "tar.gz",
			"sources": []interface{}{testFile1, testFile2},
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if archive was created
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("Expected archive to be created")
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}

	if files, exists := result.Output["archived_files"]; !exists || files != 2 {
		t.Errorf("Expected 2 archived files, got %v", files)
	}
}

func TestExecutor_Execute_ExtractTarGz(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()

	// First create an archive to extract
	sourceFile := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(sourceFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	createTask := &types.TaskConfig{
		ID:   "create",
		Name: "Create Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":    archivePath,
			"state":   "create",
			"sources": []interface{}{sourceFile},
		},
	}

	createResult := executor.Execute(context.Background(), createTask, contextManager)
	if createResult.Status != types.TaskSuccess {
		t.Fatalf("Failed to create test archive: %s", createResult.Message)
	}

	// Now extract to a different location
	extractDir := filepath.Join(tmpDir, "extract")
	extractTask := &types.TaskConfig{
		ID:   "extract",
		Name: "Extract Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":        archivePath,
			"state":       "extract",
			"destination": extractDir,
		},
	}

	result := executor.Execute(context.Background(), extractTask, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if file was extracted
	extractedFile := filepath.Join(extractDir, sourceFile)
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Error("Expected extracted file to exist")
	} else {
		// Check content
		content, err := os.ReadFile(extractedFile)
		if err != nil {
			t.Fatalf("Failed to read extracted file: %v", err)
		}
		if string(content) != "test content" {
			t.Errorf("Expected 'test content', got '%s'", string(content))
		}
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}
}

func TestExecutor_Execute_CreateZip(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("zip content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create zip archive
	archivePath := filepath.Join(tmpDir, "test.zip")
	task := &types.TaskConfig{
		ID:   "test",
		Name: "Create Zip Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":    archivePath,
			"state":   "create",
			"format":  "zip",
			"sources": []interface{}{testFile},
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if archive was created
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("Expected zip archive to be created")
	}
}

func TestExecutor_Execute_ExtractZip(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()

	// Create test file and zip archive
	sourceFile := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(sourceFile, []byte("zip test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	archivePath := filepath.Join(tmpDir, "test.zip")
	createTask := &types.TaskConfig{
		ID:   "create",
		Name: "Create Zip Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":    archivePath,
			"state":   "create",
			"format":  "zip",
			"sources": []interface{}{sourceFile},
		},
	}

	createResult := executor.Execute(context.Background(), createTask, contextManager)
	if createResult.Status != types.TaskSuccess {
		t.Fatalf("Failed to create test zip: %s", createResult.Message)
	}

	// Extract zip
	extractDir := filepath.Join(tmpDir, "extract")
	extractTask := &types.TaskConfig{
		ID:   "extract",
		Name: "Extract Zip Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":        archivePath,
			"state":       "extract",
			"destination": extractDir,
		},
	}

	result := executor.Execute(context.Background(), extractTask, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if file was extracted
	extractedFile := filepath.Join(extractDir, sourceFile)
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Error("Expected extracted file to exist")
	} else {
		// Check content
		content, err := os.ReadFile(extractedFile)
		if err != nil {
			t.Fatalf("Failed to read extracted file: %v", err)
		}
		if string(content) != "zip test content" {
			t.Errorf("Expected 'zip test content', got '%s'", string(content))
		}
	}
}

func TestExecutor_Execute_RemoveArchive(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()

	// Create test archive
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	err := os.WriteFile(archivePath, []byte("fake archive"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test archive: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Remove Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":  archivePath,
			"state": "absent",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check if archive was removed
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Error("Expected archive to be removed")
	}

	// Check output
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}
}

func TestExecutor_Execute_ArchiveAlreadyExists(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()

	// Create test file and archive
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	archivePath := filepath.Join(tmpDir, "existing.tar.gz")
	err = os.WriteFile(archivePath, []byte("existing archive"), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing archive: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Create Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":    archivePath,
			"state":   "present",
			"sources": []interface{}{testFile},
			// overwrite not set, should not overwrite
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check output indicates no change
	if changed, exists := result.Output["changed"]; !exists || changed != false {
		t.Error("Expected changed=false in output")
	}

	if !strings.Contains(result.Message, "already exists") {
		t.Errorf("Expected message about existing archive, got: %s", result.Message)
	}
}

func TestExecutor_Execute_OverwriteArchive(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	tmpDir := t.TempDir()

	// Create test file and existing archive
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	archivePath := filepath.Join(tmpDir, "existing.tar.gz")
	err = os.WriteFile(archivePath, []byte("old archive"), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing archive: %v", err)
	}

	task := &types.TaskConfig{
		ID:   "test",
		Name: "Overwrite Archive",
		Type: "compress",
		Config: map[string]interface{}{
			"path":      archivePath,
			"state":     "create",
			"sources":   []interface{}{testFile},
			"overwrite": true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success, got %s: %s", result.Status, result.Message)
	}

	// Check output indicates change
	if changed, exists := result.Output["changed"]; !exists || changed != true {
		t.Error("Expected changed=true in output")
	}
}

func TestExecutor_ShouldExclude(t *testing.T) {
	executor := New()

	config := &CompressConfig{
		Exclude: []string{".git", "*.log"},
		Include: []string{"*.txt"},
	}

	// Test exclude patterns
	if !executor.shouldExclude(".git/config", config) {
		t.Error("Expected .git/config to be excluded")
	}

	if !executor.shouldExclude("app.log", config) {
		t.Error("Expected app.log to be excluded")
	}

	// Test include patterns (only when include is specified)
	if !executor.shouldExclude("README.md", config) {
		t.Error("Expected README.md to be excluded (not in include list)")
	}

	if executor.shouldExclude("README.txt", config) {
		t.Error("Expected README.txt to be included")
	}

	// Test with no include patterns
	configNoInclude := &CompressConfig{
		Exclude: []string{".git"},
	}

	if executor.shouldExclude("README.md", configNoInclude) {
		t.Error("Expected README.md to be included (no include patterns)")
	}
}