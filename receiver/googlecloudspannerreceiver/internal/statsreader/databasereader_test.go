// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type mockReader struct {
	mock.Mock
}

func (r *mockReader) Name() string {
	return "mockReader"
}

func (r *mockReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*metadata.MetricsDataPoint), args.Error(1)
}

func TestNewDatabaseReader(t *testing.T) {
	ctx := context.Background()
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	serviceAccountPath := "../../testdata/serviceAccount.json"
	readerConfig := ReaderConfig{
		TopMetricsQueryMaxRows: topMetricsQueryMaxRows,
		BackfillEnabled:        false,
	}
	logger := zaptest.NewLogger(t)
	var parsedMetadata []*metadata.MetricsMetadata

	reader, err := NewDatabaseReader(ctx, parsedMetadata, databaseID, serviceAccountPath, readerConfig, logger)

	assert.Nil(t, err)

	defer executeShutdown(reader)

	assert.Equal(t, databaseID, reader.database.DatabaseID())
	assert.Equal(t, logger, reader.logger)
	assert.Equal(t, 0, len(reader.readers))
}

func TestNewDatabaseReaderWithError(t *testing.T) {
	ctx := context.Background()
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	serviceAccountPath := "does not exist"
	readerConfig := ReaderConfig{
		TopMetricsQueryMaxRows: topMetricsQueryMaxRows,
		BackfillEnabled:        false,
	}
	logger := zaptest.NewLogger(t)
	var parsedMetadata []*metadata.MetricsMetadata

	reader, err := NewDatabaseReader(ctx, parsedMetadata, databaseID, serviceAccountPath, readerConfig, logger)

	assert.NotNil(t, err)
	// Do not call executeShutdown() here because reader hasn't been created
	assert.Nil(t, reader)
}

func TestInitializeReaders(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	logger := zaptest.NewLogger(t)
	var client *spanner.Client
	database := datasource.NewDatabaseFromClient(client, databaseID)
	currentStatsMetadata := createMetricsMetadata(query)
	intervalStatsMetadata := createMetricsMetadata(query)

	currentStatsMetadata.Name = "Current"
	currentStatsMetadata.TimestampColumnName = ""
	intervalStatsMetadata.Name = "Interval"

	parsedMetadata := []*metadata.MetricsMetadata{
		currentStatsMetadata,
		intervalStatsMetadata,
	}
	readerConfig := ReaderConfig{
		TopMetricsQueryMaxRows: topMetricsQueryMaxRows,
		BackfillEnabled:        false,
	}

	readers := initializeReaders(logger, parsedMetadata, database, readerConfig)

	assert.Equal(t, 2, len(readers))
	assert.IsType(t, &currentStatsReader{}, readers[0])
	assert.IsType(t, &intervalStatsReader{}, readers[1])
}

func TestDatabaseReader_Name(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, databaseName)
	database := datasource.NewDatabaseFromClient(client, databaseID)
	logger := zaptest.NewLogger(t)

	reader := &DatabaseReader{
		logger:   logger,
		database: database,
	}
	defer executeShutdown(reader)

	assert.Equal(t, database.DatabaseID().ID(), reader.Name())
}

func TestDatabaseReader_Shutdown(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, databaseName)
	database := datasource.NewDatabaseFromClient(client, databaseID)
	logger := zaptest.NewLogger(t)

	reader := &DatabaseReader{
		logger:   logger,
		database: database,
	}

	executeShutdown(reader)
}

func TestDatabaseReader_Read(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, databaseName)
	database := datasource.NewDatabaseFromClient(client, databaseID)
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		expectedError error
	}{
		"Read with no error": {nil},
		"Read with error":    {errors.New("read error")},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mr := &mockReader{}
			readers := []Reader{mr}
			reader := &DatabaseReader{
				logger:   logger,
				database: database,
				readers:  readers,
			}
			defer executeShutdown(reader)

			mr.On("Read", ctx).Return([]*metadata.MetricsDataPoint{}, testCase.expectedError)

			_, err := reader.Read(ctx)

			mr.AssertExpectations(t)

			if testCase.expectedError != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func executeShutdown(reader *DatabaseReader) {
	// Doing this because can't control instantiation of Spanner DB client.
	// In Shutdown() invocation client with wrong DB parameters produces panic when calling its Close() method.
	defer func() {
		_ = recover()
	}()

	reader.Shutdown()
}
