package prometheusremotewriteexporter

import (
	"go.uber.org/zap"
)

// ErrorType represents different categories of export failures
type ErrorType string

const (
	ErrorTypeClientError    ErrorType = "client_error"    // 4xx HTTP status codes
	ErrorTypeServerError    ErrorType = "server_error"    // 5xx HTTP status codes
	ErrorTypeNetworkTimeout ErrorType = "network_timeout" // Network issues, timeouts
	ErrorTypeRetryExhausted ErrorType = "retry_exhausted" // Max retries reached
	ErrorTypeUnknown        ErrorType = "unknown"         // Other errors
)

// classifyError determines the error type based on the error and HTTP status code
func classifyError(err error, statusCode int, retryExhausted bool) ErrorType {
	if retryExhausted {
		return ErrorTypeRetryExhausted
	}

	if statusCode >= 400 && statusCode < 500 {
		return ErrorTypeClientError
	}

	if statusCode >= 500 && statusCode < 600 {
		return ErrorTypeServerError
	}

	if err != nil {
		errStr := err.Error()
		if contains(errStr, "timeout") || contains(errStr, "deadline exceeded") {
			return ErrorTypeNetworkTimeout
		}
		if contains(errStr, "context canceled") || contains(errStr, "context cancelled") {
			return ErrorTypeNetworkTimeout
		}
	}

	return ErrorTypeUnknown
}

// logEnhancedError logs detailed error information
func logEnhancedError(logger *zap.Logger, err error, statusCode int, retryAttempt int, retryExhausted bool) {
	// Handle nil logger gracefully (e.g., in tests)
	if logger == nil {
		return
	}

	errorType := classifyError(err, statusCode, retryExhausted)

	fields := []zap.Field{
		zap.String("error_type", string(errorType)),
		zap.Int("status_code", statusCode),
		zap.Int("retry_attempt", retryAttempt),
		zap.Bool("retry_exhausted", retryExhausted),
		zap.Error(err),
	}

	message := determineMessage(errorType, statusCode)

	if retryExhausted {
		logger.Error(message, fields...)
	} else {
		logger.Warn(message, fields...)
	}
}

// logEnhancedSuccess logs successful exports with retry context
func logEnhancedSuccess(logger *zap.Logger, retryAttempt int) {
	// Handle nil logger gracefully (e.g., in tests)
	if logger == nil {
		return
	}

	if retryAttempt > 0 {
		logger.Info("Remote write succeeded after retries",
			zap.Int("retry_attempts", retryAttempt),
		)
	}
}

func determineMessage(errorType ErrorType, statusCode int) string {
	switch errorType {
	case ErrorTypeClientError:
		return "Check client-side configuration or authentication."
	case ErrorTypeServerError:
		return "Server-side issue; retry or contact support."
	case ErrorTypeNetworkTimeout:
		return "Network issue; check connectivity."
	case ErrorTypeRetryExhausted:
		return "Max retries reached; data may be lost."
	default:
		return "Unknown error occurred."
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
