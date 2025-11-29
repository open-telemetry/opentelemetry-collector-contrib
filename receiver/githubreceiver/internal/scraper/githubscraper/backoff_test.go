// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

// mockRateLimit implements the rateLimitInfo interface for testing
type mockRateLimit struct {
	limit     int
	cost      int
	remaining int
	resetAt   time.Time
}

func (m *mockRateLimit) GetLimit() int {
	return m.limit
}

func (m *mockRateLimit) GetCost() int {
	return m.cost
}

func (m *mockRateLimit) GetRemaining() int {
	return m.remaining
}

func (m *mockRateLimit) GetResetAt() time.Time {
	return m.resetAt
}

// mockResponse implements the response structure for testing
type mockResponse struct {
	rateLimit *mockRateLimit
	data      string
}

func (m *mockResponse) GetRateLimit() rateLimitInfo {
	return m.rateLimit
}

func TestIsRetriableError_GenqlientHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"403 Forbidden is retriable", 403, true},
		{"429 Too Many Requests is retriable", 429, true},
		{"502 Bad Gateway is retriable", 502, true},
		{"503 Service Unavailable is retriable", 503, true},
		{"504 Gateway Timeout is retriable", 504, true},
		{"400 Bad Request is not retriable", 400, false},
		{"401 Unauthorized is not retriable", 401, false},
		{"404 Not Found is not retriable", 404, false},
		{"500 Internal Server Error is not retriable", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &graphql.HTTPError{
				StatusCode: tt.statusCode,
			}
			assert.Equal(t, tt.want, isRetriableError(err))
		})
	}
}

func TestIsRetriableError_GitHubErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"403 Forbidden is retriable", 403, true},
		{"429 Too Many Requests is retriable", 429, true},
		{"502 Bad Gateway is retriable", 502, true},
		{"503 Service Unavailable is retriable", 503, true},
		{"504 Gateway Timeout is retriable", 504, true},
		{"400 Bad Request is not retriable", 400, false},
		{"401 Unauthorized is not retriable", 401, false},
		{"404 Not Found is not retriable", 404, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: tt.statusCode,
				},
				Message: "test error",
			}
			assert.Equal(t, tt.want, isRetriableError(err))
		})
	}
}

func TestIsRetriableError_GitHubErrorResponseNilResponse(t *testing.T) {
	// ErrorResponse with nil Response field should not panic
	err := &github.ErrorResponse{
		Response: nil,
		Message:  "test error",
	}
	assert.False(t, isRetriableError(err))
}

func TestIsRetriableError_GraphQLLevelErrors(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{
			name:   "API rate limit exceeded",
			errMsg: "GraphQL error: API rate limit exceeded for user",
			want:   true,
		},
		{
			name:   "secondary rate limit",
			errMsg: "You have exceeded a secondary rate limit",
			want:   true,
		},
		{
			name:   "generic error",
			errMsg: "something went wrong",
			want:   false,
		},
		{
			name:   "database error",
			errMsg: "database connection failed",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			assert.Equal(t, tt.want, isRetriableError(err))
		})
	}
}

func TestIsRetriableError_NilError(t *testing.T) {
	assert.False(t, isRetriableError(nil))
}

func TestIsRetriableStatusCode(t *testing.T) {
	tests := []struct {
		statusCode int
		want       bool
	}{
		{403, true},
		{429, true},
		{502, true},
		{503, true},
		{504, true},
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{404, false},
		{500, false},
		{501, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			assert.Equal(t, tt.want, isRetriableStatusCode(tt.statusCode))
		})
	}
}

func TestGraphqlCallWithRetry_ProactiveRateLimit(t *testing.T) {
	logger := zap.NewNop()
	telemetry := componenttest.NewNopTelemetrySettings()

	callCount := 0
	resetTime := time.Now().Add(100 * time.Millisecond) // Short wait for test

	apiCall := func() (*mockResponse, error) {
		callCount++

		// First call: low remaining quota, should trigger proactive wait
		if callCount == 1 {
			return &mockResponse{
				rateLimit: &mockRateLimit{
					limit:     5000,
					cost:      10,
					remaining: 5, // Cost >= Remaining, should wait
					resetAt:   resetTime,
				},
				data: "first response",
			}, nil
		}

		// Second call: after reset, should succeed
		return &mockResponse{
			rateLimit: &mockRateLimit{
				limit:     5000,
				cost:      10,
				remaining: 5000,
				resetAt:   time.Now().Add(1 * time.Hour),
			},
			data: "second response",
		}, nil
	}

	result, err := graphqlCallWithRetry(
		t.Context(),
		logger,
		telemetry,
		5*time.Second, // Allow enough time for retry
		apiCall,
	)

	// Should eventually succeed (after retry logic)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, callCount, "Should be called twice: once triggering wait, once succeeding")
	assert.Equal(t, "second response", result.data)
}

func TestGraphqlCallWithRetry_SuccessOnFirstTry(t *testing.T) {
	logger := zap.NewNop()
	telemetry := componenttest.NewNopTelemetrySettings()

	callCount := 0
	apiCall := func() (*mockResponse, error) {
		callCount++
		return &mockResponse{
			rateLimit: &mockRateLimit{
				limit:     5000,
				cost:      4,
				remaining: 4996,
				resetAt:   time.Now().Add(1 * time.Hour),
			},
			data: "success",
		}, nil
	}

	result, err := graphqlCallWithRetry(
		t.Context(),
		logger,
		telemetry,
		5*time.Minute,
		apiCall,
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, callCount, "Should succeed on first try")
	assert.Equal(t, "success", result.data)
}

func TestGraphqlCallWithRetry_NonRetriableError(t *testing.T) {
	logger := zap.NewNop()
	telemetry := componenttest.NewNopTelemetrySettings()

	callCount := 0
	apiCall := func() (*mockResponse, error) {
		callCount++
		// Return non-retriable error (404)
		return nil, &graphql.HTTPError{
			StatusCode: 404,
		}
	}

	result, err := graphqlCallWithRetry(
		t.Context(),
		logger,
		telemetry,
		5*time.Minute,
		apiCall,
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, callCount, "Should not retry non-retriable errors")
}

func TestGraphqlCallWithRetry_RetriableErrorEventuallySucceeds(t *testing.T) {
	logger := zap.NewNop()
	telemetry := componenttest.NewNopTelemetrySettings()

	callCount := 0
	apiCall := func() (*mockResponse, error) {
		callCount++

		// Fail first 2 times, then succeed
		if callCount < 3 {
			return nil, &graphql.HTTPError{
				StatusCode: 503,
			}
		}

		return &mockResponse{
			rateLimit: &mockRateLimit{
				limit:     5000,
				cost:      4,
				remaining: 4996,
				resetAt:   time.Now().Add(1 * time.Hour),
			},
			data: "success after retries",
		}, nil
	}

	result, err := graphqlCallWithRetry(
		t.Context(),
		logger,
		telemetry,
		30*time.Second,
		apiCall,
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, callCount, "Should retry and eventually succeed")
	assert.Equal(t, "success after retries", result.data)
}

func TestGraphqlCallWithRetry_NoRateLimitData(t *testing.T) {
	logger := zap.NewNop()
	telemetry := componenttest.NewNopTelemetrySettings()

	// Response without rate limit (like REST API responses)
	type simpleResponse struct {
		data string
	}

	callCount := 0
	apiCall := func() (*simpleResponse, error) {
		callCount++
		return &simpleResponse{
			data: "success without rate limit",
		}, nil
	}

	result, err := graphqlCallWithRetry(
		t.Context(),
		logger,
		telemetry,
		5*time.Minute,
		apiCall,
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, callCount, "Should succeed even without rate limit data")
	assert.Equal(t, "success without rate limit", result.data)
}
