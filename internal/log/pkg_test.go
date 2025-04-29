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
