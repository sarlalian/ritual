// ABOUTME: Checksum task executor for calculating and verifying file hashes
// ABOUTME: Supports SHA256, SHA512, MD5, and Blake2b hash algorithms

package checksum

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/blake2b"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles checksum task execution
type Executor struct{}

// ChecksumConfig represents the configuration for a checksum task
type ChecksumConfig struct {
	// File path to calculate checksum for
	Path string `yaml:"path" json:"path"`

	// Algorithm to use (sha256, sha512, md5, blake2b)
	Algorithm string `yaml:"algorithm" json:"algorithm"`

	// Expected checksum for verification (optional)
	Expected string `yaml:"expected,omitempty" json:"expected,omitempty"`

	// Action to perform: "calculate" or "verify"
	Action string `yaml:"action,omitempty" json:"action,omitempty"`
}

// New creates a new checksum executor
func New() *Executor {
	return &Executor{}
}

// Execute runs a checksum task
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

	// Set default action
	if config.Action == "" {
		if config.Expected != "" {
			config.Action = "verify"
		} else {
			config.Action = "calculate"
		}
	}

	// Calculate checksum
	checksum, err := e.calculateChecksum(config.Path, config.Algorithm)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to calculate checksum: %v", err)
		return result
	}

	result.Output["checksum"] = checksum
	result.Output["algorithm"] = config.Algorithm
	result.Output["path"] = config.Path

	// Perform action
	switch config.Action {
	case "calculate":
		result.Status = types.TaskSuccess
		result.Message = fmt.Sprintf("Calculated %s checksum: %s", config.Algorithm, checksum)
		result.Stdout = checksum

	case "verify":
		if config.Expected == "" {
			result.Status = types.TaskFailed
			result.Message = "Expected checksum not provided for verification"
			return result
		}

		if checksum == config.Expected {
			result.Status = types.TaskSuccess
			result.Message = fmt.Sprintf("Checksum verification passed: %s", checksum)
			result.Output["verified"] = true
		} else {
			result.Status = types.TaskFailed
			result.Message = fmt.Sprintf("Checksum verification failed: expected %s, got %s", config.Expected, checksum)
			result.Output["verified"] = false
			result.Output["expected"] = config.Expected
		}

	default:
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Unknown action: %s (must be 'calculate' or 'verify')", config.Action)
	}

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	config := &ChecksumConfig{}

	// Extract path
	if path, ok := task.Config["path"].(string); ok {
		config.Path = path
	} else {
		return fmt.Errorf("checksum task '%s': path is required", task.Name)
	}

	// Extract algorithm
	if algo, ok := task.Config["algorithm"].(string); ok {
		config.Algorithm = algo
	} else {
		config.Algorithm = "sha256" // Default
	}

	// Validate algorithm
	validAlgos := map[string]bool{
		"sha256":  true,
		"sha512":  true,
		"md5":     true,
		"blake2b": true,
	}

	if !validAlgos[config.Algorithm] {
		return fmt.Errorf("checksum task '%s': invalid algorithm '%s' (must be sha256, sha512, md5, or blake2b)", task.Name, config.Algorithm)
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig extracts and evaluates the configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*ChecksumConfig, error) {
	config := &ChecksumConfig{}

	// Evaluate config map with template engine
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate config: %w", err)
	}

	// Extract path (required)
	if path, ok := evaluatedConfig["path"].(string); ok {
		config.Path = path
	} else {
		return nil, fmt.Errorf("path is required")
	}

	// Extract algorithm (optional, default to sha256)
	if algo, ok := evaluatedConfig["algorithm"].(string); ok {
		config.Algorithm = algo
	} else {
		config.Algorithm = "sha256"
	}

	// Extract expected checksum (optional)
	if expected, ok := evaluatedConfig["expected"].(string); ok {
		config.Expected = expected
	}

	// Extract action (optional)
	if action, ok := evaluatedConfig["action"].(string); ok {
		config.Action = action
	}

	return config, nil
}

// calculateChecksum calculates the checksum of a file using the specified algorithm
func (e *Executor) calculateChecksum(path, algorithm string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var hasher io.Writer
	var sum []byte

	switch algorithm {
	case "sha256":
		h := sha256.New()
		hasher = h
		if _, err := io.Copy(hasher, file); err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		sum = h.Sum(nil)

	case "sha512":
		h := sha512.New()
		hasher = h
		if _, err := io.Copy(hasher, file); err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		sum = h.Sum(nil)

	case "md5":
		h := md5.New()
		hasher = h
		if _, err := io.Copy(hasher, file); err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		sum = h.Sum(nil)

	case "blake2b":
		h, err := blake2b.New256(nil)
		if err != nil {
			return "", fmt.Errorf("failed to create blake2b hasher: %w", err)
		}
		hasher = h
		if _, err := io.Copy(hasher, file); err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		sum = h.Sum(nil)

	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	return hex.EncodeToString(sum), nil
}
