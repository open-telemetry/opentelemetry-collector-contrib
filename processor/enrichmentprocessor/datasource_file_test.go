// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFileDataSource_JSON(t *testing.T) {
	// Create temporary JSON file
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "test.json")

	jsonData := `[
		{"service_name": "user-service", "team": "platform", "environment": "prod"},
		{"service_name": "payment-service", "team": "payments", "environment": "stage"}
	]`

	err := os.WriteFile(jsonFile, []byte(jsonData), 0600)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            jsonFile,
		Format:          "json",
		RefreshInterval: time.Minute,
	}

	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	ds := NewFileDataSource(config, logger, indexFields)

	// Test refresh (simulates Start behavior without goroutine)
	err = ds.refresh(t.Context())
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

func TestFileDataSource_CSV(t *testing.T) {
	// Create temporary CSV file
	tempDir := t.TempDir()
	csvFile := filepath.Join(tempDir, "test.csv")

	csvData := `service_name,team,environment
user-service,platform,prod
payment-service,payments,stage`

	err := os.WriteFile(csvFile, []byte(csvData), 0600)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            csvFile,
		Format:          "csv",
		RefreshInterval: time.Minute,
	}

	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	ds := NewFileDataSource(config, logger, indexFields)

	// Test refresh
	err = ds.refresh(t.Context())
	require.NoError(t, err)

	// Test lookup
	row, index, err := ds.Lookup("service_name", "payment-service")
	require.NoError(t, err)

	assert.Equal(t, "payment-service", row[index["service_name"]])
	assert.Equal(t, "payments", row[index["team"]])
	assert.Equal(t, "stage", row[index["environment"]])
}

func TestFileDataSource_FileModificationDetection(t *testing.T) {
	// Create temporary JSON file
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "test.json")

	initialData := `[
		{"service_name": "initial-service", "team": "initial-team"}
	]`

	err := os.WriteFile(jsonFile, []byte(initialData), 0600)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            jsonFile,
		Format:          "json",
		RefreshInterval: time.Millisecond * 100,
	}

	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	ds := NewFileDataSource(config, logger, indexFields)

	// Initial load
	err = ds.refresh(t.Context())
	require.NoError(t, err)

	// Verify initial data
	row, index, err := ds.Lookup("service_name", "initial-service")
	require.NoError(t, err)
	assert.Equal(t, "initial-service", row[index["service_name"]])

	// Wait a bit to ensure different modification time
	time.Sleep(time.Millisecond * 10)

	// Update file
	updatedData := `[
		{"service_name": "updated-service", "team": "updated-team"}
	]`

	err = os.WriteFile(jsonFile, []byte(updatedData), 0600)
	require.NoError(t, err)

	// Refresh should detect the change
	err = ds.refresh(t.Context())
	require.NoError(t, err)

	// Verify updated data
	row, index, err = ds.Lookup("service_name", "updated-service")
	require.NoError(t, err)
	assert.Equal(t, "updated-service", row[index["service_name"]])

	// Old data should no longer exist
	_, _, err = ds.Lookup("service_name", "initial-service")
	assert.Error(t, err)
}

func TestFileDataSource_ErrorHandling(t *testing.T) {
	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	t.Run("file_not_found", func(t *testing.T) {
		config := FileDataSourceConfig{
			Path:   "/non/existent/file.json",
			Format: "json",
		}

		ds := NewFileDataSource(config, logger, indexFields)
		err := ds.refresh(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to stat file")
	})

	t.Run("invalid_json_file", func(t *testing.T) {
		tempDir := t.TempDir()
		jsonFile := filepath.Join(tempDir, "invalid.json")

		err := os.WriteFile(jsonFile, []byte("{invalid json}"), 0600)
		require.NoError(t, err)

		config := FileDataSourceConfig{
			Path:   jsonFile,
			Format: "json",
		}

		ds := NewFileDataSource(config, logger, indexFields)
		err = ds.refresh(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse file")
	})

	t.Run("invalid_csv_file", func(t *testing.T) {
		tempDir := t.TempDir()
		csvFile := filepath.Join(tempDir, "invalid.csv")

		// CSV with unclosed quotes
		err := os.WriteFile(csvFile, []byte("name,team\n\"unclosed,quote"), 0600)
		require.NoError(t, err)

		config := FileDataSourceConfig{
			Path:   csvFile,
			Format: "csv",
		}

		ds := NewFileDataSource(config, logger, indexFields)
		err = ds.refresh(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse file")
	})

	t.Run("unsupported_format", func(t *testing.T) {
		tempDir := t.TempDir()
		xmlFile := filepath.Join(tempDir, "test.xml")

		err := os.WriteFile(xmlFile, []byte("<data></data>"), 0600)
		require.NoError(t, err)

		config := FileDataSourceConfig{
			Path:   xmlFile,
			Format: "xml",
		}

		ds := NewFileDataSource(config, logger, indexFields)
		err = ds.refresh(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file format: xml")
	})
}

func TestFileDataSource_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "test.json")

	jsonData := `[{"service_name": "test-service"}]`
	err := os.WriteFile(jsonFile, []byte(jsonData), 0600)
	require.NoError(t, err)

	config := FileDataSourceConfig{
		Path:            jsonFile,
		Format:          "json",
		RefreshInterval: time.Millisecond * 100,
	}

	logger := zap.NewNop()
	indexFields := []string{"service_name"}

	ds := NewFileDataSource(config, logger, indexFields)

	// Test Start
	ctx := t.Context()
	err = ds.Start(ctx)
	require.NoError(t, err)

	// Verify data was loaded
	row, index, err := ds.Lookup("service_name", "test-service")
	require.NoError(t, err)
	assert.Equal(t, "test-service", row[index["service_name"]])

	// Test Stop
	err = ds.Stop()
	assert.NoError(t, err)
}
