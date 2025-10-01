// ABOUTME: Tests for the task registry and task type management
// ABOUTME: Validates task registration, validation, and executor integration

package tasks

import (
	"context"
	"strings"
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

func TestRegistry_New(t *testing.T) {
	registry := New()
	if registry == nil {
		t.Error("Expected registry to be created")
	}

	// Check that built-in tasks are registered
	availableTypes := registry.GetAvailableTypes()
	if len(availableTypes) == 0 {
		t.Error("Expected built-in tasks to be registered")
	}
}

func TestRegistry_GetAvailableTypes(t *testing.T) {
	registry := New()
	types := registry.GetAvailableTypes()

	expectedTypes := []string{
		"command", "shell", "script",
		"file", "copy", "template",
		"compress", "archive", "unarchive",
	}

	for _, expectedType := range expectedTypes {
		found := false
		for _, actualType := range types {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected type '%s' to be available", expectedType)
		}
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := New()

	// Test getting a known task type
	executor, exists := registry.Get("command")
	if !exists {
		t.Error("Expected 'command' task type to exist")
	}
	if executor == nil {
		t.Error("Expected executor to be returned")
	}

	// Test getting an unknown task type
	_, exists = registry.Get("unknown")
	if exists {
		t.Error("Expected 'unknown' task type to not exist")
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := New()

	// Create a mock executor
	mockExecutor := &MockTaskExecutor{}

	// Register a custom task type
	registry.Register("custom", mockExecutor)

	// Check that it's available
	executor, exists := registry.Get("custom")
	if !exists {
		t.Error("Expected custom task type to be registered")
	}
	if executor.(*MockTaskExecutor) != mockExecutor {
		t.Error("Expected registered executor to match")
	}

	// Check that it appears in available types
	types := registry.GetAvailableTypes()
	found := false
	for _, taskType := range types {
		if taskType == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected custom task type to appear in available types")
	}
}

func TestRegistry_Validate(t *testing.T) {
	registry := New()

	// Test valid task configuration
	validTask := &types.TaskConfig{
		ID:   "test",
		Name: "Test Command",
		Type: "command",
		Config: map[string]interface{}{
			"command": "echo hello",
		},
	}

	err := registry.Validate(validTask)
	if err != nil {
		t.Errorf("Expected valid task to pass validation, got: %v", err)
	}

	// Test invalid task configuration
	invalidTask := &types.TaskConfig{
		ID:     "test",
		Name:   "Test Invalid",
		Type:   "command",
		Config: map[string]interface{}{
			// Missing command
		},
	}

	err = registry.Validate(invalidTask)
	if err == nil {
		t.Error("Expected invalid task to fail validation")
	}

	// Test unknown task type
	unknownTask := &types.TaskConfig{
		ID:   "test",
		Name: "Test Unknown",
		Type: "unknown",
		Config: map[string]interface{}{
			"test": "value",
		},
	}

	err = registry.Validate(unknownTask)
	if err == nil {
		t.Error("Expected unknown task type to fail validation")
	}

	// Check that it's a TaskError
	if taskErr, ok := err.(*types.TaskError); !ok {
		t.Errorf("Expected TaskError, got %T", err)
	} else {
		if taskErr.TaskID != "test" {
			t.Errorf("Expected task ID 'test', got '%s'", taskErr.TaskID)
		}
		if taskErr.TaskType != "unknown" {
			t.Errorf("Expected task type 'unknown', got '%s'", taskErr.TaskType)
		}
		if !strings.Contains(taskErr.Message, "unknown task type") {
			t.Errorf("Expected unknown task type message, got '%s'", taskErr.Message)
		}
	}
}

func TestRegistry_ValidateAll(t *testing.T) {
	registry := New()

	tasks := []types.TaskConfig{
		{
			ID:   "valid",
			Name: "Valid Task",
			Type: "command",
			Config: map[string]interface{}{
				"command": "echo hello",
			},
		},
		{
			ID:     "invalid",
			Name:   "Invalid Task",
			Type:   "command",
			Config: map[string]interface{}{
				// Missing command
			},
		},
		{
			ID:   "unknown",
			Name: "Unknown Task",
			Type: "unknown_type",
			Config: map[string]interface{}{
				"test": "value",
			},
		},
	}

	errors := registry.ValidateAll(tasks)

	// Debug: print errors to understand what we're getting
	for i, err := range errors {
		t.Logf("Error %d: %v", i, err)
	}

	// Should have 2 errors (invalid and unknown)
	if len(errors) != 2 {
		t.Errorf("Expected 2 validation errors, got %d", len(errors))
	}

	// Check that we got the expected error types
	foundInvalidError := false
	foundUnknownError := false

	for _, err := range errors {
		errorStr := err.Error()
		if strings.Contains(errorStr, "command task must specify") {
			foundInvalidError = true
		}
		if taskErr, ok := err.(*types.TaskError); ok {
			if taskErr.TaskID == "unknown" {
				foundUnknownError = true
			}
		}
	}

	if !foundInvalidError {
		t.Error("Expected validation error for invalid command task")
	}
	if !foundUnknownError {
		t.Error("Expected validation error for 'unknown' task")
	}
}

func TestRegistry_RegisterToExecutor(t *testing.T) {
	registry := New()

	// Create a mock executor that supports task registration
	mockExecutor := &MockExecutor{}

	// Register tasks to the executor
	err := registry.RegisterToExecutor(mockExecutor)
	if err != nil {
		t.Errorf("Expected no error registering to executor, got: %v", err)
	}

	// Check that tasks were registered
	if len(mockExecutor.RegisteredTasks) == 0 {
		t.Error("Expected tasks to be registered to executor")
	}

	// Check specific task types
	if _, exists := mockExecutor.RegisteredTasks["command"]; !exists {
		t.Error("Expected 'command' task to be registered")
	}
	if _, exists := mockExecutor.RegisteredTasks["file"]; !exists {
		t.Error("Expected 'file' task to be registered")
	}
	if _, exists := mockExecutor.RegisteredTasks["compress"]; !exists {
		t.Error("Expected 'compress' task to be registered")
	}
}

// Mock task executor for testing
type MockTaskExecutor struct{}

func (m *MockTaskExecutor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	return &types.TaskResult{
		ID:     task.ID,
		Status: types.TaskSuccess,
	}
}

func (m *MockTaskExecutor) Validate(task *types.TaskConfig) error {
	return nil
}

func (m *MockTaskExecutor) SupportsDryRun() bool {
	return true
}

// Mock executor that supports task registration
type MockExecutor struct {
	RegisteredTasks map[string]types.TaskExecutor
}

func (m *MockExecutor) RegisterTask(taskType string, executor types.TaskExecutor) {
	if m.RegisteredTasks == nil {
		m.RegisteredTasks = make(map[string]types.TaskExecutor)
	}
	m.RegisteredTasks[taskType] = executor
}