// ABOUTME: Error types and utilities for the Ritual workflow engine
// ABOUTME: Defines custom error types for different failure scenarios

package types

import (
	"errors"
	"fmt"
	"strings"
)

// WorkflowError represents an error that occurred during workflow execution
type WorkflowError struct {
	Workflow string
	Message  string
	Cause    error
}

func (e *WorkflowError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("workflow '%s': %s: %v", e.Workflow, e.Message, e.Cause)
	}
	return fmt.Sprintf("workflow '%s': %s", e.Workflow, e.Message)
}

func (e *WorkflowError) Unwrap() error {
	return e.Cause
}

// NewWorkflowError creates a new workflow error
func NewWorkflowError(workflow, message string, cause error) *WorkflowError {
	return &WorkflowError{
		Workflow: workflow,
		Message:  message,
		Cause:    cause,
	}
}

// TaskError represents an error that occurred during task execution
type TaskError struct {
	TaskID   string
	TaskName string
	TaskType string
	Message  string
	Cause    error
}

func (e *TaskError) Error() string {
	task := e.TaskName
	if task == "" {
		task = e.TaskID
	}
	if e.TaskType != "" {
		task = fmt.Sprintf("%s (%s)", task, e.TaskType)
	}

	if e.Cause != nil {
		return fmt.Sprintf("task '%s': %s: %v", task, e.Message, e.Cause)
	}
	return fmt.Sprintf("task '%s': %s", task, e.Message)
}

func (e *TaskError) Unwrap() error {
	return e.Cause
}

// NewTaskError creates a new task error
func NewTaskError(taskID, taskName, taskType, message string, cause error) *TaskError {
	return &TaskError{
		TaskID:   taskID,
		TaskName: taskName,
		TaskType: taskType,
		Message:  message,
		Cause:    cause,
	}
}

// ValidationError represents an error in workflow or task validation
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// ParseError represents an error in parsing YAML workflow definitions
type ParseError struct {
	File    string
	Line    int
	Column  int
	Message string
	Cause   error
}

func (e *ParseError) Error() string {
	location := ""
	if e.File != "" {
		location = e.File
		if e.Line > 0 {
			location = fmt.Sprintf("%s:%d", location, e.Line)
			if e.Column > 0 {
				location = fmt.Sprintf("%s:%d", location, e.Column)
			}
		}
		location = fmt.Sprintf(" in %s", location)
	}

	if e.Cause != nil {
		return fmt.Sprintf("parse error%s: %s: %v", location, e.Message, e.Cause)
	}
	return fmt.Sprintf("parse error%s: %s", location, e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// NewParseError creates a new parse error
func NewParseError(file string, line, column int, message string, cause error) *ParseError {
	return &ParseError{
		File:    file,
		Line:    line,
		Column:  column,
		Message: message,
		Cause:   cause,
	}
}

// DependencyError represents an error in task dependencies
type DependencyError struct {
	TaskID       string
	Dependencies []string
	Message      string
}

func (e *DependencyError) Error() string {
	if len(e.Dependencies) > 0 {
		deps := strings.Join(e.Dependencies, ", ")
		return fmt.Sprintf("dependency error for task '%s' (depends on: %s): %s", e.TaskID, deps, e.Message)
	}
	return fmt.Sprintf("dependency error for task '%s': %s", e.TaskID, e.Message)
}

// NewDependencyError creates a new dependency error
func NewDependencyError(taskID string, deps []string, message string) *DependencyError {
	return &DependencyError{
		TaskID:       taskID,
		Dependencies: deps,
		Message:      message,
	}
}

// TemplateError represents an error in template evaluation
type TemplateError struct {
	Template string
	Context  string
	Message  string
	Cause    error
}

func (e *TemplateError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("template error in '%s': %s: %v", e.Template, e.Message, e.Cause)
	}
	return fmt.Sprintf("template error in '%s': %s", e.Template, e.Message)
}

func (e *TemplateError) Unwrap() error {
	return e.Cause
}

// NewTemplateError creates a new template error
func NewTemplateError(template, context, message string, cause error) *TemplateError {
	return &TemplateError{
		Template: template,
		Context:  context,
		Message:  message,
		Cause:    cause,
	}
}

// ImportError represents an error in importing workflows
type ImportError struct {
	ImportPath string
	Message    string
	Cause      error
}

func (e *ImportError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("import error for '%s': %s: %v", e.ImportPath, e.Message, e.Cause)
	}
	return fmt.Sprintf("import error for '%s': %s", e.ImportPath, e.Message)
}

func (e *ImportError) Unwrap() error {
	return e.Cause
}

// NewImportError creates a new import error
func NewImportError(importPath, message string, cause error) *ImportError {
	return &ImportError{
		ImportPath: importPath,
		Message:    message,
		Cause:      cause,
	}
}

// RetryableError indicates that an operation can be retried
type RetryableError struct {
	Err       error
	Retryable bool
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

func (e *RetryableError) IsRetryable() bool {
	return e.Retryable
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, retryable bool) *RetryableError {
	return &RetryableError{
		Err:       err,
		Retryable: retryable,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryableErr *RetryableError
	if ok := errors.As(err, &retryableErr); ok {
		return retryableErr.IsRetryable()
	}
	return false
}
