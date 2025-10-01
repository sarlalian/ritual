// ABOUTME: Logger wrapper that adapts zerolog to the types.Logger interface
// ABOUTME: Provides compatibility between external logging libraries and internal interfaces

package logging

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/sarlalian/ritual/pkg/types"
)

// ZerologWrapper wraps zerolog.Logger to implement types.Logger
type ZerologWrapper struct {
	logger zerolog.Logger
}

// NewZerologWrapper creates a new logger wrapper
func NewZerologWrapper(logger zerolog.Logger) types.Logger {
	return &ZerologWrapper{logger: logger}
}

// Debug implements types.Logger
func (z *ZerologWrapper) Debug() types.LogEvent {
	return &ZerologEvent{event: z.logger.Debug()}
}

// Info implements types.Logger
func (z *ZerologWrapper) Info() types.LogEvent {
	return &ZerologEvent{event: z.logger.Info()}
}

// Warn implements types.Logger
func (z *ZerologWrapper) Warn() types.LogEvent {
	return &ZerologEvent{event: z.logger.Warn()}
}

// Error implements types.Logger
func (z *ZerologWrapper) Error() types.LogEvent {
	return &ZerologEvent{event: z.logger.Error()}
}

// With implements types.Logger
func (z *ZerologWrapper) With() types.LogContext {
	return &ZerologContext{context: z.logger.With()}
}

// ZerologEvent wraps zerolog.Event to implement types.LogEvent
type ZerologEvent struct {
	event *zerolog.Event
}

// Str implements types.LogEvent
func (e *ZerologEvent) Str(key, val string) types.LogEvent {
	return &ZerologEvent{event: e.event.Str(key, val)}
}

// Int implements types.LogEvent
func (e *ZerologEvent) Int(key string, val int) types.LogEvent {
	return &ZerologEvent{event: e.event.Int(key, val)}
}

// Dur implements types.LogEvent
func (e *ZerologEvent) Dur(key string, val time.Duration) types.LogEvent {
	return &ZerologEvent{event: e.event.Dur(key, val)}
}

// Err implements types.LogEvent
func (e *ZerologEvent) Err(err error) types.LogEvent {
	return &ZerologEvent{event: e.event.Err(err)}
}

// Any implements types.LogEvent
func (e *ZerologEvent) Any(key string, val interface{}) types.LogEvent {
	return &ZerologEvent{event: e.event.Interface(key, val)}
}

// Bool implements types.LogEvent (additional method not in interface but used)
func (e *ZerologEvent) Bool(key string, val bool) types.LogEvent {
	return &ZerologEvent{event: e.event.Bool(key, val)}
}

// Msg implements types.LogEvent
func (e *ZerologEvent) Msg(msg string) {
	e.event.Msg(msg)
}

// Msgf implements types.LogEvent
func (e *ZerologEvent) Msgf(format string, args ...interface{}) {
	e.event.Msgf(format, args...)
}

// ZerologContext wraps zerolog.Context to implement types.LogContext
type ZerologContext struct {
	context zerolog.Context
}

// Str implements types.LogContext
func (c *ZerologContext) Str(key, val string) types.LogContext {
	return &ZerologContext{context: c.context.Str(key, val)}
}

// Int implements types.LogContext
func (c *ZerologContext) Int(key string, val int) types.LogContext {
	return &ZerologContext{context: c.context.Int(key, val)}
}

// Logger implements types.LogContext
func (c *ZerologContext) Logger() types.Logger {
	return &ZerologWrapper{logger: c.context.Logger()}
}
