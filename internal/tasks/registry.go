// ABOUTME: Task registry for registering and managing all available task types
// ABOUTME: Provides centralized access to command, file, compress, and other task executors

package tasks

import (
	"github.com/sarlalian/ritual/internal/tasks/checksum"
	"github.com/sarlalian/ritual/internal/tasks/command"
	"github.com/sarlalian/ritual/internal/tasks/compress"
	"github.com/sarlalian/ritual/internal/tasks/debug"
	"github.com/sarlalian/ritual/internal/tasks/email"
	"github.com/sarlalian/ritual/internal/tasks/file"
	"github.com/sarlalian/ritual/internal/tasks/slack"
	"github.com/sarlalian/ritual/internal/tasks/ssh"
	"github.com/sarlalian/ritual/pkg/types"
)

// Registry manages all available task executors
type Registry struct {
	executors map[string]types.TaskExecutor
}

// New creates a new task registry with all built-in task types
func New() *Registry {
	registry := &Registry{
		executors: make(map[string]types.TaskExecutor),
	}

	// Register built-in task types
	registry.RegisterBuiltinTasks()

	return registry
}

// RegisterBuiltinTasks registers all built-in task types
func (r *Registry) RegisterBuiltinTasks() {
	// Command task for running shell commands and scripts
	r.Register("command", command.New())
	r.Register("shell", command.New())    // Alias
	r.Register("script", command.New())   // Alias

	// File task for file system operations
	r.Register("file", file.New())
	r.Register("copy", file.New())        // Alias
	r.Register("template", file.New())    // Alias

	// Compress task for archive operations
	r.Register("compress", compress.New())
	r.Register("archive", compress.New()) // Alias
	r.Register("unarchive", compress.New()) // Alias

	// Checksum task for hash calculation and verification
	r.Register("checksum", checksum.New())
	r.Register("hash", checksum.New())      // Alias
	r.Register("verify", checksum.New())    // Alias

	// SSH task for remote command execution
	r.Register("ssh", ssh.New())
	r.Register("remote", ssh.New())         // Alias

	// Email task for sending emails via SMTP
	r.Register("email", email.New())
	r.Register("mail", email.New())         // Alias

	// Slack task for posting messages to Slack
	r.Register("slack", slack.New())
	r.Register("notify", slack.New())       // Alias

	// Debug task for logging templated strings
	r.Register("debug", debug.New())
	r.Register("log", debug.New())          // Alias
}

// Register adds a task executor for a specific task type
func (r *Registry) Register(taskType string, executor types.TaskExecutor) {
	r.executors[taskType] = executor
}

// Get retrieves a task executor by type
func (r *Registry) Get(taskType string) (types.TaskExecutor, bool) {
	executor, exists := r.executors[taskType]
	return executor, exists
}

// GetAvailableTypes returns all registered task types
func (r *Registry) GetAvailableTypes() []string {
	types := make([]string, 0, len(r.executors))
	for taskType := range r.executors {
		types = append(types, taskType)
	}
	return types
}

// RegisterToExecutor registers all task types to an executor instance
func (r *Registry) RegisterToExecutor(executor interface{}) error {
	// Type assertion to check if executor has RegisterTask method
	if exec, ok := executor.(interface {
		RegisterTask(string, types.TaskExecutor)
	}); ok {
		for taskType, taskExecutor := range r.executors {
			exec.RegisterTask(taskType, taskExecutor)
		}
		return nil
	}

	return nil // Silently ignore if executor doesn't support registration
}

// Validate validates a task configuration using the appropriate executor
func (r *Registry) Validate(task *types.TaskConfig) error {
	executor, exists := r.Get(task.Type)
	if !exists {
		return &types.TaskError{
			TaskID:   task.ID,
			TaskName: task.Name,
			TaskType: task.Type,
			Message:  "unknown task type",
		}
	}

	return executor.Validate(task)
}

// ValidateAll validates multiple task configurations
func (r *Registry) ValidateAll(tasks []types.TaskConfig) []error {
	var errors []error

	for _, task := range tasks {
		if err := r.Validate(&task); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}