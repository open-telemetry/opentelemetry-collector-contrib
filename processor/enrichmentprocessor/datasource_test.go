// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewDataSources(t *testing.T) {
	logger := zap.NewNop()
	indexFields := []string{"name", "environment"}

	t.Run("http_data_source", func(t *testing.T) {
		config := HTTPDataSourceConfig{
			URL:             "http://example.com/api",
			Timeout:         30 * time.Second,
			RefreshInterval: 5 * time.Minute,
			Headers: map[string]string{
				"Authorization": "Bearer token",
			},
		}

		dataSource := NewHTTPDataSource(config, logger, indexFields)

		assert.NotNil(t, dataSource)
		assert.Equal(t, config, dataSource.config)
		assert.NotNil(t, dataSource.client)
		assert.Equal(t, config.Timeout, dataSource.client.Timeout)
		assert.Equal(t, indexFields, dataSource.indexField)
	})

	t.Run("file_data_source", func(t *testing.T) {
		config := FileDataSourceConfig{
			Path:            "/tmp/test.json",
			Format:          "json",
			RefreshInterval: 5 * time.Minute,
		}

		dataSource := NewFileDataSource(config, logger, indexFields)

		assert.NotNil(t, dataSource)
		assert.Equal(t, config, dataSource.config)
		assert.Equal(t, indexFields, dataSource.indexField)
	})
}

func TestDataSource_Lifecycle(t *testing.T) {
	t.Run("http_data_source_integration", func(t *testing.T) {
		// Create test server with JSON data
		testData := []map[string]interface{}{
			{
				"name":        "user-service",
				"owner":       "platform-team",
				"environment": "production",
			},
			{
				"name":        "payment-service",
				"owner":       "payments-team",
				"environment": "production",
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(testData)
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:             server.URL,
			Timeout:         5 * time.Second,
			RefreshInterval: 1 * time.Minute,
		}

		logger := zap.NewNop()
		indexFields := []string{"name", "environment"}
		dataSource := NewHTTPDataSource(config, logger, indexFields)

		// Start the data source
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := dataSource.Start(ctx)
		require.NoError(t, err)

		// Wait a bit for initial load
		time.Sleep(100 * time.Millisecond)

		// Verify data source started successfully (no specific assertions needed for internal state)
		// The fact that Start() succeeded and we can call Stop() shows the lifecycle works

		// Stop the data source
		err = dataSource.Stop()
		assert.NoError(t, err)
	})

	t.Run("file_data_source_json", func(t *testing.T) {
		// Create temp file with JSON data
		testData := []map[string]interface{}{
			{
				"hostname":    "web-01",
				"environment": "production",
				"region":      "us-east-1",
			},
			{
				"hostname":    "db-01",
				"environment": "production",
				"region":      "us-west-2",
			},
		}

		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.json")

		jsonData, err := json.Marshal(testData)
		require.NoError(t, err)
		err = os.WriteFile(testFile, jsonData, 0o644)
		require.NoError(t, err)

		config := FileDataSourceConfig{
			Path:            testFile,
			Format:          "json",
			RefreshInterval: 1 * time.Minute,
		}

		logger := zap.NewNop()
		indexFields := []string{"hostname", "environment"}
		dataSource := NewFileDataSource(config, logger, indexFields)

		// Start the data source
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = dataSource.Start(ctx)
		require.NoError(t, err)

		// Verify data source started successfully (no specific assertions needed for internal state)
		// The fact that Start() succeeded and we can call Stop() shows the lifecycle works

		// Stop the data source
		err = dataSource.Stop()
		assert.NoError(t, err)
	})

	t.Run("file_data_source_csv", func(t *testing.T) {
		// Create temp file with CSV data
		csvData := `hostname,environment,region
web-01,production,us-east-1
db-01,production,us-west-2
cache-01,staging,us-east-1`

		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.csv")
		err := os.WriteFile(testFile, []byte(csvData), 0o644)
		require.NoError(t, err)

		config := FileDataSourceConfig{
			Path:            testFile,
			Format:          "csv",
			RefreshInterval: 1 * time.Minute,
		}

		logger := zap.NewNop()
		indexFields := []string{"hostname", "environment"}
		dataSource := NewFileDataSource(config, logger, indexFields)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = dataSource.Start(ctx)
		require.NoError(t, err)

		// Verify data source started successfully (no specific assertions needed for internal state)
		// The fact that Start() succeeded and we can call Stop() shows the lifecycle works

		err = dataSource.Stop()
		assert.NoError(t, err)
	})
}

func TestHTTPDataSource_ErrorHandling(t *testing.T) {
	// Test with server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	config := HTTPDataSourceConfig{
		URL:             server.URL,
		Timeout:         1 * time.Second,
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	indexFields := []string{"name"}
	dataSource := NewHTTPDataSource(config, logger, indexFields)

	ctx := context.Background()
	err := dataSource.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request failed with status: 500")
}

func TestParseJSON(t *testing.T) {
	// Test valid JSON array of objects
	jsonData := `[
		{
			"service_name": "user-service",
			"owner_team": "platform-team",
			"environment": "production"
		},
		{
			"service_name": "payment-service", 
			"owner_team": "payments-team",
			"environment": "staging"
		}
	]`

	rows, headerIndex, err := parseJSON([]byte(jsonData))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rows))
	assert.Equal(t, 3, len(headerIndex))

	// Check header mapping
	assert.Contains(t, headerIndex, "service_name")
	assert.Contains(t, headerIndex, "owner_team")
	assert.Contains(t, headerIndex, "environment")

	// Check data is correctly parsed
	serviceNameIdx := headerIndex["service_name"]
	ownerIdx := headerIndex["owner_team"]
	envIdx := headerIndex["environment"]

	assert.Equal(t, "user-service", rows[0][serviceNameIdx])
	assert.Equal(t, "platform-team", rows[0][ownerIdx])
	assert.Equal(t, "production", rows[0][envIdx])

	assert.Equal(t, "payment-service", rows[1][serviceNameIdx])
	assert.Equal(t, "payments-team", rows[1][ownerIdx])
	assert.Equal(t, "staging", rows[1][envIdx])
}

func TestParseJSON_InvalidFormats(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantErr  bool
	}{
		{
			name:     "invalid json",
			jsonData: "invalid json",
			wantErr:  true,
		},
		{
			name:     "not an array",
			jsonData: `{"key": "value"}`,
			wantErr:  true,
		},
		{
			name:     "empty array",
			jsonData: `[]`,
			wantErr:  true,
		},
		{
			name:     "array with non-object",
			jsonData: `["string", "another"]`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseJSON([]byte(tt.jsonData))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
