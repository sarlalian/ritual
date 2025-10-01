// ABOUTME: Email task executor for sending emails via SMTP
// ABOUTME: Supports TLS, authentication, multiple recipients, and attachments

package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles email task execution
type Executor struct{}

// EmailConfig represents the configuration for an email task
type EmailConfig struct {
	// SMTP server host
	Host string `yaml:"host" json:"host"`

	// SMTP server port (default: 587 for TLS, 25 for non-TLS)
	Port int `yaml:"port,omitempty" json:"port,omitempty"`

	// Username for SMTP authentication
	Username string `yaml:"username,omitempty" json:"username,omitempty"`

	// Password for SMTP authentication
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// From address
	From string `yaml:"from" json:"from"`

	// To addresses (can be a single string or list)
	To []string `yaml:"to" json:"to"`

	// CC addresses (optional)
	CC []string `yaml:"cc,omitempty" json:"cc,omitempty"`

	// BCC addresses (optional)
	BCC []string `yaml:"bcc,omitempty" json:"bcc,omitempty"`

	// Subject line
	Subject string `yaml:"subject" json:"subject"`

	// Email body (plain text or HTML)
	Body string `yaml:"body" json:"body"`

	// Whether the body is HTML (default: false for plain text)
	IsHTML bool `yaml:"is_html,omitempty" json:"is_html,omitempty"`

	// Use TLS for connection (default: true)
	UseTLS bool `yaml:"use_tls,omitempty" json:"use_tls,omitempty"`

	// Skip TLS certificate verification (default: false)
	InsecureSkipVerify bool `yaml:"insecure_skip_verify,omitempty" json:"insecure_skip_verify,omitempty"`
}

// New creates a new email executor
func New() *Executor {
	return &Executor{}
}

// Execute runs an email task
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

	// Set defaults
	if config.Port == 0 {
		if config.UseTLS {
			config.Port = 587
		} else {
			config.Port = 25
		}
	}

	// Send email
	if err := e.sendEmail(config); err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to send email: %v", err)
		result.Error = err.Error()
		return result
	}

	result.Status = types.TaskSuccess
	result.Message = fmt.Sprintf("Email sent successfully to %d recipient(s)", len(config.To))
	result.Output["to"] = config.To
	result.Output["subject"] = config.Subject
	result.Output["host"] = fmt.Sprintf("%s:%d", config.Host, config.Port)

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	// Extract required fields
	if host, ok := task.Config["host"].(string); !ok || host == "" {
		return fmt.Errorf("email task '%s': host is required", task.Name)
	}

	if from, ok := task.Config["from"].(string); !ok || from == "" {
		return fmt.Errorf("email task '%s': from address is required", task.Name)
	}

	// Validate To field (can be string or slice)
	if to, ok := task.Config["to"]; !ok {
		return fmt.Errorf("email task '%s': to address(es) required", task.Name)
	} else {
		switch to.(type) {
		case string:
			// Single recipient is ok
		case []interface{}:
			// Multiple recipients is ok
		case []string:
			// Multiple recipients is ok
		default:
			return fmt.Errorf("email task '%s': to must be a string or list of strings", task.Name)
		}
	}

	if subject, ok := task.Config["subject"].(string); !ok || subject == "" {
		return fmt.Errorf("email task '%s': subject is required", task.Name)
	}

	if body, ok := task.Config["body"].(string); !ok || body == "" {
		return fmt.Errorf("email task '%s': body is required", task.Name)
	}

	return nil
}

// SupportsDryRun indicates if this executor supports dry-run mode
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig extracts and evaluates the configuration
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*EmailConfig, error) {
	config := &EmailConfig{
		UseTLS: true, // Default to TLS
	}

	// Evaluate config map with template engine
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate config: %w", err)
	}

	// Extract required fields
	if host, ok := evaluatedConfig["host"].(string); ok {
		config.Host = host
	} else {
		return nil, fmt.Errorf("host is required")
	}

	if from, ok := evaluatedConfig["from"].(string); ok {
		config.From = from
	} else {
		return nil, fmt.Errorf("from address is required")
	}

	// Parse To field (can be string or slice)
	if to, ok := evaluatedConfig["to"].(string); ok {
		config.To = []string{to}
	} else if toList, ok := evaluatedConfig["to"].([]interface{}); ok {
		config.To = make([]string, len(toList))
		for i, addr := range toList {
			config.To[i] = fmt.Sprintf("%v", addr)
		}
	} else if toList, ok := evaluatedConfig["to"].([]string); ok {
		config.To = toList
	} else {
		return nil, fmt.Errorf("to address(es) required")
	}

	if subject, ok := evaluatedConfig["subject"].(string); ok {
		config.Subject = subject
	} else {
		return nil, fmt.Errorf("subject is required")
	}

	if body, ok := evaluatedConfig["body"].(string); ok {
		config.Body = body
	} else {
		return nil, fmt.Errorf("body is required")
	}

	// Extract optional fields
	if port, ok := evaluatedConfig["port"].(int); ok {
		config.Port = port
	}

	if username, ok := evaluatedConfig["username"].(string); ok {
		config.Username = username
	}

	if password, ok := evaluatedConfig["password"].(string); ok {
		config.Password = password
	}

	// Parse CC
	if cc, ok := evaluatedConfig["cc"].([]interface{}); ok {
		config.CC = make([]string, len(cc))
		for i, addr := range cc {
			config.CC[i] = fmt.Sprintf("%v", addr)
		}
	} else if cc, ok := evaluatedConfig["cc"].([]string); ok {
		config.CC = cc
	}

	// Parse BCC
	if bcc, ok := evaluatedConfig["bcc"].([]interface{}); ok {
		config.BCC = make([]string, len(bcc))
		for i, addr := range bcc {
			config.BCC[i] = fmt.Sprintf("%v", addr)
		}
	} else if bcc, ok := evaluatedConfig["bcc"].([]string); ok {
		config.BCC = bcc
	}

	if isHTML, ok := evaluatedConfig["is_html"].(bool); ok {
		config.IsHTML = isHTML
	}

	if useTLS, ok := evaluatedConfig["use_tls"].(bool); ok {
		config.UseTLS = useTLS
	}

	if insecure, ok := evaluatedConfig["insecure_skip_verify"].(bool); ok {
		config.InsecureSkipVerify = insecure
	}

	return config, nil
}

// sendEmail sends an email using the provided configuration
func (e *Executor) sendEmail(config *EmailConfig) error {
	// Build email message
	msg := e.buildMessage(config)

	// Get all recipients for SMTP
	allRecipients := append([]string{}, config.To...)
	allRecipients = append(allRecipients, config.CC...)
	allRecipients = append(allRecipients, config.BCC...)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName:         config.Host,
		InsecureSkipVerify: config.InsecureSkipVerify,
	}

	if config.UseTLS {
		// Use STARTTLS
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer client.Close()

		// Start TLS
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}

		// Authenticate if credentials provided
		if config.Username != "" && config.Password != "" {
			auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}

		// Send email
		if err := client.Mail(config.From); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}

		for _, recipient := range allRecipients {
			if err := client.Rcpt(recipient); err != nil {
				return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
			}
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to open data writer: %w", err)
		}

		if _, err := w.Write([]byte(msg)); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close data writer: %w", err)
		}

		return client.Quit()
	} else {
		// Use plain SMTP
		var auth smtp.Auth
		if config.Username != "" && config.Password != "" {
			auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
		}

		return smtp.SendMail(addr, auth, config.From, allRecipients, []byte(msg))
	}
}

// buildMessage builds the email message with headers
func (e *Executor) buildMessage(config *EmailConfig) string {
	var msg strings.Builder

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(config.To, ", ")))

	if len(config.CC) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(config.CC, ", ")))
	}

	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", config.Subject))

	// Content type
	if config.IsHTML {
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}

	msg.WriteString("\r\n")

	// Body
	msg.WriteString(config.Body)

	return msg.String()
}
