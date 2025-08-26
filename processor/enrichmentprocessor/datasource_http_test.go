// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestParseJSONData(t *testing.T) {
	testCases := []struct {
		name        string
		jsonData    string
		expectError bool
		expectedLen int
	}{
		{
			name: "valid_json_array",
			jsonData: `[
				{"service_name": "user-service", "team": "platform", "environment": "prod"},
				{"service_name": "payment-service", "team": "payments", "environment": "stage"}
			]`,
			expectError: false,
			expectedLen: 2,
		},
		{
			name:        "empty_json_array",
			jsonData:    `[]`,
			expectError: true,
		},
		{
			name:        "invalid_json",
			jsonData:    `{invalid json}`,
			expectError: true,
		},
		{
			name:        "json_object_not_array",
			jsonData:    `{"service_name": "test"}`,
			expectError: true,
		},
		{
			name:        "array_with_non_object",
			jsonData:    `["string", {"service_name": "test"}]`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, index, err := ParseData([]byte(tc.jsonData), "json")

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, data)
				assert.Nil(t, index)
			} else {
				require.NoError(t, err)
				assert.Len(t, data, tc.expectedLen)
				assert.NotNil(t, index)

				if tc.expectedLen > 0 {
					// Verify structure
					assert.Contains(t, index, "service_name")
					assert.Contains(t, index, "team")
					assert.Contains(t, index, "environment")

					// Verify first row data
					assert.Equal(t, "user-service", data[0][index["service_name"]])
					assert.Equal(t, "platform", data[0][index["team"]])
					assert.Equal(t, "prod", data[0][index["environment"]])
				}
			}
		})
	}
}

func TestParseCSVData(t *testing.T) {
	testCases := []struct {
		name        string
		csvData     string
		expectError bool
		expectedLen int
	}{
		{
			name: "valid_csv",
			csvData: `service_name,team,environment
user-service,platform,prod
payment-service,payments,stage`,
			expectError: false,
			expectedLen: 2,
		},
		{
			name:        "csv_with_header_only",
			csvData:     `service_name,team,environment`,
			expectError: true,
		},
		{
			name:        "invalid_csv",
			csvData:     "service_name,team\n\"unclosed,quote",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, index, err := ParseData([]byte(tc.csvData), "csv")

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, data, tc.expectedLen)
				assert.NotNil(t, index)

				if tc.expectedLen > 0 {
					// Verify header structure
					assert.Contains(t, index, "service_name")
					assert.Contains(t, index, "team")
					assert.Contains(t, index, "environment")

					// Verify first row data
					assert.Equal(t, "user-service", data[0][index["service_name"]])
					assert.Equal(t, "platform", data[0][index["team"]])
					assert.Equal(t, "prod", data[0][index["environment"]])
				}
			}
		})
	}
}

func TestHTTPDataSource_FormatDetection(t *testing.T) {
	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	testCases := []struct {
		name           string
		contentType    string
		configFormat   string
		expectedFormat string
	}{
		{
			name:           "csv_content_type",
			contentType:    "text/csv",
			configFormat:   "",
			expectedFormat: "csv",
		},
		{
			name:           "json_content_type",
			contentType:    "application/json",
			configFormat:   "",
			expectedFormat: "json",
		},
		{
			name:           "explicit_format_override",
			contentType:    "text/plain",
			configFormat:   "json",
			expectedFormat: "json",
		},
		{
			name:           "unknown_content_type",
			contentType:    "text/plain",
			configFormat:   "",
			expectedFormat: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := HTTPDataSourceConfig{
				URL:    "http://test.com",
				Format: tc.configFormat,
			}

			ds := NewHTTPDataSource(config, logger, indexFields)
			format := ds.determineFormat(tc.contentType)

			assert.Equal(t, tc.expectedFormat, format)
		})
	}
}

func TestHTTPDataSource_Integration(t *testing.T) {
	// Create test server with JSON response
	jsonResponse := `[
		{"service_name": "user-service", "team": "platform", "environment": "prod"},
		{"service_name": "payment-service", "team": "payments", "environment": "stage"}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonResponse))
	}))
	defer server.Close()

	config := HTTPDataSourceConfig{
		URL:             server.URL,
		Timeout:         time.Second * 5,
		RefreshInterval: time.Minute,
	}

	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	ds := NewHTTPDataSource(config, logger, indexFields)

	// Test refresh (simulates Start behavior without goroutine)
	err := ds.refresh(t.Context())
	require.NoError(t, err)

	// Test lookup
	row, index, err := ds.Lookup("service_name", "user-service")
	require.NoError(t, err)

	assert.Equal(t, "user-service", row[index["service_name"]])
	assert.Equal(t, "platform", row[index["team"]])
	assert.Equal(t, "prod", row[index["environment"]])

	// Test non-existent lookup
	_, _, err = ds.Lookup("service_name", "non-existent")
	assert.Error(t, err)
}

func TestHTTPDataSource_ErrorHandling(t *testing.T) {
	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	t.Run("http_error_status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:     server.URL,
			Timeout: time.Second,
		}

		ds := NewHTTPDataSource(config, logger, indexFields)
		err := ds.refresh(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP request failed with status: 500")
	})

	t.Run("invalid_json_response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{invalid json}"))
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:     server.URL,
			Timeout: time.Second,
		}

		ds := NewHTTPDataSource(config, logger, indexFields)
		err := ds.refresh(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse json data")
	})

	t.Run("unknown_content_type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("some data"))
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:     server.URL,
			Timeout: time.Second,
		}

		ds := NewHTTPDataSource(config, logger, indexFields)
		err := ds.refresh(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to determine data format")
	})
}
