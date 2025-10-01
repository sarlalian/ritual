// ABOUTME: Structured logging implementation using zerolog
// ABOUTME: Provides a consistent logging interface throughout the application

package utils

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/sarlalian/ritual/pkg/types"
)

// LogLevel represents logging levels
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

// Logger wraps zerolog.Logger to implement our Logger interface
type Logger struct {
	logger zerolog.Logger
}

// LogEvent wraps zerolog.Event to implement our LogEvent interface
type LogEvent struct {
	event *zerolog.Event
}

// LogContext wraps zerolog.Context to implement our LogContext interface
type LogContext struct {
	context zerolog.Context
}

// NewLogger creates a new structured logger
func NewLogger(level LogLevel, output io.Writer) types.Logger {
	if output == nil {
		output = os.Stderr
	}

	// Set global log level
	switch level {
	case DebugLevel:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case InfoLevel:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case WarnLevel:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case ErrorLevel:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Create logger with console writer for human-readable output
	consoleWriter := zerolog.ConsoleWriter{
		Out:        output,
		TimeFormat: time.RFC3339,
		NoColor:    os.Getenv("NO_COLOR") != "",
	}

	logger := zerolog.New(consoleWriter).
		With().
		Timestamp().
		Caller().
		Logger()

	return &Logger{logger: logger}
}

// NewJSONLogger creates a new JSON logger for structured output
func NewJSONLogger(level LogLevel, output io.Writer) types.Logger {
	if output == nil {
		output = os.Stderr
	}

	// Set global log level
	switch level {
	case DebugLevel:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case InfoLevel:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case WarnLevel:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case ErrorLevel:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	logger := zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	return &Logger{logger: logger}
}

// Debug logs a debug message
func (l *Logger) Debug() types.LogEvent {
	return &LogEvent{event: l.logger.Debug()}
}

// Info logs an info message
func (l *Logger) Info() types.LogEvent {
	return &LogEvent{event: l.logger.Info()}
}

// Warn logs a warning message
func (l *Logger) Warn() types.LogEvent {
	return &LogEvent{event: l.logger.Warn()}
}

// Error logs an error message
func (l *Logger) Error() types.LogEvent {
	return &LogEvent{event: l.logger.Error()}
}

// With returns a logger with additional context
func (l *Logger) With() types.LogContext {
	return &LogContext{context: l.logger.With()}
}

// Str adds a string field
func (e *LogEvent) Str(key, val string) types.LogEvent {
	e.event = e.event.Str(key, val)
	return e
}

// Int adds an integer field
func (e *LogEvent) Int(key string, val int) types.LogEvent {
	e.event = e.event.Int(key, val)
	return e
}

// Dur adds a duration field
func (e *LogEvent) Dur(key string, val time.Duration) types.LogEvent {
	e.event = e.event.Dur(key, val)
	return e
}

// Err adds an error field
func (e *LogEvent) Err(err error) types.LogEvent {
	e.event = e.event.Err(err)
	return e
}

// Bool adds a boolean field
func (e *LogEvent) Bool(key string, val bool) types.LogEvent {
	e.event = e.event.Bool(key, val)
	return e
}

// Any adds an arbitrary field
func (e *LogEvent) Any(key string, val interface{}) types.LogEvent {
	e.event = e.event.Interface(key, val)
	return e
}

// Msg logs the event with a message
func (e *LogEvent) Msg(msg string) {
	e.event.Msg(msg)
}

// Msgf logs the event with a formatted message
func (e *LogEvent) Msgf(format string, args ...interface{}) {
	e.event.Msgf(format, args...)
}

// Str adds a string field to the context
func (c *LogContext) Str(key, val string) types.LogContext {
	c.context = c.context.Str(key, val)
	return c
}

// Logger returns the logger with the built context
func (c *LogContext) Logger() types.Logger {
	return &Logger{logger: c.context.Logger()}
}

// NewTaskLogger creates a logger with task-specific context
func NewTaskLogger(baseLogger types.Logger, taskID, taskName, taskType string) types.Logger {
	return baseLogger.With().
		Str("task_id", taskID).
		Str("task_name", taskName).
		Str("task_type", taskType).
		Logger()
}

// NewWorkflowLogger creates a logger with workflow-specific context
func NewWorkflowLogger(baseLogger types.Logger, workflowName string) types.Logger {
	return baseLogger.With().
		Str("workflow", workflowName).
		Logger()
}
