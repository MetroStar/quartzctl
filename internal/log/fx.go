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
