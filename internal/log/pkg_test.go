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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxevent"
)

func TestLogDefaultSettings(t *testing.T) {
	msg := "test log entry"

	Debug(msg)
	Info(msg)
	Warn(msg)
	Error(msg)
	Sync()
}

func TestDefault_InitializesLoggerOnce(t *testing.T) {
	// Reset the defaultLogger and defaultLoggerOnce for testing
	defaultLogger = nil
	defaultLoggerOnce = sync.Once{}

	logger := Default()
	assert.NotNil(t, logger, "Default logger should be initialized")
	assert.Equal(t, logger, defaultLogger, "Default logger should match the initialized logger")

	// Call Default again to ensure it does not reinitialize
	anotherLogger := Default()
	assert.Equal(t, logger, anotherLogger, "Default logger should not be reinitialized")
}

func TestDefault_UsesExistingLogger(t *testing.T) {
	// Set a mock logger as the defaultLogger
	mockLogger := &mockAppLogger{}
	defaultLogger = mockLogger

	logger := Default()
	assert.Equal(t, mockLogger, logger, "Default should return the existing logger")
}

func TestDefault_InitializesWithDefaultConfig(t *testing.T) {
	// Reset the defaultLogger and defaultLoggerOnce for testing
	defaultLogger = nil
	defaultLoggerOnce = sync.Once{}

	logger := Default()
	assert.NotNil(t, logger, "Default logger should be initialized")
	assert.IsType(t, &ZapLogger{}, logger, "Default logger should be of type ZapLogger")
}

type mockAppLogger struct{}

func (m *mockAppLogger) Sync() error                                   { return nil }
func (m *mockAppLogger) Debug(msg interface{}, keyvals ...interface{}) {}
func (m *mockAppLogger) Info(msg interface{}, keyvals ...interface{})  {}
func (m *mockAppLogger) Warn(msg interface{}, keyvals ...interface{})  {}
func (m *mockAppLogger) Error(msg interface{}, keyvals ...interface{}) {}
func (m *mockAppLogger) LogEvent(event fxevent.Event)                  {}
