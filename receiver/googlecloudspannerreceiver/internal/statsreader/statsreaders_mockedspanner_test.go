// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"cloud.google.com/go/spanner/spannertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	spannerDatabaseName = "projects/" + projectID + "/instances/" + instanceID + "/databases/" + databaseName
	maxRowsLimit        = 1
)

func createMetricsMetadata(query string) *metadata.MetricsMetadata {
	return createMetricsMetadataFromTimestampColumn(query, "INTERVAL_END")
}

func createMetricsMetadataFromTimestampColumn(query string, timestampColumn string) *metadata.MetricsMetadata {
	labelValueMetadata, _ := metadata.NewLabelValueMetadata("metric_label", "METRIC_LABEL",
		metadata.StringValueType)
	// Labels
	queryLabelValuesMetadata := []metadata.LabelValueMetadata{labelValueMetadata}

	metricDataType := metadata.NewMetricType(pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, false)

	metricValueMetadata, _ := metadata.NewMetricValueMetadata("metric_value", "METRIC_VALUE", metricDataType, "unit",
		metadata.IntValueType)
	// Metrics
	queryMetricValuesMetadata := []metadata.MetricValueMetadata{metricValueMetadata}

	return &metadata.MetricsMetadata{
		Name:                      "test stats",
		Query:                     query,
		MetricNamePrefix:          "test_stats/",
		TimestampColumnName:       timestampColumn,
		QueryLabelValuesMetadata:  queryLabelValuesMetadata,
		QueryMetricValuesMetadata: queryMetricValuesMetadata,
	}
}

func createCurrentStatsReaderWithCorruptedMetadata(client *spanner.Client) Reader { //nolint
	query := "SELECT * FROM STATS"
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	databaseFromClient := datasource.NewDatabaseFromClient(client, databaseID)

	return newCurrentStatsReader(zap.NewNop(), databaseFromClient,
		createMetricsMetadataFromTimestampColumn(query, "NOT_EXISTING"), ReaderConfig{})
}

func createCurrentStatsReader(client *spanner.Client) Reader { //nolint
	query := "SELECT * FROM STATS"
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	databaseFromClient := datasource.NewDatabaseFromClient(client, databaseID)

	return newCurrentStatsReader(zap.NewNop(), databaseFromClient, createMetricsMetadata(query), ReaderConfig{})
}

func createCurrentStatsReaderWithMaxRowsLimit(client *spanner.Client) Reader { //nolint
	query := "SELECT * FROM STATS"
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	databaseFromClient := datasource.NewDatabaseFromClient(client, databaseID)
	config := ReaderConfig{
		TopMetricsQueryMaxRows: maxRowsLimit,
	}

	return newCurrentStatsReader(zap.NewNop(), databaseFromClient, createMetricsMetadata(query), config)
}

func createIntervalStatsReaderWithCorruptedMetadata(client *spanner.Client, backfillEnabled bool) Reader { //nolint
	query := "SELECT * FROM STATS WHERE INTERVAL_END = @pullTimestamp"
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	databaseFromClient := datasource.NewDatabaseFromClient(client, databaseID)
	config := ReaderConfig{
		BackfillEnabled: backfillEnabled,
	}

	return newIntervalStatsReader(zap.NewNop(), databaseFromClient,
		createMetricsMetadataFromTimestampColumn(query, "NOT_EXISTING"), config)
}

func createIntervalStatsReader(client *spanner.Client, backfillEnabled bool) Reader { //nolint
	query := "SELECT * FROM STATS WHERE INTERVAL_END = @pullTimestamp"
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	databaseFromClient := datasource.NewDatabaseFromClient(client, databaseID)
	config := ReaderConfig{
		BackfillEnabled: backfillEnabled,
	}

	return newIntervalStatsReader(zap.NewNop(), databaseFromClient, createMetricsMetadata(query), config)
}

func createIntervalStatsReaderWithMaxRowsLimit(client *spanner.Client, backfillEnabled bool) Reader { //nolint
	query := "SELECT * FROM STATS WHERE INTERVAL_END = @pullTimestamp"
	databaseID := datasource.NewDatabaseID(projectID, instanceID, databaseName)
	databaseFromClient := datasource.NewDatabaseFromClient(client, databaseID)
	config := ReaderConfig{
		TopMetricsQueryMaxRows: maxRowsLimit,
		BackfillEnabled:        backfillEnabled,
	}

	return newIntervalStatsReader(zap.NewNop(), databaseFromClient, createMetricsMetadata(query), config)
}

func TestStatsReaders_Read(t *testing.T) {
	t.Skip("Flaky test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/6318")
	timestamp := shiftToStartOfMinute(time.Now().UTC())
	ctx := context.Background()
	server, err := spannertest.NewServer(":0")
	require.NoError(t, err)
	defer server.Close()

	conn, err := grpc.Dial(server.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	databaseAdminClient, err := database.NewDatabaseAdminClient(ctx, option.WithGRPCConn(conn))
	require.NoError(t, err)
	defer func(databaseAdminClient *database.DatabaseAdminClient) {
		_ = databaseAdminClient.Close()
	}(databaseAdminClient)

	op, err := databaseAdminClient.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database: databaseName,
		Statements: []string{`CREATE TABLE STATS (
			INTERVAL_END TIMESTAMP,
			METRIC_LABEL STRING(MAX),
			METRIC_VALUE INT64
		) PRIMARY KEY (METRIC_LABEL)
		`},
	})
	require.NoError(t, err)

	err = op.Wait(ctx)
	require.NoError(t, err)

	databaseClient, err := spanner.NewClient(ctx, spannerDatabaseName, option.WithGRPCConn(conn))
	require.NoError(t, err)
	defer databaseClient.Close()

	_, err = databaseClient.Apply(ctx, []*spanner.Mutation{
		spanner.Insert("STATS",
			[]string{"INTERVAL_END", "METRIC_LABEL", "METRIC_VALUE"},
			[]interface{}{timestamp, "Qwerty", 10}),
		spanner.Insert("STATS",
			[]string{"INTERVAL_END", "METRIC_LABEL", "METRIC_VALUE"},
			[]interface{}{timestamp.Add(-1 * time.Minute), "Test", 20}),
		spanner.Insert("STATS",
			[]string{"INTERVAL_END", "METRIC_LABEL", "METRIC_VALUE"},
			[]interface{}{timestamp.Add(-1 * time.Minute), "Spanner", 30}),
	})

	require.NoError(t, err)

	testCases := map[string]struct {
		reader                Reader
		expectedMetricsAmount int
		expectError           bool
	}{
		"Current stats reader without max rows limit":                    {createCurrentStatsReader(databaseClient), 3, false},
		"Current stats reader with max rows limit":                       {createCurrentStatsReaderWithMaxRowsLimit(databaseClient), 1, false},
		"Current stats reader with corrupted metadata":                   {createCurrentStatsReaderWithCorruptedMetadata(databaseClient), 0, true},
		"Interval stats reader without backfill without max rows limit":  {createIntervalStatsReader(databaseClient, false), 1, false},
		"Interval stats reader without backfill with max rows limit":     {createIntervalStatsReaderWithMaxRowsLimit(databaseClient, false), 1, false},
		"Interval stats reader with backfill without max rows limit":     {createIntervalStatsReader(databaseClient, true), 3, false},
		"Interval stats reader with backfill with max rows limit":        {createIntervalStatsReaderWithMaxRowsLimit(databaseClient, true), 2, false},
		"Interval stats reader without backfill with corrupted metadata": {createIntervalStatsReaderWithCorruptedMetadata(databaseClient, false), 0, true},
		"Interval stats reader with backfill with corrupted metadata":    {createIntervalStatsReaderWithCorruptedMetadata(databaseClient, true), 0, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metrics, err := testCase.reader.Read(ctx)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.expectedMetricsAmount, len(metrics))
			}
		})
	}
}
