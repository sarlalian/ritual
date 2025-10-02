// ABOUTME: SSH task executor for running commands on remote hosts
// ABOUTME: Supports key-based and password authentication with command execution

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles SSH task execution
type Executor struct{}

// SSHConfig represents the configuration for an SSH task
type SSHConfig struct {
	// Host to connect to (hostname or IP)
	Host string `yaml:"host" json:"host"`

	// Port to connect to (default: 22)
	Port int `yaml:"port,omitempty" json:"port,omitempty"`

	// User to authenticate as
	User string `yaml:"user" json:"user"`

	// Password for authentication (optional if using key)
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Path to private key file (optional if using password)
	KeyFile string `yaml:"key_file,omitempty" json:"key_file,omitempty"`

	// Passphrase for encrypted private key (optional)
	Passphrase string `yaml:"passphrase,omitempty" json:"passphrase,omitempty"`

	// Command to execute on remote host
	Command string `yaml:"command" json:"command"`

	// Environment variables to set before command execution
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`

	// Timeout for the SSH connection and command execution
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// New creates a new SSH executor
func New() *Executor {
	return &Executor{}
}

// Execute runs an SSH task
func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:     task.ID,
		Name:   task.Name,
		Type:   task.Type,
		Status: types.TaskPending,
		Output: make(map[string]interface{}),
	}

	// Parse configuration
	config, err := e.parseConfig(task, contextManager)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to parse configuration: %v", err)
		return result
	}

	// Set default port
	if config.Port == 0 {
		config.Port = 22
	}

	// Parse timeout
	timeout := 30 * time.Second
	if config.Timeout != "" {
		if t, err := time.ParseDuration(config.Timeout); err == nil {
			timeout = t
		}
	}

	// Create SSH client
	client, err := e.createSSHClient(config, timeout)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to connect to SSH host: %v", err)
		return result
	}
	defer func() { _ = client.Close() }()

	// Create session
	session, err := client.NewSession()
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to create SSH session: %v", err)
		return result
	}
	defer func() { _ = session.Close() }()

	// Set environment variables
	for key, value := range config.Environment {
		// Some SSH servers don't allow setting env vars, so we ignore errors
		// and set them in the command prefix instead
		_ = session.Setenv(key, value)
	}

	// Prepare command with environment variables if server doesn't support Setenv
	command := config.Command
	if len(config.Environment) > 0 {
		envPrefix := []string{}
		for key, value := range config.Environment {
			envPrefix = append(envPrefix, fmt.Sprintf("export %s=%q", key, value))
		}
		command = strings.Join(envPrefix, "; ") + "; " + command
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Execute command with timeout
	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		result.Status = types.TaskFailed
		result.Message = "Task cancelled by context"
		return result

	case <-time.After(timeout):
		_ = session.Signal(ssh.SIGKILL)
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Command timed out after %v", timeout)
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
		return result

	case err := <-errChan:
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()

		if err != nil {
			// Extract exit code from SSH error if available
			if exitErr, ok := err.(*ssh.ExitError); ok {
				result.ReturnCode = exitErr.ExitStatus()
			} else {
				result.ReturnCode = 1
			}

			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Command failed: %v", err)
			result.Error = err.Error()
		} else {
			result.ReturnCode = 0
			result.Status = types.TaskSuccess
			result.Message = "Command completed successfully"
		}
	}

	result.Output["host"] = fmt.Sprintf("%s@%s:%d", config.User, config.Host, config.Port)
	result.Output["command"] = config.Command

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	config := &SSHConfig{}

	// Extract and validate required fields
	if host, ok := task.Config["host"].(string); ok {
		config.Host = host
	} else {
		return fmt.Errorf("ssh task '%s': host is required", task.Name)
	}

	if user, ok := task.Config["user"].(string); ok {
		config.User = user
	} else {
		return fmt.Errorf("ssh task '%s': user is required", task.Name)
	}

	if command, ok := task.Config["command"].(string); ok {
		config.Command = command
	} else {
		return fmt.Errorf("ssh task '%s': command is required", task.Name)
	}

	// At least one authentication method is required
	hasPassword := false
	if password, ok := task.Config["password"].(string); ok && password != "" {
		hasPassword = true
	}

	hasKeyFile := false
	if keyFile, ok := task.Config["key_file"].(string); ok && keyFile != "" {
		hasKeyFile = true
	}

	if !hasPassword && !hasKeyFile {
		return fmt.Errorf("ssh task '%s': either password or key_file must be provided", task.Name)
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig extracts and evaluates the configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*SSHConfig, error) {
	config := &SSHConfig{}

	// Evaluate config map with template engine
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate config: %w", err)
	}

	// Extract host (required)
	if host, ok := evaluatedConfig["host"].(string); ok {
		config.Host = host
	} else {
		return nil, fmt.Errorf("host is required")
	}

	// Extract user (required)
	if user, ok := evaluatedConfig["user"].(string); ok {
		config.User = user
	} else {
		return nil, fmt.Errorf("user is required")
	}

	// Extract command (required)
	if command, ok := evaluatedConfig["command"].(string); ok {
		config.Command = command
	} else {
		return nil, fmt.Errorf("command is required")
	}

	// Extract optional fields
	if port, ok := evaluatedConfig["port"].(int); ok {
		config.Port = port
	}

	if password, ok := evaluatedConfig["password"].(string); ok {
		config.Password = password
	}

	if keyFile, ok := evaluatedConfig["key_file"].(string); ok {
		config.KeyFile = keyFile
	}

	if passphrase, ok := evaluatedConfig["passphrase"].(string); ok {
		config.Passphrase = passphrase
	}

	if timeout, ok := evaluatedConfig["timeout"].(string); ok {
		config.Timeout = timeout
	}

	// Extract environment variables
	if env, ok := evaluatedConfig["environment"].(map[string]string); ok {
		config.Environment = env
	} else if env, ok := evaluatedConfig["environment"].(map[string]interface{}); ok {
		config.Environment = make(map[string]string)
		for k, v := range env {
			config.Environment[k] = fmt.Sprintf("%v", v)
		}
	}

	return config, nil
}

// createSSHClient creates and returns an SSH client connection
func (e *Executor) createSSHClient(config *SSHConfig, timeout time.Duration) (*ssh.Client, error) {
	authMethods := []ssh.AuthMethod{}

	// Try key-based authentication first
	if config.KeyFile != "" {
		keyAuth, err := e.getKeyAuth(config.KeyFile, config.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to load private key: %w", err)
		}
		authMethods = append(authMethods, keyAuth)
	}

	// Add password authentication
	if config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	// Create SSH client config
	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add proper host key verification
		Timeout:         timeout,
	}

	// Connect to SSH server
	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return client, nil
}

// getKeyAuth loads a private key and returns an SSH auth method
func (e *Executor) getKeyAuth(keyFile, passphrase string) (ssh.AuthMethod, error) {
	// Expand home directory if needed
	if strings.HasPrefix(keyFile, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyFile = filepath.Join(home, keyFile[2:])
	}

	// Read private key file
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Parse private key
	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}
