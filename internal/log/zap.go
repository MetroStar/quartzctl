package log

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger is a logger implementation using Uber's Zap library.
type ZapLogger struct {
	zap *zap.SugaredLogger
	fx  fxevent.Logger
	cfg LogConfig
}

// NewZapLogger creates a new instance of ZapLogger with the provided configuration and writer.
func NewZapLogger(cfg LogConfig, w io.Writer) AppLogger {
	// Create a core for console output
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(zapcore.AddSync(w)),
		parseZapLogLevel(cfg.Log.Console.Level),
	)

	if !cfg.Log.File.Enabled {
		l := zap.New(consoleCore)
		return &ZapLogger{
			zap: l.Sugar(),
			fx:  &fxevent.ZapLogger{Logger: l},
			cfg: cfg,
		}
	}

	path, _ := filepath.Abs(cfg.Log.File.Path)
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0740) //nolint:errcheck

	now := time.Now()

	path = strings.ReplaceAll(path, "$name", cfg.Name)
	path = strings.ReplaceAll(path, "$date", now.Format("2006-01-02"))

	logFile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640) // #nosec G304
	if err != nil {
		panic(err)
	}

	// Create a core for file output
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(zapcore.AddSync(logFile)),
		parseZapLogLevel(cfg.Log.File.Level),
	)

	// Combine the cores using NewTee
	combinedCore := zapcore.NewTee(consoleCore, fileCore)

	// Create the logger with the combined core
	logger := zap.New(combinedCore)
	return &ZapLogger{
		zap: logger.Sugar(),
		fx:  &fxevent.ZapLogger{Logger: logger},
		cfg: cfg,
	}
}

// Sync flushes any buffered log entries.
func (l *ZapLogger) Sync() error {
	return l.zap.Sync()
}

// Debug prints a debug message with optional key-value pairs.
func (l *ZapLogger) Debug(msg interface{}, keyvals ...interface{}) {
	l.zap.Debugw(msg.(string), keyvals...)
}

// Info prints an informational message with optional key-value pairs.
func (l *ZapLogger) Info(msg interface{}, keyvals ...interface{}) {
	l.zap.Infow(msg.(string), keyvals...)
}

// Warn prints a warning message with optional key-value pairs.
func (l *ZapLogger) Warn(msg interface{}, keyvals ...interface{}) {
	l.zap.Warnw(msg.(string), keyvals...)
}

// Error prints an error message with optional key-value pairs.
func (l *ZapLogger) Error(msg interface{}, keyvals ...interface{}) {
	l.zap.Errorw(msg.(string), keyvals...)
}

// LogEvent logs an fxevent.Event using the underlying fxevent.Logger.
func (l *ZapLogger) LogEvent(event fxevent.Event) {
	l.fx.LogEvent(event)
}

// parseZapLogLevel parses a string into a zapcore.Level. If DebugEnvOverride is enabled, it always returns DebugLevel.
func parseZapLogLevel(l string) zapcore.Level {
	if DebugEnvOverride() {
		return zap.DebugLevel
	}

	p, err := zapcore.ParseLevel(strings.ToLower(l))
	if err != nil {
		return zap.WarnLevel
	}
	return p
}
