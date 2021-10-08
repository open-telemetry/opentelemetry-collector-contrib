// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statsreader

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type testReader struct {
	throwError bool
}

func (tr testReader) Name() string {
	return "testReader"
}

func (tr testReader) Read(_ context.Context) ([]pdata.Metrics, error) {
	if tr.throwError {
		return nil, errors.New("error")
	}

	return []pdata.Metrics{{}}, nil
}

func TestNewDatabaseReader(t *testing.T) {
	ctx := context.Background()
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	serviceAccountPath := "../../testdata/serviceAccount.json"
	readerConfig := ReaderConfig{
		TopMetricsQueryMaxRows: topMetricsQueryMaxRows,
		BackfillEnabled:        false,
	}
	logger := zap.NewNop()
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
	logger := zap.NewNop()
	var parsedMetadata []*metadata.MetricsMetadata

	reader, err := NewDatabaseReader(ctx, parsedMetadata, databaseID, serviceAccountPath, readerConfig, logger)

	assert.NotNil(t, err)
	// Do not call executeShutdown() here because reader hasn't been created
	assert.Nil(t, reader)
}

func TestInitializeReaders(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	logger := zap.NewNop()
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
	logger := zap.NewNop()

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
	logger := zap.NewNop()

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
	logger := zap.NewNop()

	testReaderThrowNoError := testReader{
		throwError: false,
	}

	testReaderThrowError := testReader{
		throwError: true,
	}

	testCases := map[string]struct {
		readers              []Reader
		expectedMetricsCount int
	}{
		"Read with no error": {[]Reader{testReaderThrowNoError}, 1},
		"Read with error":    {[]Reader{testReaderThrowError}, 0},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			reader := &DatabaseReader{
				logger:   logger,
				database: database,
				readers:  testCase.readers,
			}
			defer executeShutdown(reader)

			metrics := reader.Read(ctx)

			assert.Equal(t, testCase.expectedMetricsCount, len(metrics))
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
