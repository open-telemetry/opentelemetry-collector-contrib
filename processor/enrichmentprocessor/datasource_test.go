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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewHTTPDataSource(t *testing.T) {
	config := HTTPDataSourceConfig{
		URL:             "http://example.com/api",
		Timeout:         30 * time.Second,
		RefreshInterval: 5 * time.Minute,
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
	}

	logger := zap.NewNop()
	dataSource := NewHTTPDataSource(config, logger)

	assert.NotNil(t, dataSource)
	assert.Equal(t, config, dataSource.config)
	assert.NotNil(t, dataSource.client)
	assert.Equal(t, config.Timeout, dataSource.client.Timeout)
	assert.Empty(t, dataSource.data)
}

func TestHTTPDataSource_Integration(t *testing.T) {
	// Create test server
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
		// Check headers
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testData)
	}))
	defer server.Close()

	config := HTTPDataSourceConfig{
		URL:             server.URL,
		Timeout:         5 * time.Second,
		RefreshInterval: 1 * time.Minute,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
			"Content-Type":  "application/json",
		},
	}

	logger := zap.NewNop()
	dataSource := NewHTTPDataSource(config, logger)

	// Start the data source
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := dataSource.Start(ctx)
	require.NoError(t, err)

	// Wait a bit for initial load
	time.Sleep(100 * time.Millisecond)

	// Test lookup
	result, err := dataSource.Lookup(ctx, "user-service")
	assert.NoError(t, err)
	assert.Equal(t, "platform-team", result["owner"])
	assert.Equal(t, "production", result["environment"])

	result, err = dataSource.Lookup(ctx, "payment-service")
	assert.NoError(t, err)
	assert.Equal(t, "payments-team", result["owner"])

	// Test lookup miss
	_, err = dataSource.Lookup(ctx, "nonexistent-service")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")

	// Stop the data source
	err = dataSource.Stop()
	assert.NoError(t, err)
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
	dataSource := NewHTTPDataSource(config, logger)

	ctx := context.Background()
	err := dataSource.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request failed with status: 500")
}

func TestHTTPDataSource_InvalidJSON(t *testing.T) {
	// Test with server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	config := HTTPDataSourceConfig{
		URL:             server.URL,
		Timeout:         1 * time.Second,
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewHTTPDataSource(config, logger)

	ctx := context.Background()
	err := dataSource.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

func TestNewFileDataSource(t *testing.T) {
	config := FileDataSourceConfig{
		Path:            "/path/to/file.json",
		Format:          "json",
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	assert.NotNil(t, dataSource)
	assert.Equal(t, config, dataSource.config)
	assert.Empty(t, dataSource.data)
}

func TestFileDataSource_JSON(t *testing.T) {
	// Create temporary JSON file
	testData := map[string]interface{}{
		"web-01": map[string]interface{}{
			"system_id":   "SYS-001",
			"location":    "us-east-1",
			"environment": "production",
		},
		"db-01": map[string]interface{}{
			"system_id":   "SYS-002",
			"location":    "us-west-2",
			"environment": "production",
		},
	}

	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "test.json")

	data, err := json.Marshal(testData)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, data, 0644)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            jsonFile,
		Format:          "json",
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	// Start the data source
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = dataSource.Start(ctx)
	require.NoError(t, err)

	// Test lookup
	result, err := dataSource.Lookup(ctx, "web-01")
	assert.NoError(t, err)
	assert.Equal(t, "SYS-001", result["system_id"])
	assert.Equal(t, "us-east-1", result["location"])

	result, err = dataSource.Lookup(ctx, "db-01")
	assert.NoError(t, err)
	assert.Equal(t, "SYS-002", result["system_id"])

	// Test lookup miss
	_, err = dataSource.Lookup(ctx, "nonexistent-host")
	assert.Error(t, err)

	// Stop the data source
	err = dataSource.Stop()
	assert.NoError(t, err)
}

func TestFileDataSource_CSV(t *testing.T) {
	// Create temporary CSV file
	csvContent := `hostname,system_id,location,environment
web-01,SYS-001,us-east-1,production
db-01,SYS-002,us-west-2,production
cache-01,SYS-003,us-central-1,staging`

	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test.csv")

	err := os.WriteFile(csvFile, []byte(csvContent), 0644)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            csvFile,
		Format:          "csv",
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	// Start the data source
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = dataSource.Start(ctx)
	require.NoError(t, err)

	// Test lookup
	result, err := dataSource.Lookup(ctx, "web-01")
	assert.NoError(t, err)
	assert.Equal(t, "SYS-001", result["system_id"])
	assert.Equal(t, "us-east-1", result["location"])
	assert.Equal(t, "production", result["environment"])

	result, err = dataSource.Lookup(ctx, "cache-01")
	assert.NoError(t, err)
	assert.Equal(t, "SYS-003", result["system_id"])
	assert.Equal(t, "staging", result["environment"])

	// Stop the data source
	err = dataSource.Stop()
	assert.NoError(t, err)
}

func TestFileDataSource_UnsupportedFormat(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.xml")

	err := os.WriteFile(testFile, []byte("<data></data>"), 0644)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            testFile,
		Format:          "xml",
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	ctx := context.Background()
	err = dataSource.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported file format: xml")
}

func TestFileDataSource_FileNotFound(t *testing.T) {
	config := FileDataSourceConfig{
		Path:            "/nonexistent/file.json",
		Format:          "json",
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	ctx := context.Background()
	err := dataSource.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat file")
}

func TestFileDataSource_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "invalid.json")

	err := os.WriteFile(jsonFile, []byte("invalid json content"), 0644)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            jsonFile,
		Format:          "json",
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	ctx := context.Background()
	err = dataSource.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse file")
}

func TestFileDataSource_InvalidCSV(t *testing.T) {
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "invalid.csv")

	// CSV with only header, no data
	err := os.WriteFile(csvFile, []byte("header1,header2"), 0644)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            csvFile,
		Format:          "csv",
		RefreshInterval: 1 * time.Minute,
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	ctx := context.Background()
	err = dataSource.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CSV file must have at least 2 rows")
}

func TestFileDataSource_FileModification(t *testing.T) {
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "dynamic.json")

	// Initial data
	initialData := []map[string]interface{}{
		{
			"hostname": "initial-host",
			"value":    "initial-value",
		},
	}

	data, err := json.Marshal(initialData)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, data, 0644)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            jsonFile,
		Format:          "json",
		RefreshInterval: 100 * time.Millisecond, // Fast refresh for testing
	}

	logger := zap.NewNop()
	dataSource := NewFileDataSource(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = dataSource.Start(ctx)
	require.NoError(t, err)

	// Check initial data
	result, err := dataSource.Lookup(ctx, "initial-host")
	assert.NoError(t, err)
	assert.Equal(t, "initial-value", result["value"])

	// Wait a bit then modify file
	time.Sleep(50 * time.Millisecond)

	updatedData := []map[string]interface{}{
		{
			"hostname": "initial-host",
			"value":    "updated-value",
		},
		{
			"hostname": "new-host",
			"value":    "new-value",
		},
	}

	data, err = json.Marshal(updatedData)
	require.NoError(t, err)

	err = os.WriteFile(jsonFile, data, 0644)
	require.NoError(t, err)

	// Wait for refresh
	time.Sleep(200 * time.Millisecond)

	// Check updated data
	result, err = dataSource.Lookup(ctx, "initial-host")
	assert.NoError(t, err)
	assert.Equal(t, "updated-value", result["value"])

	result, err = dataSource.Lookup(ctx, "new-host")
	assert.NoError(t, err)
	assert.Equal(t, "new-value", result["value"])

	err = dataSource.Stop()
	assert.NoError(t, err)
}

func TestHTTPDataSource_ProcessJSONData(t *testing.T) {
	logger := zap.NewNop()
	dataSource := NewHTTPDataSource(HTTPDataSourceConfig{}, logger)

	t.Run("Array of objects", func(t *testing.T) {
		jsonData := []interface{}{
			map[string]interface{}{
				"name":  "service1",
				"owner": "team1",
			},
			map[string]interface{}{
				"name":  "service2",
				"owner": "team2",
			},
		}

		result, err := dataSource.processJSONData(jsonData)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Contains(t, result, "service1")
		assert.Contains(t, result, "service2")
		assert.Equal(t, "team1", result["service1"]["owner"])
	})

	t.Run("Object with nested objects", func(t *testing.T) {
		jsonData := map[string]interface{}{
			"service1": map[string]interface{}{
				"owner": "team1",
				"env":   "prod",
			},
			"service2": map[string]interface{}{
				"owner": "team2",
				"env":   "staging",
			},
		}

		result, err := dataSource.processJSONData(jsonData)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "team1", result["service1"]["owner"])
		assert.Equal(t, "prod", result["service1"]["env"])
	})
}

func TestFileDataSource_ParseCSV(t *testing.T) {
	logger := zap.NewNop()
	dataSource := NewFileDataSource(FileDataSourceConfig{}, logger)

	t.Run("Valid CSV", func(t *testing.T) {
		csvContent := `hostname,owner,environment
web-01,platform,production
db-01,data,production`

		reader := strings.NewReader(csvContent)
		result, err := dataSource.parseCSV(reader)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "platform", result["web-01"]["owner"])
		assert.Equal(t, "production", result["web-01"]["environment"])
	})

	t.Run("CSV with malformed rows", func(t *testing.T) {
		csvContent := `hostname,owner,environment
web-01,platform,production
db-01,data
cache-01,platform,staging`

		reader := strings.NewReader(csvContent)
		result, err := dataSource.parseCSV(reader)
		assert.NoError(t, err)
		assert.Len(t, result, 2) // Malformed row should be skipped
		assert.Contains(t, result, "web-01")
		assert.Contains(t, result, "cache-01")
		assert.NotContains(t, result, "db-01")
	})
}

func TestDataSourceInterface(t *testing.T) {
	// Test that our implementations satisfy the DataSource interface
	var _ DataSource = &HTTPDataSource{}
	var _ DataSource = &FileDataSource{}

	// Test interface methods are implemented
	logger := zap.NewNop()

	httpDS := NewHTTPDataSource(HTTPDataSourceConfig{}, logger)
	assert.Implements(t, (*DataSource)(nil), httpDS)

	fileDS := NewFileDataSource(FileDataSourceConfig{}, logger)
	assert.Implements(t, (*DataSource)(nil), fileDS)
}
