// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"os"
	"sync"

	"go.uber.org/fx/fxevent"
)

var (
	// defaultLoggerOnce ensures the default logger is initialized only once.
	defaultLoggerOnce sync.Once
	// defaultLogger holds the global logger instance.
	defaultLogger AppLogger
)

// AppLogger defines the interface for application loggers.
type AppLogger interface {
	Sync() error
	Debug(msg interface{}, keyvals ...interface{})
	Info(msg interface{}, keyvals ...interface{})
	Warn(msg interface{}, keyvals ...interface{})
	Error(msg interface{}, keyvals ...interface{})
	LogEvent(event fxevent.Event)
}

// Default returns the default global logger instance.
// It initializes the logger if it has not been set already.
func Default() AppLogger {
	defaultLoggerOnce.Do(func() {
		if defaultLogger == nil {
			// Initialize the default logger with default configuration.
			defaultLogger = NewZapLogger(DefaultLogConfig, os.Stderr)
		}
	})
	return defaultLogger
}

// SetDefault sets the provided logger as the default global logger.
func SetDefault(logger AppLogger) {
	defaultLogger = logger
}

// Sync flushes any buffered log entries from the default logger.
func Sync() error {
	l := Default()
	if l == nil {
		return nil
	}
	return l.Sync()
}

// Debug logs a debug message using the default logger.
func Debug(msg interface{}, keyvals ...interface{}) {
	Default().Debug(msg, keyvals...)
}

// Info logs an informational message using the default logger.
func Info(msg interface{}, keyvals ...interface{}) {
	Default().Info(msg, keyvals...)
}

// Warn logs a warning message using the default logger.
func Warn(msg interface{}, keyvals ...interface{}) {
	Default().Warn(msg, keyvals...)
}

// Error logs an error message using the default logger.
func Error(msg interface{}, keyvals ...interface{}) {
	Default().Error(msg, keyvals...)
}
