// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	projectID    = "ProjectID"
	instanceID   = "InstanceID"
	databaseName = "DatabaseName"

	name = "name"
)

func TestCurrentStatsReader_Name(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, "")
	database := datasource.NewDatabaseFromClient(client, databaseID)
	metricsMetadata := &metadata.MetricsMetadata{
		Name: name,
	}

	reader := currentStatsReader{
		database:        database,
		metricsMetadata: metricsMetadata,
	}

	assert.Equal(t, reader.metricsMetadata.Name+" "+databaseID.ProjectID()+"::"+
		databaseID.InstanceID()+"::"+databaseID.DatabaseName(), reader.Name())
}

func TestNewCurrentStatsReader(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, "")
	database := datasource.NewDatabaseFromClient(client, databaseID)
	metricsMetadata := &metadata.MetricsMetadata{
		Name: name,
	}
	logger := zaptest.NewLogger(t)
	config := ReaderConfig{
		TopMetricsQueryMaxRows: topMetricsQueryMaxRows,
	}

	reader := newCurrentStatsReader(logger, database, metricsMetadata, config)

	assert.Equal(t, database, reader.database)
	assert.Equal(t, logger, reader.logger)
	assert.Equal(t, metricsMetadata, reader.metricsMetadata)
	assert.Equal(t, topMetricsQueryMaxRows, reader.topMetricsQueryMaxRows)
}

func TestCurrentStatsReader_NewPullStatement(t *testing.T) {
	metricsMetadata := &metadata.MetricsMetadata{
		Query: query,
	}

	reader := currentStatsReader{
		metricsMetadata:        metricsMetadata,
		topMetricsQueryMaxRows: topMetricsQueryMaxRows,
		statement:              currentStatsStatement,
	}

	assert.NotZero(t, reader.newPullStatement())
}

func TestIsSafeToUseStaleRead(t *testing.T) {
	testCases := map[string]struct {
		secondsAfterStartOfMinute int
		expectedResult            bool
	}{
		"Statement with top metrics query max rows":    {dataStalenessSeconds, false},
		"Statement without top metrics query max rows": {dataStalenessSeconds*2 + 5, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			readTimestamp := time.Date(2021, 9, 17, 16, 25, testCase.secondsAfterStartOfMinute, 0, time.UTC)

			assert.Equal(t, testCase.expectedResult, isSafeToUseStaleRead(readTimestamp))
		})
	}
}
