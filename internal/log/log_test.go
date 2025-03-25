package log

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger(t *testing.T) {
	// Test case 1: Debug level
	logger, err := NewLogger(true)
	assert.NoError(t, err)
	assert.NotNil(t, logger)
	defer func() {
		err := logger.Sync()
		if err != nil {
			t.Logf("Warning: logger.Sync() error: %v", err)
		}
	}()

	// Test case 2: Info level
	logger, err = NewLogger(false)
	assert.NoError(t, err)
	assert.NotNil(t, logger)
	defer func() {
		err := logger.Sync()
		if err != nil {
			t.Logf("Warning: logger.Sync() error: %v", err)
		}
	}()
}

func TestLogOutputs(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create a custom encoder config that writes to the buffer
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zap.DebugLevel)

	testLogger := zap.New(core)

	// Test debug level
	testLogger.Debug("debug message", zap.String("key", "value"))
	assert.Contains(t, buf.String(), "debug message")
	assert.Contains(t, buf.String(), "key")
	assert.Contains(t, buf.String(), "value")

	// Reset buffer
	buf.Reset()

	// Test info level
	testLogger.Info("info message", zap.Int("count", 42))
	assert.Contains(t, buf.String(), "info message")
	assert.Contains(t, buf.String(), "count")
	assert.Contains(t, buf.String(), "42")
}
