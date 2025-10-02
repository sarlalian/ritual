// ABOUTME: Tests for SES task executor
// ABOUTME: Validates email sending via Amazon SES with various configurations

package ses

import (
	"context"
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

// MockContextManager for testing
type MockContextManager struct{}

func (m *MockContextManager) Initialize(workflow *types.Workflow, envVars []string) error {
	return nil
}

func (m *MockContextManager) GetContext() *types.WorkflowContext {
	return &types.WorkflowContext{}
}

func (m *MockContextManager) GetVariable(name string) (interface{}, error) {
	return nil, nil
}

func (m *MockContextManager) SetVariable(name string, value interface{}) error {
	return nil
}

func (m *MockContextManager) GetEnvironment(name, defaultValue string) string {
	return defaultValue
}

func (m *MockContextManager) SetEnvironment(name, value string) error {
	return nil
}

func (m *MockContextManager) RegisterTaskResult(taskResult *types.TaskResult) error {
	return nil
}

func (m *MockContextManager) GetTaskResult(identifier string) (*types.TaskResult, error) {
	return nil, nil
}

func (m *MockContextManager) EvaluateString(templateStr string) (string, error) {
	return templateStr, nil
}

func (m *MockContextManager) EvaluateMap(data map[string]interface{}) (map[string]interface{}, error) {
	return data, nil
}

func (m *MockContextManager) GetTemplateEngine() types.TemplateEngine {
	return &MockTemplateEngine{}
}

func (m *MockContextManager) Clone() types.ContextManager {
	return &MockContextManager{}
}

// MockTemplateEngine for testing
type MockTemplateEngine struct{}

func (m *MockTemplateEngine) Evaluate(template string, ctx *types.WorkflowContext) (string, error) {
	return template, nil
}

func (m *MockTemplateEngine) EvaluateAll(data map[string]interface{}, ctx *types.WorkflowContext) (map[string]interface{}, error) {
	return data, nil
}

func TestExecutor_New(t *testing.T) {
	executor := New()
	if executor == nil {
		t.Error("Expected executor to be created")
	}
}

func TestExecutor_SupportsDryRun(t *testing.T) {
	executor := New()
	if !executor.SupportsDryRun() {
		t.Error("Expected SES executor to support dry run")
	}
}

func TestExecutor_Validate_Valid(t *testing.T) {
	executor := New()

	validConfigs := []map[string]interface{}{
		{
			"region":  "us-east-1",
			"from":    "sender@example.com",
			"to":      []interface{}{"recipient@example.com"},
			"subject": "Test Email",
			"body":    "This is a test email",
		},
		{
			"region":    "us-west-2",
			"from":      "sender@example.com",
			"to":        []interface{}{"recipient1@example.com", "recipient2@example.com"},
			"subject":   "Test Email",
			"body_html": "<h1>Test Email</h1>",
		},
		{
			"region":        "eu-west-1",
			"from":          "sender@example.com",
			"to":            []interface{}{"recipient@example.com"},
			"subject":       "Test Email",
			"template":      "MyTemplate",
			"template_data": `{"name": "John"}`,
		},
		{
			"region":            "us-east-1",
			"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"from":              "sender@example.com",
			"to":                []interface{}{"recipient@example.com"},
			"cc":                []interface{}{"cc@example.com"},
			"bcc":               []interface{}{"bcc@example.com"},
			"subject":           "Test Email",
			"body":              "Test body",
			"reply_to":          []interface{}{"reply@example.com"},
			"configuration_set": "MyConfigSet",
		},
	}

	for i, config := range validConfigs {
		task := &types.TaskConfig{
			ID:     "test",
			Name:   "Test SES",
			Type:   "ses",
			Config: config,
		}

		err := executor.Validate(task)
		if err != nil {
			t.Errorf("Config %d: Expected valid config to pass validation, got: %v", i, err)
		}
	}
}

func TestExecutor_Validate_Invalid(t *testing.T) {
	executor := New()

	invalidConfigs := []struct {
		config map[string]interface{}
		reason string
	}{
		{
			config: map[string]interface{}{
				"from":    "sender@example.com",
				"to":      []interface{}{"recipient@example.com"},
				"subject": "Test",
				"body":    "Test",
			},
			reason: "missing region",
		},
		{
			config: map[string]interface{}{
				"region":  "us-east-1",
				"to":      []interface{}{"recipient@example.com"},
				"subject": "Test",
				"body":    "Test",
			},
			reason: "missing from",
		},
		{
			config: map[string]interface{}{
				"region":  "us-east-1",
				"from":    "sender@example.com",
				"subject": "Test",
				"body":    "Test",
			},
			reason: "missing to",
		},
		{
			config: map[string]interface{}{
				"region": "us-east-1",
				"from":   "sender@example.com",
				"to":     []interface{}{"recipient@example.com"},
				"body":   "Test",
			},
			reason: "missing subject",
		},
		{
			config: map[string]interface{}{
				"region":  "us-east-1",
				"from":    "sender@example.com",
				"to":      []interface{}{"recipient@example.com"},
				"subject": "Test",
			},
			reason: "missing body and template",
		},
		{
			config: map[string]interface{}{
				"region":  "us-east-1",
				"from":    "sender@example.com",
				"to":      []interface{}{},
				"subject": "Test",
				"body":    "Test",
			},
			reason: "empty to list",
		},
	}

	for i, test := range invalidConfigs {
		task := &types.TaskConfig{
			ID:     "test",
			Name:   "Test SES",
			Type:   "ses",
			Config: test.config,
		}

		err := executor.Validate(task)
		if err == nil {
			t.Errorf("Config %d (%s): Expected validation error", i, test.reason)
		}
	}
}

func TestExecutor_Execute_DryRun(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	task := &types.TaskConfig{
		ID:   "test-ses",
		Name: "Test SES Email",
		Type: "ses",
		Config: map[string]interface{}{
			"region":  "us-east-1",
			"from":    "sender@example.com",
			"to":      []interface{}{"recipient@example.com"},
			"subject": "Test Email",
			"body":    "This is a test email",
			"dry_run": true,
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskSuccess {
		t.Errorf("Expected task success in dry run, got %s: %s", result.Status, result.Message)
	}

	if result.Output["dry_run"] != true {
		t.Errorf("Expected dry_run=true in output, got %v", result.Output["dry_run"])
	}

	if result.Output["from"] != "sender@example.com" {
		t.Errorf("Expected from=sender@example.com, got %v", result.Output["from"])
	}
}

func TestExecutor_Execute_MissingRegion(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	task := &types.TaskConfig{
		ID:   "test-ses",
		Name: "Test SES Email",
		Type: "ses",
		Config: map[string]interface{}{
			"from":    "sender@example.com",
			"to":      []interface{}{"recipient@example.com"},
			"subject": "Test Email",
			"body":    "This is a test email",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for missing region, got %s", result.Status)
	}
}

func TestExecutor_Execute_MissingFrom(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	task := &types.TaskConfig{
		ID:   "test-ses",
		Name: "Test SES Email",
		Type: "ses",
		Config: map[string]interface{}{
			"region":  "us-east-1",
			"to":      []interface{}{"recipient@example.com"},
			"subject": "Test Email",
			"body":    "This is a test email",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for missing from, got %s", result.Status)
	}
}

func TestExecutor_Execute_MissingTo(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	task := &types.TaskConfig{
		ID:   "test-ses",
		Name: "Test SES Email",
		Type: "ses",
		Config: map[string]interface{}{
			"region":  "us-east-1",
			"from":    "sender@example.com",
			"subject": "Test Email",
			"body":    "This is a test email",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for missing to, got %s", result.Status)
	}
}

func TestExecutor_Execute_MissingSubject(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	task := &types.TaskConfig{
		ID:   "test-ses",
		Name: "Test SES Email",
		Type: "ses",
		Config: map[string]interface{}{
			"region": "us-east-1",
			"from":   "sender@example.com",
			"to":     []interface{}{"recipient@example.com"},
			"body":   "This is a test email",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for missing subject, got %s", result.Status)
	}
}

func TestExecutor_Execute_MissingBodyAndTemplate(t *testing.T) {
	executor := New()
	contextManager := &MockContextManager{}

	task := &types.TaskConfig{
		ID:   "test-ses",
		Name: "Test SES Email",
		Type: "ses",
		Config: map[string]interface{}{
			"region":  "us-east-1",
			"from":    "sender@example.com",
			"to":      []interface{}{"recipient@example.com"},
			"subject": "Test Email",
		},
	}

	result := executor.Execute(context.Background(), task, contextManager)

	if result.Status != types.TaskFailed {
		t.Errorf("Expected task failure for missing body and template, got %s", result.Status)
	}
}

func TestParseConfigRaw_ValidConfigs(t *testing.T) {
	executor := New()

	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name: "basic email",
			config: map[string]interface{}{
				"region":  "us-east-1",
				"from":    "sender@example.com",
				"to":      []interface{}{"recipient@example.com"},
				"subject": "Test",
				"body":    "Test body",
			},
		},
		{
			name: "with credentials",
			config: map[string]interface{}{
				"region":            "us-east-1",
				"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
				"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"from":              "sender@example.com",
				"to":                []interface{}{"recipient@example.com"},
				"subject":           "Test",
				"body":              "Test body",
			},
		},
		{
			name: "with multiple recipients",
			config: map[string]interface{}{
				"region":  "us-east-1",
				"from":    "sender@example.com",
				"to":      []interface{}{"r1@example.com", "r2@example.com"},
				"cc":      []interface{}{"cc@example.com"},
				"bcc":     []interface{}{"bcc@example.com"},
				"subject": "Test",
				"body":    "Test body",
			},
		},
		{
			name: "with template",
			config: map[string]interface{}{
				"region":        "us-east-1",
				"from":          "sender@example.com",
				"to":            []interface{}{"recipient@example.com"},
				"subject":       "Test",
				"template":      "MyTemplate",
				"template_data": `{"name": "John"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := executor.parseConfigRaw(tt.config)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if config == nil {
				t.Error("Expected config to be non-nil")
			}
		})
	}
}

func TestParseConfigRaw_TypeConversions(t *testing.T) {
	executor := New()

	config := map[string]interface{}{
		"region":            "us-east-1",
		"from":              "sender@example.com",
		"to":                []interface{}{"recipient@example.com"},
		"cc":                []interface{}{"cc@example.com"},
		"bcc":               []interface{}{"bcc@example.com"},
		"reply_to":          []interface{}{"reply@example.com"},
		"subject":           "Test Subject",
		"body":              "Test Body",
		"body_html":         "<h1>Test</h1>",
		"charset":           "UTF-8",
		"configuration_set": "MyConfigSet",
		"tags": map[string]interface{}{
			"Environment": "Test",
			"Team":        "Engineering",
		},
		"return_path":        "return@example.com",
		"return_path_arn":    "arn:aws:ses:us-east-1:123456789012:identity/return@example.com",
		"source_arn":         "arn:aws:ses:us-east-1:123456789012:identity/sender@example.com",
		"template":           "MyTemplate",
		"template_data":      `{"name": "John"}`,
		"template_arn":       "arn:aws:ses:us-east-1:123456789012:template/MyTemplate",
		"access_key_id":      "AKIAIOSFODNN7EXAMPLE",
		"secret_access_key":  "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"session_token":      "FwoGZXIvYXdzEBQaDCzExAmPlEcOdE",
	}

	result, err := executor.parseConfigRaw(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify all fields were parsed correctly
	if result.Region != "us-east-1" {
		t.Errorf("Expected region=us-east-1, got %s", result.Region)
	}
	if result.From != "sender@example.com" {
		t.Errorf("Expected from=sender@example.com, got %s", result.From)
	}
	if len(result.To) != 1 || result.To[0] != "recipient@example.com" {
		t.Errorf("Expected to=[recipient@example.com], got %v", result.To)
	}
	if len(result.CC) != 1 || result.CC[0] != "cc@example.com" {
		t.Errorf("Expected cc=[cc@example.com], got %v", result.CC)
	}
	if len(result.BCC) != 1 || result.BCC[0] != "bcc@example.com" {
		t.Errorf("Expected bcc=[bcc@example.com], got %v", result.BCC)
	}
	if result.Subject != "Test Subject" {
		t.Errorf("Expected subject='Test Subject', got %s", result.Subject)
	}
	if result.Body != "Test Body" {
		t.Errorf("Expected body='Test Body', got %s", result.Body)
	}
	if result.BodyHTML != "<h1>Test</h1>" {
		t.Errorf("Expected body_html='<h1>Test</h1>', got %s", result.BodyHTML)
	}
	if result.ConfigurationSet != "MyConfigSet" {
		t.Errorf("Expected configuration_set=MyConfigSet, got %s", result.ConfigurationSet)
	}
	if len(result.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(result.Tags))
	}
}
