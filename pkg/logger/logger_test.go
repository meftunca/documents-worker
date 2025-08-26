package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   bool
	}{
		{
			name: "valid json config",
			config: &Config{
				Level:      "info",
				Format:     "json",
				Output:     "stdout",
				TimeFormat: "2006-01-02T15:04:05Z07:00",
			},
			want: true,
		},
		{
			name: "valid console config",
			config: &Config{
				Level:      "debug",
				Format:     "console",
				Output:     "stderr",
				TimeFormat: "2006-01-02T15:04:05Z07:00",
			},
			want: true,
		},
		{
			name:   "nil config uses defaults",
			config: nil,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)
			assert.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestLoggerContext(t *testing.T) {
	logger, err := New(DefaultConfig())
	assert.NoError(t, err)

	t.Run("correlation ID context", func(t *testing.T) {
		ctx := WithCorrelationID(context.Background())
		correlationID := ctx.Value(CorrelationIDKey)
		assert.NotNil(t, correlationID)
		assert.IsType(t, "", correlationID)
	})

	t.Run("request ID context", func(t *testing.T) {
		requestID := "test-request-123"
		ctx := WithRequestID(context.Background(), requestID)
		assert.Equal(t, requestID, ctx.Value(RequestIDKey))
	})

	t.Run("user ID context", func(t *testing.T) {
		userID := "user-456"
		ctx := WithUserID(context.Background(), userID)
		assert.Equal(t, userID, ctx.Value(UserIDKey))
	})

	t.Run("logger from context", func(t *testing.T) {
		ctx := WithCorrelationID(context.Background())
		ctx = WithRequestID(ctx, "test-request")
		ctx = WithUserID(ctx, "test-user")

		contextLogger := logger.FromContext(ctx)
		assert.NotNil(t, contextLogger)
	})
}

func TestGlobalLogger(t *testing.T) {
	t.Run("get returns logger", func(t *testing.T) {
		logger := Get()
		assert.NotNil(t, logger)
	})

	t.Run("init and get", func(t *testing.T) {
		config := DefaultConfig()
		err := Init(config)
		assert.NoError(t, err)

		logger := Get()
		assert.NotNil(t, logger)
	})
}
