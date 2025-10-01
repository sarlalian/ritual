// ABOUTME: Template engine with Sprig function integration
// ABOUTME: Handles template evaluation for workflow variables and configurations

package template

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"

	"github.com/sarlalian/ritual/pkg/types"
)

// Engine implements the template engine interface
type Engine struct {
	funcMap template.FuncMap
}

// New creates a new template engine with Sprig functions
func New() types.TemplateEngine {
	engine := &Engine{
		funcMap: make(template.FuncMap),
	}

	// Add all Sprig functions
	for name, fn := range sprig.TxtFuncMap() {
		engine.funcMap[name] = fn
	}

	// Add custom functions
	engine.addCustomFunctions()

	return engine
}

// addCustomFunctions adds custom template functions
func (e *Engine) addCustomFunctions() {
	customFuncs := template.FuncMap{
		// Environment functions
		"env": func(name string, defaultValue ...string) string {
			if value := os.Getenv(name); value != "" {
				return value
			}
			if len(defaultValue) > 0 {
				return defaultValue[0]
			}
			return ""
		},

		// Hostname function
		"hostname": func() string {
			if hostname, err := os.Hostname(); err == nil {
				return hostname
			}
			return "unknown"
		},

		// Time functions (override Sprig's now to be more consistent)
		"now": func() time.Time {
			return time.Now()
		},

		"timestamp": func() string {
			return time.Now().Format(time.RFC3339)
		},

		"unixTimestamp": func() int64 {
			return time.Now().Unix()
		},

		// String manipulation functions
		"contains": strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"split": strings.Split,
		"join": strings.Join,
		"trim": strings.TrimSpace,
		"toLower": strings.ToLower,
		"toUpper": strings.ToUpper,

		// Context access functions (will be populated per evaluation)
		"getVar": func(name string) interface{} {
			return fmt.Sprintf("{{CONTEXT_VAR_%s}}", name)
		},
		"getTask": func(name string) interface{} {
			return fmt.Sprintf("{{CONTEXT_TASK_%s}}", name)
		},
		"getEnv": func(name string) string {
			return fmt.Sprintf("{{CONTEXT_ENV_%s}}", name)
		},
	}

	// Merge custom functions with existing funcMap
	for name, fn := range customFuncs {
		e.funcMap[name] = fn
	}
}

// Evaluate evaluates a template string with the given context
func (e *Engine) Evaluate(templateStr string, ctx *types.WorkflowContext) (string, error) {
	if templateStr == "" {
		return "", nil
	}

	// Skip evaluation if no template syntax is present
	if !strings.Contains(templateStr, "{{") || !strings.Contains(templateStr, "}}") {
		return templateStr, nil
	}

	// Create a new function map with context-aware functions
	contextFuncMap := e.createContextFuncMap(ctx)

	// Create and parse the template with strict error checking for missing keys
	tmpl, err := template.New("template").Option("missingkey=error").Funcs(contextFuncMap).Parse(templateStr)
	if err != nil {
		return "", types.NewTemplateError(templateStr, "", "failed to parse template", err)
	}

	// Create template data structure
	data := e.createTemplateData(ctx)

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", types.NewTemplateError(templateStr, "", "failed to execute template", err)
	}

	result := buf.String()

	// Post-process any remaining context placeholders
	result = e.postProcessContext(result, ctx)

	return result, nil
}

// EvaluateAll evaluates all template strings in a map
func (e *Engine) EvaluateAll(data map[string]interface{}, ctx *types.WorkflowContext) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range data {
		evaluatedValue, err := e.evaluateValue(value, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate key '%s': %w", key, err)
		}
		result[key] = evaluatedValue
	}

	return result, nil
}

// evaluateValue evaluates a single value (recursive for nested structures)
func (e *Engine) evaluateValue(value interface{}, ctx *types.WorkflowContext) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return e.Evaluate(v, ctx)

	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			evaluatedKey, err := e.Evaluate(key, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate map key '%s': %w", key, err)
			}
			evaluatedVal, err := e.evaluateValue(val, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate map value for key '%s': %w", key, err)
			}
			result[evaluatedKey] = evaluatedVal
		}
		return result, nil

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			evaluatedVal, err := e.evaluateValue(val, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate array index %d: %w", i, err)
			}
			result[i] = evaluatedVal
		}
		return result, nil

	default:
		// Return non-string values as-is
		return value, nil
	}
}

// createContextFuncMap creates a function map with context-aware functions
func (e *Engine) createContextFuncMap(ctx *types.WorkflowContext) template.FuncMap {
	contextFuncMap := make(template.FuncMap)

	// Copy base functions
	for name, fn := range e.funcMap {
		contextFuncMap[name] = fn
	}

	// Override context functions with actual implementations
	contextFuncMap["getVar"] = func(name string) interface{} {
		if ctx != nil && ctx.Variables != nil {
			return ctx.Variables[name]
		}
		return nil
	}

	contextFuncMap["getTask"] = func(name string) interface{} {
		if ctx != nil && ctx.Tasks != nil {
			return ctx.Tasks[name]
		}
		return nil
	}

	contextFuncMap["getEnv"] = func(name string) string {
		if ctx != nil && ctx.Environment != nil {
			if value, exists := ctx.Environment[name]; exists {
				return value
			}
		}
		return os.Getenv(name)
	}

	return contextFuncMap
}

// createTemplateData creates the data structure available to templates
func (e *Engine) createTemplateData(ctx *types.WorkflowContext) map[string]interface{} {
	data := make(map[string]interface{})

	if ctx != nil {
		// Environment variables
		if ctx.Environment != nil {
			data["environment"] = ctx.Environment
			data["env"] = ctx.Environment // Shorthand alias
		}

		// Workflow variables
		if ctx.Variables != nil {
			data["variables"] = ctx.Variables
			data["vars"] = ctx.Variables // Shorthand alias
		}

		// Task results
		if ctx.Tasks != nil {
			data["tasks"] = ctx.Tasks
		}

		// Metadata
		if ctx.Metadata != nil {
			data["metadata"] = ctx.Metadata
			// Also expose workflow metadata directly
			if workflow, exists := ctx.Metadata["workflow"]; exists {
				data["workflow"] = workflow
			}
		}
	}

	return data
}

// postProcessContext handles any remaining context placeholders
func (e *Engine) postProcessContext(text string, ctx *types.WorkflowContext) string {
	if ctx == nil {
		return text
	}

	// Replace environment placeholders
	if ctx.Environment != nil {
		for key, value := range ctx.Environment {
			placeholder := fmt.Sprintf("{{CONTEXT_ENV_%s}}", key)
			text = strings.ReplaceAll(text, placeholder, value)
		}
	}

	// Replace variable placeholders
	if ctx.Variables != nil {
		for key, value := range ctx.Variables {
			placeholder := fmt.Sprintf("{{CONTEXT_VAR_%s}}", key)
			if strValue, ok := value.(string); ok {
				text = strings.ReplaceAll(text, placeholder, strValue)
			}
		}
	}

	// Replace task placeholders
	if ctx.Tasks != nil {
		for key, task := range ctx.Tasks {
			placeholder := fmt.Sprintf("{{CONTEXT_TASK_%s}}", key)
			if task != nil {
				// For now, just replace with task status
				text = strings.ReplaceAll(text, placeholder, string(task.Status))
			}
		}
	}

	return text
}

// ValidateTemplate validates a template string without executing it
func ValidateTemplate(templateStr string) error {
	if templateStr == "" {
		return nil
	}

	if !strings.Contains(templateStr, "{{") || !strings.Contains(templateStr, "}}") {
		return nil // No template syntax, considered valid
	}

	engine := New()
	funcMap := engine.(*Engine).funcMap

	_, err := template.New("validation").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return types.NewTemplateError(templateStr, "", "invalid template syntax", err)
	}

	return nil
}

// EvaluateString is a convenience function for evaluating simple templates
func EvaluateString(templateStr string, ctx *types.WorkflowContext) (string, error) {
	engine := New()
	return engine.Evaluate(templateStr, ctx)
}