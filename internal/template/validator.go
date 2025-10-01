// ABOUTME: Template validation utilities for pre-execution checks
// ABOUTME: Validates that templates don't reference non-existent tasks, variables, or environment

package template

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sarlalian/ritual/pkg/types"
)

// ValidationError represents a template validation error
type ValidationError struct {
	TaskID      string
	TaskName    string
	Field       string
	Template    string
	Message     string
	Suggestion  string
}

func (e *ValidationError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("task '%s' (%s) field '%s': %s (suggestion: %s)",
			e.TaskName, e.TaskID, e.Field, e.Message, e.Suggestion)
	}
	return fmt.Sprintf("task '%s' (%s) field '%s': %s",
		e.TaskName, e.TaskID, e.Field, e.Message)
}

// ValidateTaskTemplates validates all templates in task configurations
// Note: This validation is performed before workflow execution, so it uses
// a mock context. The validation checks for structural errors (like referencing
// non-existent tasks) but cannot validate runtime values (like variables that
// depend on task execution results).
func ValidateTaskTemplates(tasks []types.TaskConfig, templateEngine types.TemplateEngine) []error {
	var errors []error

	// Build list of available task IDs
	availableTaskIDs := make(map[string]bool)
	for _, task := range tasks {
		availableTaskIDs[task.ID] = true
		if task.Name != task.ID {
			availableTaskIDs[task.Name] = true
		}
	}

	// Validate each task's configuration for task references only
	// We skip full template validation here because variables and environment
	// haven't been initialized yet during the validation phase
	for _, task := range tasks {
		taskErrors := validateTaskReferences(&task, availableTaskIDs)
		errors = append(errors, taskErrors...)
	}

	return errors
}

// validateTaskReferences validates that task references in templates point to valid tasks
func validateTaskReferences(task *types.TaskConfig, availableTaskIDs map[string]bool) []error {
	var errors []error

	// Check templates in config map
	for field, value := range task.Config {
		fieldErrors := checkValueForTaskReferences(task, field, value, availableTaskIDs)
		errors = append(errors, fieldErrors...)
	}

	return errors
}

// checkValueForTaskReferences recursively checks a value for task references
func checkValueForTaskReferences(task *types.TaskConfig, field string, value interface{}, availableTaskIDs map[string]bool) []error {
	var errors []error

	switch v := value.(type) {
	case string:
		// Check if this contains task references
		if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
			taskRefErrors := checkTaskReferences(task, field, v, availableTaskIDs)
			errors = append(errors, taskRefErrors...)
		}

	case map[string]interface{}:
		// Recursively check nested maps
		for nestedField, nestedValue := range v {
			nestedErrors := checkValueForTaskReferences(task, fmt.Sprintf("%s.%s", field, nestedField), nestedValue, availableTaskIDs)
			errors = append(errors, nestedErrors...)
		}

	case []interface{}:
		// Recursively check arrays
		for i, item := range v {
			itemErrors := checkValueForTaskReferences(task, fmt.Sprintf("%s[%d]", field, i), item, availableTaskIDs)
			errors = append(errors, itemErrors...)
		}
	}

	return errors
}


// checkTaskReferences checks if task references in templates point to valid tasks
func checkTaskReferences(task *types.TaskConfig, field string, templateStr string, availableTaskIDs map[string]bool) []error {
	var errors []error

	// Regex to match .tasks.TASKID patterns
	taskRefPattern := regexp.MustCompile(`\.tasks\.([a-zA-Z0-9_-]+)`)
	matches := taskRefPattern.FindAllStringSubmatch(templateStr, -1)

	for _, match := range matches {
		if len(match) > 1 {
			referencedTaskID := match[1]

			// Check if the referenced task exists
			if !availableTaskIDs[referencedTaskID] {
				// Try to find similar task IDs for suggestion
				suggestion := findSimilarTaskID(referencedTaskID, availableTaskIDs)

				validationErr := &ValidationError{
					TaskID:     task.ID,
					TaskName:   task.Name,
					Field:      field,
					Template:   templateStr,
					Message:    fmt.Sprintf("references non-existent task '%s'", referencedTaskID),
					Suggestion: suggestion,
				}
				errors = append(errors, validationErr)
			}
		}
	}

	return errors
}

// findSimilarTaskID finds a similar task ID for suggestions (simple string matching)
func findSimilarTaskID(target string, availableIDs map[string]bool) string {
	targetLower := strings.ToLower(target)

	// Look for tasks with similar names
	for id := range availableIDs {
		idLower := strings.ToLower(id)

		// Check if one contains the other
		if strings.Contains(idLower, targetLower) || strings.Contains(targetLower, idLower) {
			return fmt.Sprintf("did you mean '%s'?", id)
		}
	}

	return ""
}
