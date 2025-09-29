// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScraperError_Error(t *testing.T) {
	tests := []struct {
		name           string
		scraperError   ScraperError
		expectedOutput string
	}{
		{
			name: "error with all fields",
			scraperError: ScraperError{
				Operation: "session_count",
				Query:     "SELECT COUNT(*) FROM v$session WHERE status = 'ACTIVE'",
				Err:       errors.New("connection timeout"),
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				Context: map[string]interface{}{
					"instance": "PROD01",
					"host":     "db.example.com",
				},
			},
			expectedOutput: "oracle scraper error: connection timeout [operation=session_count, query=SELECT COUNT(*) FROM v$session WHERE status = 'ACTIVE', instance=PROD01, host=db.example.com]",
		},
		{
			name: "error with long query (truncated)",
			scraperError: ScraperError{
				Operation: "tablespace_metrics",
				Query:     "SELECT tablespace_name, file_name, bytes, maxbytes, autoextensible FROM dba_data_files UNION ALL SELECT tablespace_name, file_name, bytes, maxbytes, autoextensible FROM dba_temp_files ORDER BY tablespace_name",
				Err:       errors.New("ORA-00942: table or view does not exist"),
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			expectedOutput: "oracle scraper error: ORA-00942: table or view does not exist [operation=tablespace_metrics, query=SELECT tablespace_name, file_name, bytes, maxbytes, autoextensible FROM dba_data_files UNION ALL SEL...]",
		},
		{
			name: "error with multiline query (normalized)",
			scraperError: ScraperError{
				Operation: "core_metrics",
				Query: `SELECT 
					instance_name,
					startup_time,
					status
				FROM v$instance`,
				Err:       errors.New("permission denied"),
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			expectedOutput: "oracle scraper error: permission denied [operation=core_metrics, query=SELECT instance_name, startup_time, status FROM v$instance]",
		},
		{
			name: "error without operation and query",
			scraperError: ScraperError{
				Err:       errors.New("database connection failed"),
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				Context: map[string]interface{}{
					"retry_count": 3,
				},
			},
			expectedOutput: "oracle scraper error: database connection failed [retry_count=3]",
		},
		{
			name: "error without context",
			scraperError: ScraperError{
				Operation: "system_metrics",
				Err:       errors.New("timeout"),
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			expectedOutput: "oracle scraper error: timeout [operation=system_metrics]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scraperError.Error()
			assert.Contains(t, result, tt.expectedOutput)
		})
	}
}

func TestScraperError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	scraperErr := ScraperError{
		Operation: "test_operation",
		Err:       originalErr,
		Timestamp: time.Now(),
	}

	unwrapped := scraperErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestNewScraperError(t *testing.T) {
	originalErr := errors.New("test error")
	operation := "test_operation"
	context := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	scraperErr := NewScraperError(operation, originalErr, context)

	assert.Equal(t, operation, scraperErr.Operation)
	assert.Equal(t, originalErr, scraperErr.Err)
	assert.Equal(t, context, scraperErr.Context)
	assert.NotZero(t, scraperErr.Timestamp)
}

func TestNewQueryError(t *testing.T) {
	originalErr := errors.New("test error")
	operation := "test_operation"
	query := "SELECT * FROM test_table"
	context := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	scraperErr := NewQueryError(operation, query, originalErr, context)

	assert.Equal(t, operation, scraperErr.Operation)
	assert.Equal(t, query, scraperErr.Query)
	assert.Equal(t, originalErr, scraperErr.Err)
	assert.Equal(t, context, scraperErr.Context)
	assert.NotZero(t, scraperErr.Timestamp)
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "ORA-00028 - session killed (retryable)",
			err:         errors.New("ORA-00028: your session has been killed"),
			shouldRetry: true,
		},
		{
			name:        "ORA-00030 - user session ID does not exist (retryable)",
			err:         errors.New("ORA-00030: User session ID does not exist"),
			shouldRetry: true,
		},
		{
			name:        "ORA-01012 - not logged on (retryable)",
			err:         errors.New("ORA-01012: not logged on"),
			shouldRetry: true,
		},
		{
			name:        "ORA-01033 - Oracle initialization in progress (retryable)",
			err:         errors.New("ORA-01033: ORACLE initialization or shutdown in progress"),
			shouldRetry: true,
		},
		{
			name:        "ORA-01034 - Oracle not available (retryable)",
			err:         errors.New("ORA-01034: ORACLE not available"),
			shouldRetry: true,
		},
		{
			name:        "ORA-01089 - immediate shutdown in progress (retryable)",
			err:         errors.New("ORA-01089: immediate shutdown in progress - no operations are permitted"),
			shouldRetry: true,
		},
		{
			name:        "ORA-01090 - shutdown in progress (retryable)",
			err:         errors.New("ORA-01090: shutdown in progress - connection is not permitted"),
			shouldRetry: true,
		},
		{
			name:        "ORA-03113 - end-of-file on communication channel (retryable)",
			err:         errors.New("ORA-03113: end-of-file on communication channel"),
			shouldRetry: true,
		},
		{
			name:        "ORA-03114 - not connected to ORACLE (retryable)",
			err:         errors.New("ORA-03114: not connected to ORACLE"),
			shouldRetry: true,
		},
		{
			name:        "ORA-12170 - TNS connect timeout (retryable)",
			err:         errors.New("ORA-12170: TNS:Connect timeout occurred"),
			shouldRetry: true,
		},
		{
			name:        "ORA-12571 - TNS packet writer failure (retryable)",
			err:         errors.New("ORA-12571: TNS:packet writer failure"),
			shouldRetry: true,
		},
		{
			name:        "ORA-00942 - table or view does not exist (not retryable)",
			err:         errors.New("ORA-00942: table or view does not exist"),
			shouldRetry: false,
		},
		{
			name:        "ORA-00904 - invalid identifier (not retryable)",
			err:         errors.New("ORA-00904: invalid identifier"),
			shouldRetry: false,
		},
		{
			name:        "ORA-01031 - insufficient privileges (not retryable)",
			err:         errors.New("ORA-01031: insufficient privileges"),
			shouldRetry: false,
		},
		{
			name:        "Generic network error (retryable)",
			err:         errors.New("connection refused"),
			shouldRetry: true,
		},
		{
			name:        "Connection reset error (retryable)",
			err:         errors.New("connection reset by peer"),
			shouldRetry: true,
		},
		{
			name:        "Context timeout error (retryable)",
			err:         errors.New("context deadline exceeded"),
			shouldRetry: true,
		},
		{
			name:        "Unknown error (not retryable)",
			err:         errors.New("some unknown error"),
			shouldRetry: false,
		},
		{
			name: "ScraperError with retryable underlying error",
			err: &ScraperError{
				Operation: "test",
				Err:       errors.New("ORA-03113: end-of-file on communication channel"),
			},
			shouldRetry: true,
		},
		{
			name: "ScraperError with non-retryable underlying error",
			err: &ScraperError{
				Operation: "test",
				Err:       errors.New("ORA-00942: table or view does not exist"),
			},
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.shouldRetry, result, "Expected IsRetryableError to return %v for error: %v", tt.shouldRetry, tt.err)
		})
	}
}

func TestIsRetryableError_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "ORA error in lowercase",
			err:         errors.New("ora-03113: end-of-file on communication channel"),
			shouldRetry: true,
		},
		{
			name:        "Mixed case ORA error",
			err:         errors.New("ORA-03113: End-Of-File On Communication Channel"),
			shouldRetry: true,
		},
		{
			name:        "Network error in uppercase",
			err:         errors.New("CONNECTION REFUSED"),
			shouldRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.shouldRetry, result)
		})
	}
}

func TestErrorContext(t *testing.T) {
	context := map[string]interface{}{
		"instance_name": "PROD01",
		"host":          "db.example.com",
		"port":          1521,
		"retry_count":   3,
		"last_success":  time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
	}

	scraperErr := ScraperError{
		Operation: "metrics_collection",
		Query:     "SELECT COUNT(*) FROM v$session",
		Err:       errors.New("timeout"),
		Timestamp: time.Now(),
		Context:   context,
	}

	errorStr := scraperErr.Error()

	// Verify that all context fields are included in the error string
	assert.Contains(t, errorStr, "instance_name=PROD01")
	assert.Contains(t, errorStr, "host=db.example.com")
	assert.Contains(t, errorStr, "port=1521")
	assert.Contains(t, errorStr, "retry_count=3")
}
