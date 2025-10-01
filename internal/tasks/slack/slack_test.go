// ABOUTME: Tests for Slack task executor
// ABOUTME: Validates Slack configuration parsing and validation logic

package slack

import (
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

func TestSlack_Validate(t *testing.T) {
	executor := New()

	tests := []struct {
		name      string
		task      *types.TaskConfig
		shouldErr bool
	}{
		{
			name: "Valid basic message",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"webhook_url": "https://hooks.slack.com/services/TEST/WEBHOOK/URL",
					"message":     "Test message",
				},
			},
			shouldErr: false,
		},
		{
			name: "Valid with optional fields",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"webhook_url": "https://hooks.slack.com/services/TEST/WEBHOOK/URL",
					"message":     "Test message",
					"channel":     "#general",
					"username":    "Ritual Bot",
					"icon_emoji":  ":robot_face:",
				},
			},
			shouldErr: false,
		},
		{
			name: "Missing webhook_url",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"message": "Test message",
				},
			},
			shouldErr: true,
		},
		{
			name: "Missing message",
			task: &types.TaskConfig{
				Name: "Test",
				Config: map[string]interface{}{
					"webhook_url": "https://hooks.slack.com/services/TEST/WEBHOOK/URL",
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

func TestSlack_SupportsDryRun(t *testing.T) {
	executor := New()
	if !executor.SupportsDryRun() {
		t.Error("Expected Slack executor to support dry run")
	}
}

func TestSlack_BuildPayload(t *testing.T) {
	executor := New()

	config := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST/WEBHOOK/URL",
		Message:    "Test message",
		Channel:    "#general",
		Username:   "Test Bot",
		IconEmoji:  ":robot_face:",
	}

	payload := executor.buildPayload(config)

	if payload.Text != "Test message" {
		t.Errorf("Expected text 'Test message', got '%s'", payload.Text)
	}

	if payload.Channel != "#general" {
		t.Errorf("Expected channel '#general', got '%s'", payload.Channel)
	}

	if payload.Username != "Test Bot" {
		t.Errorf("Expected username 'Test Bot', got '%s'", payload.Username)
	}

	if payload.IconEmoji != ":robot_face:" {
		t.Errorf("Expected icon emoji ':robot_face:', got '%s'", payload.IconEmoji)
	}
}

func TestSlack_BuildPayload_WithAttachment(t *testing.T) {
	executor := New()

	config := &SlackConfig{
		WebhookURL: "https://hooks.slack.com/services/TEST/WEBHOOK/URL",
		Message:    "Test message",
		Color:      "good",
		Title:      "Test Title",
		TitleLink:  "https://example.com",
		Fields: map[string]string{
			"Field1": "Value1",
			"Field2": "Value2",
		},
	}

	payload := executor.buildPayload(config)

	if len(payload.Attachments) != 1 {
		t.Fatalf("Expected 1 attachment, got %d", len(payload.Attachments))
	}

	attachment := payload.Attachments[0]

	if attachment.Color != "good" {
		t.Errorf("Expected color 'good', got '%s'", attachment.Color)
	}

	if attachment.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", attachment.Title)
	}

	if attachment.TitleLink != "https://example.com" {
		t.Errorf("Expected title link 'https://example.com', got '%s'", attachment.TitleLink)
	}

	if len(attachment.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(attachment.Fields))
	}
}
