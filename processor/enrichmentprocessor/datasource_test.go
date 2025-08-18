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

func TestHTTPDataSource_Lifecycle(t *testing.T) {
	testData := []map[string]interface{}{
		{"name": "user-service", "owner": "platform-team"},
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
	indexFields := []string{"name"}
	dataSource := NewHTTPDataSource(config, logger, indexFields)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := dataSource.Start(ctx)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = dataSource.Stop()
	assert.NoError(t, err)
}

func TestFileDataSource_Lifecycle(t *testing.T) {
	testData := []map[string]interface{}{
		{"hostname": "web-01", "environment": "production"},
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
	indexFields := []string{"hostname"}
	dataSource := NewFileDataSource(config, logger, indexFields)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = dataSource.Start(ctx)
	require.NoError(t, err)

	err = dataSource.Stop()
	assert.NoError(t, err)
}

func TestHTTPDataSource_FormatDetection(t *testing.T) {
	t.Run("csv_format", func(t *testing.T) {
		csvData := "name,owner\nuser-service,platform-team"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/csv")
			w.Write([]byte(csvData))
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:             server.URL,
			RefreshInterval: 1 * time.Minute,
		}
		dataSource := NewHTTPDataSource(config, zap.NewNop(), []string{"name"})

		err := dataSource.Start(context.Background())
		require.NoError(t, err)
		dataSource.Stop()
	})

	t.Run("unknown_content_type", func(t *testing.T) {
		jsonData := `[{"name": "test"}]`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(jsonData))
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:             server.URL,
			RefreshInterval: 1 * time.Minute,
		}
		dataSource := NewHTTPDataSource(config, zap.NewNop(), []string{"name"})

		err := dataSource.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to determine data format from Content-Type header")
	})
}

func TestHTTPDataSource_ErrorHandling(t *testing.T) {
	t.Run("http_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:             server.URL,
			RefreshInterval: 1 * time.Minute,
		}
		dataSource := NewHTTPDataSource(config, zap.NewNop(), []string{"name"})

		err := dataSource.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP request failed with status: 500")
	})

	t.Run("invalid_json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := HTTPDataSourceConfig{
			URL:             server.URL,
			Format:          "json",
			RefreshInterval: 1 * time.Minute,
		}
		dataSource := NewHTTPDataSource(config, zap.NewNop(), []string{"name"})

		err := dataSource.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON")
	})
}

func TestParseJSON(t *testing.T) {
	jsonData := `[{"name": "service", "owner": "team"}]`
	rows, headerIndex, err := parseJSON([]byte(jsonData))

	assert.NoError(t, err)
	assert.Equal(t, 1, len(rows))
	assert.Contains(t, headerIndex, "name")
	assert.Contains(t, headerIndex, "owner")
}

func TestParseCSV(t *testing.T) {
	csvData := "name,owner\nservice,team"
	rows, headerIndex, err := parseCSV([]byte(csvData))

	assert.NoError(t, err)
	assert.Equal(t, 1, len(rows))
	assert.Contains(t, headerIndex, "name")
	assert.Contains(t, headerIndex, "owner")
}
