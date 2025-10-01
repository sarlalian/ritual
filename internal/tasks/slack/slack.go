// ABOUTME: Slack task executor for posting messages to Slack channels
// ABOUTME: Supports webhook URLs and message formatting with attachments

package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles Slack task execution
type Executor struct{}

// SlackConfig represents the configuration for a Slack task
type SlackConfig struct {
	// Webhook URL for posting to Slack
	WebhookURL string `yaml:"webhook_url" json:"webhook_url"`

	// Message text to post
	Message string `yaml:"message" json:"message"`

	// Channel to post to (optional, overrides webhook default)
	Channel string `yaml:"channel,omitempty" json:"channel,omitempty"`

	// Username to display (optional)
	Username string `yaml:"username,omitempty" json:"username,omitempty"`

	// Icon emoji to display (optional, e.g., ":robot_face:")
	IconEmoji string `yaml:"icon_emoji,omitempty" json:"icon_emoji,omitempty"`

	// Icon URL to display (optional)
	IconURL string `yaml:"icon_url,omitempty" json:"icon_url,omitempty"`

	// Color for message attachment (optional, e.g., "good", "warning", "danger", or hex)
	Color string `yaml:"color,omitempty" json:"color,omitempty"`

	// Attachment title (optional)
	Title string `yaml:"title,omitempty" json:"title,omitempty"`

	// Attachment title link (optional)
	TitleLink string `yaml:"title_link,omitempty" json:"title_link,omitempty"`

	// Additional fields for the attachment (optional)
	Fields map[string]string `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// slackPayload represents the JSON payload sent to Slack
type slackPayload struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Color     string       `json:"color,omitempty"`
	Title     string       `json:"title,omitempty"`
	TitleLink string       `json:"title_link,omitempty"`
	Text      string       `json:"text,omitempty"`
	Fields    []slackField `json:"fields,omitempty"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// New creates a new Slack executor
func New() *Executor {
	return &Executor{}
}

// Execute runs a Slack task
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

	// Build payload
	payload := e.buildPayload(config)

	// Send to Slack
	if err := e.sendToSlack(ctx, config.WebhookURL, payload); err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to send Slack message: %v", err)
		result.Error = err.Error()
		return result
	}

	result.Status = types.TaskSuccess
	result.Message = "Slack message sent successfully"
	result.Output["message"] = config.Message
	if config.Channel != "" {
		result.Output["channel"] = config.Channel
	}

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	// Webhook URL is required
	if webhookURL, ok := task.Config["webhook_url"].(string); !ok || webhookURL == "" {
		return fmt.Errorf("slack task '%s': webhook_url is required", task.Name)
	}

	// Message is required
	if message, ok := task.Config["message"].(string); !ok || message == "" {
		return fmt.Errorf("slack task '%s': message is required", task.Name)
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig extracts and evaluates the configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*SlackConfig, error) {
	config := &SlackConfig{}

	// Evaluate config map with template engine
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate config: %w", err)
	}

	// Extract required fields
	if webhookURL, ok := evaluatedConfig["webhook_url"].(string); ok {
		config.WebhookURL = webhookURL
	} else {
		return nil, fmt.Errorf("webhook_url is required")
	}

	if message, ok := evaluatedConfig["message"].(string); ok {
		config.Message = message
	} else {
		return nil, fmt.Errorf("message is required")
	}

	// Extract optional fields
	if channel, ok := evaluatedConfig["channel"].(string); ok {
		config.Channel = channel
	}

	if username, ok := evaluatedConfig["username"].(string); ok {
		config.Username = username
	}

	if iconEmoji, ok := evaluatedConfig["icon_emoji"].(string); ok {
		config.IconEmoji = iconEmoji
	}

	if iconURL, ok := evaluatedConfig["icon_url"].(string); ok {
		config.IconURL = iconURL
	}

	if color, ok := evaluatedConfig["color"].(string); ok {
		config.Color = color
	}

	if title, ok := evaluatedConfig["title"].(string); ok {
		config.Title = title
	}

	if titleLink, ok := evaluatedConfig["title_link"].(string); ok {
		config.TitleLink = titleLink
	}

	// Extract fields
	if fields, ok := evaluatedConfig["fields"].(map[string]interface{}); ok {
		config.Fields = make(map[string]string)
		for k, v := range fields {
			config.Fields[k] = fmt.Sprintf("%v", v)
		}
	} else if fields, ok := evaluatedConfig["fields"].(map[string]string); ok {
		config.Fields = fields
	}

	return config, nil
}

// buildPayload constructs the Slack webhook payload
func (e *Executor) buildPayload(config *SlackConfig) *slackPayload {
	payload := &slackPayload{
		Text:      config.Message,
		Channel:   config.Channel,
		Username:  config.Username,
		IconEmoji: config.IconEmoji,
		IconURL:   config.IconURL,
	}

	// Add attachment if there's additional formatting
	if config.Color != "" || config.Title != "" || len(config.Fields) > 0 {
		attachment := slackAttachment{
			Color:     config.Color,
			Title:     config.Title,
			TitleLink: config.TitleLink,
		}

		// Add fields
		if len(config.Fields) > 0 {
			attachment.Fields = make([]slackField, 0, len(config.Fields))
			for key, value := range config.Fields {
				attachment.Fields = append(attachment.Fields, slackField{
					Title: key,
					Value: value,
					Short: true,
				})
			}
		}

		payload.Attachments = []slackAttachment{attachment}
	}

	return payload
}

// sendToSlack sends the payload to the Slack webhook URL
func (e *Executor) sendToSlack(ctx context.Context, webhookURL string, payload *slackPayload) error {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}
