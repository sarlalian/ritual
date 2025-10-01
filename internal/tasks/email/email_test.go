// ABOUTME: Tests for email task executor
// ABOUTME: Validates email configuration parsing and validation logic

package email

import (
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

func TestEmail_Validate(t *testing.T) {
	executor := New()

	tests := []struct {
		name      string
		task      *types.TaskConfig
		shouldErr bool
	}{
		{
			name: "Valid with single recipient",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":    "smtp.example.com",
					"from":    "sender@example.com",
					"to":      "recipient@example.com",
					"subject": "Test Email",
					"body":    "This is a test",
				},
			},
			shouldErr: false,
		},
		{
			name: "Valid with multiple recipients",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":    "smtp.example.com",
					"from":    "sender@example.com",
					"to":      []string{"recipient1@example.com", "recipient2@example.com"},
					"subject": "Test Email",
					"body":    "This is a test",
				},
			},
			shouldErr: false,
		},
		{
			name: "Missing host",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"from":    "sender@example.com",
					"to":      "recipient@example.com",
					"subject": "Test Email",
					"body":    "This is a test",
				},
			},
			shouldErr: true,
		},
		{
			name: "Missing from",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":    "smtp.example.com",
					"to":      "recipient@example.com",
					"subject": "Test Email",
					"body":    "This is a test",
				},
			},
			shouldErr: true,
		},
		{
			name: "Missing to",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":    "smtp.example.com",
					"from":    "sender@example.com",
					"subject": "Test Email",
					"body":    "This is a test",
				},
			},
			shouldErr: true,
		},
		{
			name: "Missing subject",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host": "smtp.example.com",
					"from": "sender@example.com",
					"to":   "recipient@example.com",
					"body": "This is a test",
				},
			},
			shouldErr: true,
		},
		{
			name: "Missing body",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"host":    "smtp.example.com",
					"from":    "sender@example.com",
					"to":      "recipient@example.com",
					"subject": "Test Email",
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

func TestEmail_SupportsDryRun(t *testing.T) {
	executor := New()
	if !executor.SupportsDryRun() {
		t.Error("Expected email executor to support dry run")
	}
}

func TestEmail_BuildMessage(t *testing.T) {
	executor := New()

	config := &EmailConfig{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		Body:    "Test Body",
		IsHTML:  false,
	}

	msg := executor.buildMessage(config)

	if msg == "" {
		t.Error("Expected non-empty message")
	}

	// Check that message contains expected headers
	if !contains(msg, "From: sender@example.com") {
		t.Error("Message missing From header")
	}

	if !contains(msg, "To: recipient@example.com") {
		t.Error("Message missing To header")
	}

	if !contains(msg, "Subject: Test Subject") {
		t.Error("Message missing Subject header")
	}

	if !contains(msg, "Test Body") {
		t.Error("Message missing body")
	}
}

func TestEmail_BuildMessage_HTML(t *testing.T) {
	executor := New()

	config := &EmailConfig{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		Body:    "<html><body>Test Body</body></html>",
		IsHTML:  true,
	}

	msg := executor.buildMessage(config)

	if !contains(msg, "Content-Type: text/html") {
		t.Error("HTML message missing Content-Type header")
	}

	if !contains(msg, "MIME-Version") {
		t.Error("HTML message missing MIME-Version header")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}
