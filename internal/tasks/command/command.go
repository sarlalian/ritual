// ABOUTME: Command task executor for running shell commands and scripts
// ABOUTME: Supports environment variables, working directories, and output capture

package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles command task execution
type Executor struct{}

// CommandConfig represents the configuration for a command task
type CommandConfig struct {
	Command     string            `yaml:"command" json:"command"`
	Args        []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Script      string            `yaml:"script,omitempty" json:"script,omitempty"`
	Shell       string            `yaml:"shell,omitempty" json:"shell,omitempty"`
	WorkingDir  string            `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	FailOnError bool              `yaml:"fail_on_error" json:"fail_on_error"`
	Capture     CaptureConfig     `yaml:"capture,omitempty" json:"capture,omitempty"`
}

// CaptureConfig controls what output to capture
type CaptureConfig struct {
	Stdout   bool `yaml:"stdout" json:"stdout"`
	Stderr   bool `yaml:"stderr" json:"stderr"`
	Combined bool `yaml:"combined" json:"combined"`
}

// New creates a new command executor
func New() *Executor {
	return &Executor{}
}

// Execute runs a command task
func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
		Status:    types.TaskRunning,
	}

	// Parse command configuration
	config, err := e.parseConfig(task, contextManager)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to parse configuration: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Execute the command
	execResult := e.executeCommand(ctx, config)

	// Update result
	result.Status = execResult.Status
	result.Message = execResult.Message
	result.Stdout = execResult.Stdout
	result.Stderr = execResult.Stderr
	result.ReturnCode = execResult.ReturnCode
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	config, err := e.parseConfigRaw(task.Config)
	if err != nil {
		return fmt.Errorf("invalid command configuration: %w", err)
	}

	// Must have either command or script
	if config.Command == "" && config.Script == "" {
		return fmt.Errorf("command task must specify either 'command' or 'script'")
	}

	// Cannot have both command and script
	if config.Command != "" && config.Script != "" {
		return fmt.Errorf("command task cannot specify both 'command' and 'script'")
	}

	// Validate timeout if specified
	if config.Timeout != "" {
		_, err := time.ParseDuration(config.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout format: %w", err)
		}
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig parses and evaluates the task configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*CommandConfig, error) {
	// First evaluate all templates in the config
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate task configuration: %w", err)
	}

	return e.parseConfigRaw(evaluatedConfig)
}

// parseConfigRaw parses raw configuration without template evaluation
func (e *Executor) parseConfigRaw(configMap map[string]interface{}) (*CommandConfig, error) {
	config := &CommandConfig{
		Shell:       "/bin/sh", // Default shell
		FailOnError: true,      // Default to failing on error
		Capture: CaptureConfig{
			Stdout: true, // Default to capturing stdout
			Stderr: true, // Default to capturing stderr
		},
	}

	for key, value := range configMap {
		switch key {
		case "command":
			if str, ok := value.(string); ok {
				config.Command = str
			} else {
				return nil, fmt.Errorf("command must be a string")
			}

		case "args":
			if args, ok := value.([]interface{}); ok {
				config.Args = make([]string, len(args))
				for i, arg := range args {
					if str, ok := arg.(string); ok {
						config.Args[i] = str
					} else {
						return nil, fmt.Errorf("all args must be strings")
					}
				}
			} else {
				return nil, fmt.Errorf("args must be an array of strings")
			}

		case "script":
			if str, ok := value.(string); ok {
				config.Script = str
			} else {
				return nil, fmt.Errorf("script must be a string")
			}

		case "shell":
			if str, ok := value.(string); ok {
				config.Shell = str
			} else {
				return nil, fmt.Errorf("shell must be a string")
			}

		case "working_dir":
			if str, ok := value.(string); ok {
				config.WorkingDir = str
			} else {
				return nil, fmt.Errorf("working_dir must be a string")
			}

		case "environment":
			if env, ok := value.(map[string]interface{}); ok {
				config.Environment = make(map[string]string)
				for k, v := range env {
					if str, ok := v.(string); ok {
						config.Environment[k] = str
					} else {
						return nil, fmt.Errorf("all environment values must be strings")
					}
				}
			} else {
				return nil, fmt.Errorf("environment must be a map of strings")
			}

		case "timeout":
			if str, ok := value.(string); ok {
				config.Timeout = str
			} else {
				return nil, fmt.Errorf("timeout must be a string")
			}

		case "fail_on_error":
			if b, ok := value.(bool); ok {
				config.FailOnError = b
			} else {
				return nil, fmt.Errorf("fail_on_error must be a boolean")
			}

		case "capture":
			if capture, ok := value.(map[string]interface{}); ok {
				if stdout, exists := capture["stdout"]; exists {
					if b, ok := stdout.(bool); ok {
						config.Capture.Stdout = b
					} else {
						return nil, fmt.Errorf("capture.stdout must be a boolean")
					}
				}
				if stderr, exists := capture["stderr"]; exists {
					if b, ok := stderr.(bool); ok {
						config.Capture.Stderr = b
					} else {
						return nil, fmt.Errorf("capture.stderr must be a boolean")
					}
				}
				if combined, exists := capture["combined"]; exists {
					if b, ok := combined.(bool); ok {
						config.Capture.Combined = b
					} else {
						return nil, fmt.Errorf("capture.combined must be a boolean")
					}
				}
			} else {
				return nil, fmt.Errorf("capture must be a map")
			}
		}
	}

	return config, nil
}

// executeCommand executes the actual command
func (e *Executor) executeCommand(ctx context.Context, config *CommandConfig) *types.TaskResult {
	result := &types.TaskResult{
		Status: types.TaskRunning,
	}

	// Apply timeout if specified
	var cancel context.CancelFunc
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout)
		if err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Invalid timeout: %v", err)
			return result
		}

		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Prepare command
	var cmd *exec.Cmd
	if config.Script != "" {
		// Execute script through shell
		cmd = exec.CommandContext(ctx, config.Shell, "-c", config.Script)
	} else {
		// Execute command directly
		if len(config.Args) > 0 {
			cmd = exec.CommandContext(ctx, config.Command, config.Args...)
		} else {
			// Parse command string for args
			parts := strings.Fields(config.Command)
			if len(parts) == 0 {
				result.Status = types.TaskFailed
				result.Message = "Empty command"
				return result
			}
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		}
	}

	// Set working directory
	if config.WorkingDir != "" {
		// Expand relative paths
		workDir, err := filepath.Abs(config.WorkingDir)
		if err != nil {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Invalid working directory: %v", err)
			return result
		}

		// Check if directory exists
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Working directory does not exist: %s", workDir)
			return result
		}

		cmd.Dir = workDir
	}

	// Set environment variables
	cmd.Env = os.Environ() // Start with current environment
	for key, value := range config.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set up output capture
	var stdoutBuf, stderrBuf strings.Builder
	if config.Capture.Combined {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stdoutBuf
	} else {
		if config.Capture.Stdout {
			cmd.Stdout = &stdoutBuf
		}
		if config.Capture.Stderr {
			cmd.Stderr = &stderrBuf
		}
	}


	// Execute command
	err := cmd.Run()

	// Get output
	if config.Capture.Combined {
		result.Stdout = stdoutBuf.String()
	} else {
		result.Stdout = stdoutBuf.String()
		result.Stderr = stderrBuf.String()
	}

	// Determine result status
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Command timed out after %s", config.Timeout)
			result.ReturnCode = -1
		} else if exitError, ok := err.(*exec.ExitError); ok {
			// Command executed but returned non-zero exit code
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				result.ReturnCode = status.ExitStatus()
			} else {
				result.ReturnCode = 1
			}

			if config.FailOnError {
				result.Status = types.TaskFailed
				result.Message = fmt.Sprintf("Command failed with exit code %d", result.ReturnCode)
			} else {
				result.Status = types.TaskSuccess
				result.Message = fmt.Sprintf("Command completed with exit code %d (ignored)", result.ReturnCode)
			}
		} else {
			// Failed to execute command
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Failed to execute command: %v", err)
			result.ReturnCode = -1
		}
	} else {
		// Command executed successfully
		result.Status = types.TaskSuccess
		result.Message = "Command executed successfully"
		result.ReturnCode = 0
	}

	return result
}
