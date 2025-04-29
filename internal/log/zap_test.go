package log

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewZapLogger(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
log:
  console:
    level: debug
  file:
    enabled: true
    level: error
    path: %s/log/test.log
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	conf, err := NewLogConfig(cfgFile)
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	testLogger := NewZapLogger(conf, os.Stderr)
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	msg := "test log entry"

	testLogger.Debug(msg)
	testLogger.Info(msg)
	testLogger.Warn(msg)
	testLogger.Error(msg)
}

func TestLogSetDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
log:
  console:
    level: debug
  file:
    enabled: true
    level: error
    path: %s/log/test.log
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	ConfigureDefault(cfgFile, os.Stderr)

	Debug("test message")
}

func TestLogParseLogLevelError(t *testing.T) {
	l := parseZapLogLevel("foobar")
	if l != zap.WarnLevel {
		t.Errorf("unexpected response from invalid parse log level, %v", l)
	}
}

func TestParseZapLogLevelDebugOverride(t *testing.T) {
	t.Setenv("DEBUG", "true")

	level := parseZapLogLevel("info")
	if level != zap.DebugLevel {
		t.Errorf("expected DebugLevel due to DebugEnvOverride, got %v", level)
	}
}

func TestParseZapLogLevelValidLevels(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"debug", zap.DebugLevel},
		{"info", zap.InfoLevel},
		{"warn", zap.WarnLevel},
		{"error", zap.ErrorLevel},
		{"dpanic", zap.DPanicLevel},
		{"panic", zap.PanicLevel},
		{"fatal", zap.FatalLevel},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			level := parseZapLogLevel(test.input)
			if level != test.expected {
				t.Errorf("expected %v, got %v", test.expected, level)
			}
		})
	}
}

func TestParseZapLogLevelInvalidLevel(t *testing.T) {
	level := parseZapLogLevel("invalid-level")
	if level != zap.WarnLevel {
		t.Errorf("expected WarnLevel for invalid input, got %v", level)
	}
}
