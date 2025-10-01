// ABOUTME: Tests for SSH task executor
// ABOUTME: Validates SSH configuration parsing and validation logic

package ssh

import (
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

func TestSSH_Validate(t *testing.T) {
	executor := New()

	tests := []struct {
		name      string
		task      *types.TaskConfig
		shouldErr bool
	}{
		{
			name: "Valid with password",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":     "example.com",
					"user":     "testuser",
					"password": "testpass",
					"command":  "ls -la",
				},
			},
			shouldErr: false,
		},
		{
			name: "Valid with key file",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":     "example.com",
					"user":     "testuser",
					"key_file": "~/.ssh/id_rsa",
					"command":  "ls -la",
				},
			},
			shouldErr: false,
		},
		{
			name: "Missing host",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"user":     "testuser",
					"password": "testpass",
					"command":  "ls -la",
				},
			},
			shouldErr: true,
		},
		{
			name: "Missing user",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":     "example.com",
					"password": "testpass",
					"command":  "ls -la",
				},
			},
			shouldErr: true,
		},
		{
			name: "Missing command",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":     "example.com",
					"user":     "testuser",
					"password": "testpass",
				},
			},
			shouldErr: true,
		},
		{
			name: "No authentication method",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":    "example.com",
					"user":    "testuser",
					"command": "ls -la",
				},
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.Validate(tt.task)
			if tt.shouldErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestSSH_SupportsDryRun(t *testing.T) {
	executor := New()
	if !executor.SupportsDryRun() {
		t.Error("Expected SSH executor to support dry run")
	}
}
