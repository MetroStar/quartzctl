package log

import (
	"os"

	"go.uber.org/fx/fxevent"
)

// FxLogger is a logger implementation for Uber FX that uses a fallback logger until the default logger is configured.
type FxLogger struct {
	fallbackLogger fxevent.Logger
}

// NewFxLogger creates a new FxLogger instance. It uses the system's default logger if debugging is enabled,
// or a no-op logger otherwise. This suppresses initial DI container logs until the logger is fully configured.
func NewFxLogger() *FxLogger {
	if DebugEnvOverride() {
		return &FxLogger{
			fallbackLogger: &fxevent.ConsoleLogger{W: os.Stderr},
		}
	}

	return &FxLogger{
		fallbackLogger: &fxevent.NopLogger,
	}
}

// LogEvent logs an fxevent.Event. If the default logger is not yet configured, it uses the fallback logger.
func (l *FxLogger) LogEvent(event fxevent.Event) {
	if defaultLogger == nil {
		l.fallbackLogger.LogEvent(event)
		return
	}

	defaultLogger.LogEvent(event)
}
