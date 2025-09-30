// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"fmt"
	"strings"
	"time"
)

// ScraperError represents a structured error from the Oracle receiver scrapers
type ScraperError struct {
	Operation string
	Query     string
	Err       error
	Timestamp time.Time
	Context   map[string]interface{}
}

// Error implements the error interface
func (e *ScraperError) Error() string {
	var parts []string

	if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("operation=%s", e.Operation))
	}

	if e.Query != "" {
		// Truncate long queries for readability
		query := e.Query
		if len(query) > 100 {
			query = query[:100] + "..."
		}
		// Remove extra whitespace and newlines
		query = strings.Join(strings.Fields(query), " ")
		parts = append(parts, fmt.Sprintf("query=%s", query))
	}

	if len(e.Context) > 0 {
		for k, v := range e.Context {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}

	contextStr := ""
	if len(parts) > 0 {
		contextStr = fmt.Sprintf(" [%s]", strings.Join(parts, ", "))
	}

	return fmt.Sprintf("oracle scraper error: %v%s", e.Err, contextStr)
}

// Unwrap returns the underlying error
func (e *ScraperError) Unwrap() error {
	return e.Err
}

// NewScraperError creates a new ScraperError with context
func NewScraperError(operation string, err error, context map[string]interface{}) *ScraperError {
	return &ScraperError{
		Operation: operation,
		Err:       err,
		Timestamp: time.Now(),
		Context:   context,
	}
}

// NewQueryError creates a new ScraperError for query-related errors
func NewQueryError(operation, query string, err error, context map[string]interface{}) *ScraperError {
	return &ScraperError{
		Operation: operation,
		Query:     query,
		Err:       err,
		Timestamp: time.Now(),
		Context:   context,
	}
}

// IsRetryableError checks if an error is retryable based on Oracle error codes
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Oracle error codes that typically indicate retryable conditions
	retryablePatterns := []string{
		"ora-00028", // Session killed
		"ora-00030", // User session ID does not exist
		"ora-00060", // Deadlock detected while waiting for resource
		"ora-00600", // Internal error code
		"ora-01012", // Not logged on
		"ora-01017", // Invalid username/password; logon denied
		"ora-01033", // Oracle initialization or shutdown in progress
		"ora-01034", // Oracle not available
		"ora-01089", // Immediate shutdown in progress
		"ora-01090", // Shutdown in progress
		"ora-01092", // Oracle instance terminated
		"ora-02068", // Following severe error from
		"ora-03113", // End-of-file on communication channel
		"ora-03114", // Not connected to Oracle
		"ora-03135", // Connection lost contact
		"ora-12170", // TNS:Connect timeout occurred
		"ora-12171", // TNS:could not resolve connect identifier
		"ora-12203", // TNS:unable to connect to destination
		"ora-12500", // TNS:listener failed to start a dedicated server process
		"ora-12505", // TNS:listener does not currently know of SID
		"ora-12514", // TNS:listener does not currently know of service
		"ora-12528", // TNS:listener: all appropriate instances are blocking new connections
		"ora-12541", // TNS:no listener
		"ora-12542", // TNS:address already in use
		"ora-12543", // TNS:destination host unreachable
		"ora-12545", // Connect failed because target host or object does not exist
		"ora-12547", // TNS:lost contact
		"ora-12560", // TNS:protocol adapter error
		"ora-12571", // TNS:packet writer failure
		"ora-25401", // Cannot continue fetches
		// Network and connection issues
		"connection refused",
		"connection reset",
		"connection timed out",
		"network is unreachable",
		"no such host",
		"temporary failure",
		"i/o timeout",
		"broken pipe",
		"connection reset by peer",
		// Context errors
		"context deadline exceeded",
		"context canceled",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// IsPermanentError checks if an error is permanent and should not be retried
func IsPermanentError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Oracle error codes that indicate permanent failures
	permanentPatterns := []string{
		"ora-00900", // Invalid SQL statement
		"ora-00904", // Invalid identifier
		"ora-00911", // Invalid character
		"ora-00917", // Missing comma
		"ora-00923", // FROM keyword not found where expected
		"ora-00936", // Missing expression
		"ora-00942", // Table or view does not exist
		"ora-00955", // Name is already used by an existing object
		"ora-01017", // Invalid username/password (after auth failures)
		"ora-01031", // Insufficient privileges
		"ora-01400", // Cannot insert NULL
		"ora-01401", // Inserted value too large for column
		"ora-01403", // No data found
		"ora-01422", // Exact fetch returns more than requested number of rows
		"ora-01427", // Single-row subquery returns more than one row
		"ora-01476", // Divisor is equal to zero
		"ora-01722", // Invalid number
		"ora-01741", // Illegal zero-length identifier
		"ora-01747", // Invalid user.table.column, table.column, or column specification
		"ora-01756", // Quoted string not properly terminated
		"ora-02292", // Integrity constraint violated - child record found
		"ora-02291", // Integrity constraint violated - parent key not found
		"ora-12899", // Value too large for column
		// SQL syntax and data errors
		"sql: syntax error",
		"invalid syntax",
		"column does not exist",
		"relation does not exist",
		"permission denied",
	}

	for _, pattern := range permanentPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// FormatQueryForLogging formats a SQL query for logging by removing extra whitespace
func FormatQueryForLogging(query string) string {
	return strings.Join(strings.Fields(query), " ")
}
