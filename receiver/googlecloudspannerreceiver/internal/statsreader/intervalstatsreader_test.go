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

func TestIntervalStatsReader_Name(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, "")
	database := datasource.NewDatabaseFromClient(client, databaseID)
	metricsMetadata := &metadata.MetricsMetadata{
		Name: name,
	}

	reader := intervalStatsReader{
		currentStatsReader: currentStatsReader{
			database:        database,
			metricsMetadata: metricsMetadata,
		},
	}

	assert.Equal(t, reader.metricsMetadata.Name+" "+databaseID.ProjectID()+"::"+
		databaseID.InstanceID()+"::"+databaseID.DatabaseName(), reader.Name())
}

func TestNewIntervalStatsReader(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, "")
	database := datasource.NewDatabaseFromClient(client, databaseID)
	metricsMetadata := &metadata.MetricsMetadata{
		Name: name,
	}
	logger := zaptest.NewLogger(t)
	config := ReaderConfig{
		TopMetricsQueryMaxRows:            topMetricsQueryMaxRows,
		BackfillEnabled:                   true,
		HideTopnLockstatsRowrangestartkey: true,
		TruncateText:                      true,
	}

	reader := newIntervalStatsReader(logger, database, metricsMetadata, config)

	assert.Equal(t, database, reader.database)
	assert.Equal(t, logger, reader.logger)
	assert.Equal(t, metricsMetadata, reader.metricsMetadata)
	assert.Equal(t, topMetricsQueryMaxRows, reader.topMetricsQueryMaxRows)
	assert.NotNil(t, reader.timestampsGenerator)
	assert.True(t, reader.timestampsGenerator.backfillEnabled)
	assert.True(t, reader.hideTopnLockstatsRowrangestartkey)
	assert.True(t, reader.truncateText)
}

func TestIntervalStatsReader_NewPullStatement(t *testing.T) {
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	timestamp := time.Now().UTC()
	logger := zaptest.NewLogger(t)
	config := ReaderConfig{
		TopMetricsQueryMaxRows:            topMetricsQueryMaxRows,
		BackfillEnabled:                   false,
		HideTopnLockstatsRowrangestartkey: true,
		TruncateText:                      true,
	}
	ctx := context.Background()
	client, _ := spanner.NewClient(ctx, "")
	database := datasource.NewDatabaseFromClient(client, databaseID)
	metricsMetadata := &metadata.MetricsMetadata{
		Query: query,
	}
	reader := newIntervalStatsReader(logger, database, metricsMetadata, config)

	assert.NotZero(t, reader.newPullStatement(timestamp))
}
