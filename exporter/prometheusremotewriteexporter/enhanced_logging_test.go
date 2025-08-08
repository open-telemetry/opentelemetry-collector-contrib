package prometheusremotewriteexporter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		statusCode     int
		retryExhausted bool
		expected       ErrorType
	}{
		{
			name:           "retry exhausted takes priority",
			err:            errors.New("server error"),
			statusCode:     503,
			retryExhausted: true,
			expected:       ErrorTypeRetryExhausted,
		},
		{
			name:           "client error 4xx",
			err:            errors.New("bad request"),
			statusCode:     400,
			retryExhausted: false,
			expected:       ErrorTypeClientError,
		},
		{
			name:           "server error 5xx",
			err:            errors.New("server error"),
			statusCode:     503,
			retryExhausted: false,
			expected:       ErrorTypeServerError,
		},
		{
			name:           "network timeout error",
			err:            errors.New("context deadline exceeded"),
			statusCode:     0,
			retryExhausted: false,
			expected:       ErrorTypeNetworkTimeout,
		},
		{
			name:           "context canceled error",
			err:            errors.New("context canceled"),
			statusCode:     0,
			retryExhausted: false,
			expected:       ErrorTypeNetworkTimeout,
		},
		{
			name:           "unknown error",
			err:            errors.New("unknown issue"),
			statusCode:     0,
			retryExhausted: false,
			expected:       ErrorTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.err, tt.statusCode, tt.retryExhausted)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogEnhancedError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		statusCode     int
		retryAttempt   int
		retryExhausted bool
		expectedLevel  zapcore.Level
		expectedType   string
	}{
		{
			name:           "server error with retries exhausted logs as error",
			err:            errors.New("server unavailable"),
			statusCode:     503,
			retryAttempt:   3,
			retryExhausted: true,
			expectedLevel:  zap.ErrorLevel,
			expectedType:   "retry_exhausted",
		},
		{
			name:           "client error on first attempt logs as warning",
			err:            errors.New("bad request"),
			statusCode:     400,
			retryAttempt:   0,
			retryExhausted: false,
			expectedLevel:  zap.WarnLevel,
			expectedType:   "client_error",
		},
		{
			name:           "server error on retry logs as warning",
			err:            errors.New("server error"),
			statusCode:     503,
			retryAttempt:   1,
			retryExhausted: false,
			expectedLevel:  zap.WarnLevel,
			expectedType:   "server_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, observed := observer.New(zap.WarnLevel)
			logger := zap.New(core)

			logEnhancedError(logger, tt.err, tt.statusCode, tt.retryAttempt, tt.retryExhausted)

			logs := observed.All()
			assert.Len(t, logs, 1)

			entry := logs[0]
			assert.Equal(t, tt.expectedLevel, entry.Level)
			assert.Equal(t, tt.expectedType, entry.ContextMap()["error_type"])
			assert.Equal(t, int64(tt.statusCode), entry.ContextMap()["status_code"])
			assert.Equal(t, int64(tt.retryAttempt), entry.ContextMap()["retry_attempt"])
			assert.Equal(t, tt.retryExhausted, entry.ContextMap()["retry_exhausted"])
		})
	}
}

func TestLogEnhancedSuccess(t *testing.T) {
	tests := []struct {
		name         string
		retryAttempt int
		expectLog    bool
	}{
		{
			name:         "no retries - no log",
			retryAttempt: 0,
			expectLog:    false,
		},
		{
			name:         "with retries - logs success",
			retryAttempt: 2,
			expectLog:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, observed := observer.New(zap.InfoLevel)
			logger := zap.New(core)

			logEnhancedSuccess(logger, tt.retryAttempt)

			logs := observed.All()
			if tt.expectLog {
				assert.Len(t, logs, 1)
				assert.Equal(t, zap.InfoLevel, logs[0].Level)
				assert.Equal(t, int64(tt.retryAttempt), logs[0].ContextMap()["retry_attempts"])
			} else {
				assert.Len(t, logs, 0)
			}
		})
	}
}

func TestDetermineMessage(t *testing.T) {
	tests := []struct {
		errorType   ErrorType
		statusCode  int
		expectedMsg string
	}{
		{ErrorTypeClientError, 400, "Check client-side configuration or authentication."},
		{ErrorTypeServerError, 503, "Server-side issue; retry or contact support."},
		{ErrorTypeNetworkTimeout, 0, "Network issue; check connectivity."},
		{ErrorTypeRetryExhausted, 503, "Max retries reached; data may be lost."},
		{ErrorTypeUnknown, 0, "Unknown error occurred."},
	}

	for _, tt := range tests {
		t.Run(string(tt.errorType), func(t *testing.T) {
			result := determineMessage(tt.errorType, tt.statusCode)
			assert.Equal(t, tt.expectedMsg, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"timeout occurred", "timeout", true},
		{"context deadline exceeded", "deadline", true},
		{"normal error", "timeout", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_contains_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
