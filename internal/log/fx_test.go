package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxevent"
)

func TestNewFxLogger_DebugMode(t *testing.T) {
	t.Setenv("DEBUG", "true")

	logger := NewFxLogger()
	assert.IsType(t, &fxevent.ConsoleLogger{}, logger.fallbackLogger)
	assert.Equal(t, os.Stderr, logger.fallbackLogger.(*fxevent.ConsoleLogger).W)
}

func TestNewFxLogger_NonDebugMode(t *testing.T) {
	t.Setenv("DEBUG", "false")

	logger := NewFxLogger()
	assert.IsType(t, &fxevent.NopLogger, logger.fallbackLogger)
}

func TestFxLogger_LogEvent_WithDefaultLogger(t *testing.T) {
	// Mock defaultLogger
	mockLogger := &mockFxEventLogger{}
	defaultLogger = mockLogger
	defer func() { defaultLogger = nil }()

	logger := NewFxLogger()
	event := &fxevent.Invoked{}
	logger.LogEvent(event)

	assert.Equal(t, 1, mockLogger.logEventCallCount)
	assert.Equal(t, event, mockLogger.lastEvent)
}

func TestFxLogger_LogEvent_WithFallbackLogger(t *testing.T) {
	// Set defaultLogger to nil
	defaultLogger = nil

	// Mock fallbackLogger
	mockFallbackLogger := &mockFxEventLogger{}
	logger := &FxLogger{fallbackLogger: mockFallbackLogger}

	event := &fxevent.Invoked{}
	logger.LogEvent(event)

	assert.Equal(t, 1, mockFallbackLogger.logEventCallCount)
	assert.Equal(t, event, mockFallbackLogger.lastEvent)
}

// mockFxEventLogger is a mock implementation of fxevent.Logger for testing purposes.
type mockFxEventLogger struct {
	logEventCallCount int
	lastEvent         fxevent.Event
}

func (m *mockFxEventLogger) LogEvent(event fxevent.Event) {
	m.logEventCallCount++
	m.lastEvent = event
}

func (m *mockFxEventLogger) Sync() error                                   { return nil }
func (m *mockFxEventLogger) Debug(msg interface{}, keyvals ...interface{}) {}
func (m *mockFxEventLogger) Info(msg interface{}, keyvals ...interface{})  {}
func (m *mockFxEventLogger) Warn(msg interface{}, keyvals ...interface{})  {}
func (m *mockFxEventLogger) Error(msg interface{}, keyvals ...interface{}) {}
