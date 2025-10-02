// ABOUTME: SES task executor for sending emails via Amazon Simple Email Service
// ABOUTME: Supports templates, attachments, configuration sets, and multiple recipients

package ses

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"

	"github.com/sarlalian/ritual/pkg/types"
)

// Executor handles SES task execution
type Executor struct{}

// SESConfig represents the configuration for an SES email task
type SESConfig struct {
	// AWS Region (e.g., "us-east-1")
	Region string `yaml:"region" json:"region"`

	// AWS Access Key ID (optional if using IAM role or env vars)
	AccessKeyID string `yaml:"access_key_id,omitempty" json:"access_key_id,omitempty"`

	// AWS Secret Access Key (optional if using IAM role or env vars)
	SecretAccessKey string `yaml:"secret_access_key,omitempty" json:"secret_access_key,omitempty"`

	// AWS Session Token (optional for temporary credentials)
	SessionToken string `yaml:"session_token,omitempty" json:"session_token,omitempty"`

	// From address (must be verified in SES)
	From string `yaml:"from" json:"from"`

	// From name (optional display name)
	FromName string `yaml:"from_name,omitempty" json:"from_name,omitempty"`

	// To addresses
	To []string `yaml:"to" json:"to"`

	// CC addresses (optional)
	CC []string `yaml:"cc,omitempty" json:"cc,omitempty"`

	// BCC addresses (optional)
	BCC []string `yaml:"bcc,omitempty" json:"bcc,omitempty"`

	// Reply-To addresses (optional)
	ReplyTo []string `yaml:"reply_to,omitempty" json:"reply_to,omitempty"`

	// Subject line
	Subject string `yaml:"subject" json:"subject"`

	// Email body (plain text)
	Body string `yaml:"body,omitempty" json:"body,omitempty"`

	// Email body (HTML)
	BodyHTML string `yaml:"body_html,omitempty" json:"body_html,omitempty"`

	// Character set (default: UTF-8)
	Charset string `yaml:"charset,omitempty" json:"charset,omitempty"`

	// Configuration set name (optional)
	ConfigurationSet string `yaml:"configuration_set,omitempty" json:"configuration_set,omitempty"`

	// Return path for bounces (optional)
	ReturnPath string `yaml:"return_path,omitempty" json:"return_path,omitempty"`

	// Source ARN (optional, for cross-account sending)
	SourceArn string `yaml:"source_arn,omitempty" json:"source_arn,omitempty"`

	// Return path ARN (optional)
	ReturnPathArn string `yaml:"return_path_arn,omitempty" json:"return_path_arn,omitempty"`

	// Tags for the email (optional)
	Tags map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// Use SES template instead of body (optional)
	Template string `yaml:"template,omitempty" json:"template,omitempty"`

	// Template data (JSON string for template variables)
	TemplateData string `yaml:"template_data,omitempty" json:"template_data,omitempty"`

	// Dry run mode (simulates email sending without actually sending)
	DryRun bool `yaml:"dry_run,omitempty" json:"dry_run,omitempty"`
}

// New creates a new SES executor
func New() *Executor {
	return &Executor{}
}

// Execute runs an SES email task
func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig, contextManager types.ContextManager) *types.TaskResult {
	result := &types.TaskResult{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		StartTime: time.Now(),
		Status:    types.TaskRunning,
		Output:    make(map[string]interface{}),
	}

	// Parse configuration
	config, err := e.parseConfig(task, contextManager)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to parse configuration: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Set default charset
	if config.Charset == "" {
		config.Charset = "UTF-8"
	}

	// Handle dry run mode
	if config.DryRun {
		result.Status = types.TaskSuccess
		result.Message = "Dry run: Email would be sent via SES (not actually sent)"
		result.Output["dry_run"] = true
		result.Output["from"] = config.From
		result.Output["to"] = config.To
		result.Output["subject"] = config.Subject
		result.Output["region"] = config.Region
		if config.Template != "" {
			result.Output["template"] = config.Template
		}
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Send email via SES
	messageID, err := e.sendEmail(ctx, config)
	if err != nil {
		result.Status = types.TaskFailed
		result.Message = fmt.Sprintf("Failed to send email via SES: %v", err)
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Success
	result.Status = types.TaskSuccess
	result.Message = fmt.Sprintf("Email sent successfully via SES (MessageID: %s)", messageID)
	result.Output["message_id"] = messageID
	result.Output["from"] = config.From
	result.Output["to"] = config.To
	result.Output["subject"] = config.Subject
	result.Output["region"] = config.Region

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// Validate checks if the task configuration is valid
func (e *Executor) Validate(task *types.TaskConfig) error {
	config, err := e.parseConfigRaw(task.Config)
	if err != nil {
		return fmt.Errorf("invalid SES configuration: %w", err)
	}

	// Region is required
	if config.Region == "" {
		return fmt.Errorf("SES task must specify 'region'")
	}

	// From is required
	if config.From == "" {
		return fmt.Errorf("SES task must specify 'from' address")
	}

	// To addresses are required
	if len(config.To) == 0 {
		return fmt.Errorf("SES task must specify at least one 'to' address")
	}

	// Subject is required
	if config.Subject == "" {
		return fmt.Errorf("SES task must specify 'subject'")
	}

	// Body or BodyHTML or Template is required
	if config.Body == "" && config.BodyHTML == "" && config.Template == "" {
		return fmt.Errorf("SES task must specify 'body', 'body_html', or 'template'")
	}

	// If using template, template_data is required
	if config.Template != "" && config.TemplateData == "" {
		return fmt.Errorf("SES task using template must specify 'template_data'")
	}

	return nil
}

// SupportsDryRun returns true as SES operations can be simulated
func (e *Executor) SupportsDryRun() bool {
	return true
}

// parseConfig parses and evaluates the configuration with template evaluation
func (e *Executor) parseConfig(task *types.TaskConfig, contextManager types.ContextManager) (*SESConfig, error) {
	// First evaluate templates in the config
	evaluatedConfig, err := contextManager.EvaluateMap(task.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate templates: %w", err)
	}

	// Then parse into struct
	return e.parseConfigRaw(evaluatedConfig)
}

// parseConfigRaw parses raw configuration map into SESConfig struct
func (e *Executor) parseConfigRaw(config map[string]interface{}) (*SESConfig, error) {
	sesConfig := &SESConfig{
		Charset: "UTF-8",
	}

	for key, value := range config {
		switch key {
		case "region":
			if str, ok := value.(string); ok {
				sesConfig.Region = str
			}
		case "access_key_id":
			if str, ok := value.(string); ok {
				sesConfig.AccessKeyID = str
			}
		case "secret_access_key":
			if str, ok := value.(string); ok {
				sesConfig.SecretAccessKey = str
			}
		case "session_token":
			if str, ok := value.(string); ok {
				sesConfig.SessionToken = str
			}
		case "from":
			if str, ok := value.(string); ok {
				sesConfig.From = str
			}
		case "from_name":
			if str, ok := value.(string); ok {
				sesConfig.FromName = str
			}
		case "to":
			if str, ok := value.(string); ok {
				sesConfig.To = []string{str}
			} else if list, ok := value.([]interface{}); ok {
				for _, item := range list {
					if str, ok := item.(string); ok {
						sesConfig.To = append(sesConfig.To, str)
					}
				}
			} else if list, ok := value.([]string); ok {
				sesConfig.To = list
			}
		case "cc":
			if list, ok := value.([]interface{}); ok {
				for _, item := range list {
					if str, ok := item.(string); ok {
						sesConfig.CC = append(sesConfig.CC, str)
					}
				}
			} else if list, ok := value.([]string); ok {
				sesConfig.CC = list
			}
		case "bcc":
			if list, ok := value.([]interface{}); ok {
				for _, item := range list {
					if str, ok := item.(string); ok {
						sesConfig.BCC = append(sesConfig.BCC, str)
					}
				}
			} else if list, ok := value.([]string); ok {
				sesConfig.BCC = list
			}
		case "reply_to":
			if list, ok := value.([]interface{}); ok {
				for _, item := range list {
					if str, ok := item.(string); ok {
						sesConfig.ReplyTo = append(sesConfig.ReplyTo, str)
					}
				}
			} else if list, ok := value.([]string); ok {
				sesConfig.ReplyTo = list
			}
		case "subject":
			if str, ok := value.(string); ok {
				sesConfig.Subject = str
			}
		case "body":
			if str, ok := value.(string); ok {
				sesConfig.Body = str
			}
		case "body_html":
			if str, ok := value.(string); ok {
				sesConfig.BodyHTML = str
			}
		case "charset":
			if str, ok := value.(string); ok {
				sesConfig.Charset = str
			}
		case "configuration_set":
			if str, ok := value.(string); ok {
				sesConfig.ConfigurationSet = str
			}
		case "return_path":
			if str, ok := value.(string); ok {
				sesConfig.ReturnPath = str
			}
		case "source_arn":
			if str, ok := value.(string); ok {
				sesConfig.SourceArn = str
			}
		case "return_path_arn":
			if str, ok := value.(string); ok {
				sesConfig.ReturnPathArn = str
			}
		case "tags":
			if m, ok := value.(map[string]interface{}); ok {
				sesConfig.Tags = make(map[string]string)
				for k, v := range m {
					if str, ok := v.(string); ok {
						sesConfig.Tags[k] = str
					}
				}
			} else if m, ok := value.(map[string]string); ok {
				sesConfig.Tags = m
			}
		case "template":
			if str, ok := value.(string); ok {
				sesConfig.Template = str
			}
		case "template_data":
			if str, ok := value.(string); ok {
				sesConfig.TemplateData = str
			}
		case "dry_run":
			if b, ok := value.(bool); ok {
				sesConfig.DryRun = b
			}
		}
	}

	return sesConfig, nil
}

// sendEmail sends an email using AWS SES
func (e *Executor) sendEmail(ctx context.Context, config *SESConfig) (string, error) {
	// Create AWS session
	awsConfig := &aws.Config{
		Region: aws.String(config.Region),
	}

	// Add credentials if provided
	if config.AccessKeyID != "" && config.SecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(
			config.AccessKeyID,
			config.SecretAccessKey,
			config.SessionToken,
		)
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create SES client
	svc := ses.New(sess)

	// Build from address with optional name
	fromAddress := config.From
	if config.FromName != "" {
		fromAddress = fmt.Sprintf("%s <%s>", config.FromName, config.From)
	}

	// Use template if specified
	if config.Template != "" {
		return e.sendTemplatedEmail(ctx, svc, config, fromAddress)
	}

	// Build destination
	destination := &ses.Destination{
		ToAddresses: aws.StringSlice(config.To),
	}

	if len(config.CC) > 0 {
		destination.CcAddresses = aws.StringSlice(config.CC)
	}

	if len(config.BCC) > 0 {
		destination.BccAddresses = aws.StringSlice(config.BCC)
	}

	// Build message body
	body := &ses.Body{}
	if config.Body != "" {
		body.Text = &ses.Content{
			Charset: aws.String(config.Charset),
			Data:    aws.String(config.Body),
		}
	}
	if config.BodyHTML != "" {
		body.Html = &ses.Content{
			Charset: aws.String(config.Charset),
			Data:    aws.String(config.BodyHTML),
		}
	}

	// Build message
	message := &ses.Message{
		Subject: &ses.Content{
			Charset: aws.String(config.Charset),
			Data:    aws.String(config.Subject),
		},
		Body: body,
	}

	// Build send email input
	input := &ses.SendEmailInput{
		Source:      aws.String(fromAddress),
		Destination: destination,
		Message:     message,
	}

	// Add optional fields
	if len(config.ReplyTo) > 0 {
		input.ReplyToAddresses = aws.StringSlice(config.ReplyTo)
	}

	if config.ReturnPath != "" {
		input.ReturnPath = aws.String(config.ReturnPath)
	}

	if config.ConfigurationSet != "" {
		input.ConfigurationSetName = aws.String(config.ConfigurationSet)
	}

	if config.SourceArn != "" {
		input.SourceArn = aws.String(config.SourceArn)
	}

	if config.ReturnPathArn != "" {
		input.ReturnPathArn = aws.String(config.ReturnPathArn)
	}

	if len(config.Tags) > 0 {
		var tags []*ses.MessageTag
		for key, value := range config.Tags {
			tags = append(tags, &ses.MessageTag{
				Name:  aws.String(key),
				Value: aws.String(value),
			})
		}
		input.Tags = tags
	}

	// Send email
	result, err := svc.SendEmailWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("SES SendEmail failed: %w", err)
	}

	return aws.StringValue(result.MessageId), nil
}

// sendTemplatedEmail sends an email using an SES template
func (e *Executor) sendTemplatedEmail(ctx context.Context, svc *ses.SES, config *SESConfig, fromAddress string) (string, error) {
	// Build destination
	destination := &ses.Destination{
		ToAddresses: aws.StringSlice(config.To),
	}

	if len(config.CC) > 0 {
		destination.CcAddresses = aws.StringSlice(config.CC)
	}

	if len(config.BCC) > 0 {
		destination.BccAddresses = aws.StringSlice(config.BCC)
	}

	// Build send templated email input
	input := &ses.SendTemplatedEmailInput{
		Source:       aws.String(fromAddress),
		Destination:  destination,
		Template:     aws.String(config.Template),
		TemplateData: aws.String(config.TemplateData),
	}

	// Add optional fields
	if len(config.ReplyTo) > 0 {
		input.ReplyToAddresses = aws.StringSlice(config.ReplyTo)
	}

	if config.ReturnPath != "" {
		input.ReturnPath = aws.String(config.ReturnPath)
	}

	if config.ConfigurationSet != "" {
		input.ConfigurationSetName = aws.String(config.ConfigurationSet)
	}

	if config.SourceArn != "" {
		input.SourceArn = aws.String(config.SourceArn)
	}

	if config.ReturnPathArn != "" {
		input.ReturnPathArn = aws.String(config.ReturnPathArn)
	}

	if len(config.Tags) > 0 {
		var tags []*ses.MessageTag
		for key, value := range config.Tags {
			tags = append(tags, &ses.MessageTag{
				Name:  aws.String(key),
				Value: aws.String(value),
			})
		}
		input.Tags = tags
	}

	// Send templated email
	result, err := svc.SendTemplatedEmailWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("SES SendTemplatedEmail failed: %w", err)
	}

	return aws.StringValue(result.MessageId), nil
}

// BuildMessage builds an email message string (for dry run or testing)
func BuildMessage(config *SESConfig) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("From: %s\n", config.From))
	builder.WriteString(fmt.Sprintf("To: %s\n", strings.Join(config.To, ", ")))

	if len(config.CC) > 0 {
		builder.WriteString(fmt.Sprintf("CC: %s\n", strings.Join(config.CC, ", ")))
	}

	if len(config.BCC) > 0 {
		builder.WriteString(fmt.Sprintf("BCC: %s\n", strings.Join(config.BCC, ", ")))
	}

	builder.WriteString(fmt.Sprintf("Subject: %s\n", config.Subject))
	builder.WriteString("\n")

	if config.Template != "" {
		builder.WriteString(fmt.Sprintf("Template: %s\n", config.Template))
		builder.WriteString(fmt.Sprintf("Template Data: %s\n", config.TemplateData))
	} else {
		if config.BodyHTML != "" {
			builder.WriteString("[HTML Body]\n")
			builder.WriteString(config.BodyHTML)
		} else {
			builder.WriteString(config.Body)
		}
	}

	return builder.String()
}
