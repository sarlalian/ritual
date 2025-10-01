// ABOUTME: Debug task executor for logging templated strings during workflow execution
// ABOUTME: Useful for troubleshooting workflows and inspecting variable values

package debug

import (
	"context"
	"fmt"
	"time"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles debug task execution
type Executor struct{}

// DebugConfig represents the configuration for a debug task
type DebugConfig struct {
	Message string `yaml:"message,omitempty" json:"message,omitempty"`
	Level   string `yaml:"level,omitempty" json:"level,omitempty"`
}

// Log levels
const (
	LevelInfo  = "info"
	LevelDebug = "debug"
	LevelWarn  = "warn"
	LevelError = "error"
)

// New creates a new debug executor
func New() *Executor {
	return &Executor{}
}

// Execute runs a debug task
func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
		Status:    types.TaskRunning,
		Output:    make(map[string]interface{}),
	}

	// Parse debug configuration
	config, err := e.parseConfig(task, contextManager)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to parse configuration: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Log the message using standard logging
	e.logMessage(config)

	// Update result
	result.Status = types.TaskSuccess
	result.Message = fmt.Sprintf("Logged message at %s level", config.Level)
	result.Output["message"] = config.Message
	result.Output["level"] = config.Level
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	config, err := e.parseConfigRaw(task.Config)
	if err != nil {
		return fmt.Errorf("invalid debug configuration: %w", err)
	}

	// Message is required
	if config.Message == "" {
		return fmt.Errorf("debug task must specify 'message'")
	}

	// Validate level if specified
	if config.Level != "" {
		validLevels := []string{LevelInfo, LevelDebug, LevelWarn, LevelError}
		valid := false
		for _, level := range validLevels {
			if config.Level == level {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid level '%s', must be one of: info, debug, warn, error", config.Level)
		}
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig parses and evaluates the task configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*DebugConfig, error) {
	// First evaluate all templates in the config
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate task configuration: %w", err)
	}

	return e.parseConfigRaw(evaluatedConfig)
}

// parseConfigRaw parses raw configuration without template evaluation
func (e *Executor) parseConfigRaw(configMap map[string]interface{}) (*DebugConfig, error) {
	config := &DebugConfig{
		Level: LevelInfo, // Default to info level
	}

	for key, value := range configMap {
		switch key {
		case "message":
			if str, ok := value.(string); ok {
				config.Message = str
			} else {
				return nil, fmt.Errorf("message must be a string")
			}

		case "level":
			if str, ok := value.(string); ok {
				config.Level = str
			} else {
				return nil, fmt.Errorf("level must be a string")
			}
		}
	}

	return config, nil
}

// logMessage logs the message at the appropriate level
func (e *Executor) logMessage(config *DebugConfig) {
	prefix := ""
	switch config.Level {
	case LevelDebug:
		prefix = "[DEBUG]"
	case LevelWarn:
		prefix = "[WARN]"
	case LevelError:
		prefix = "[ERROR]"
	case LevelInfo:
		prefix = "[INFO]"
	default:
		prefix = "[INFO]"
	}

	fmt.Printf("%s %s\n", prefix, config.Message)
}
