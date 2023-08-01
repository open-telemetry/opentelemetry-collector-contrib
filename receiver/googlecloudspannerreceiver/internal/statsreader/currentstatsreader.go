// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	dataStalenessSeconds = 15
	dataStalenessPeriod  = dataStalenessSeconds * time.Second

	// Data reads for backfilling are always performed from stale replica nodes and not from the master.
	// For current/fresh data reads main node is used. But in case collector started for example at mm:30+ it is safe
	// to read data from stale replica nodes(at this time replica node will contain required data), since we are
	// requesting data for mm:00. Also stale reads are faster than reads from main node.
	dataStalenessSafeThresholdSeconds = 2 * dataStalenessSeconds
)

type currentStatsReader struct {
	logger                 *zap.Logger
	database               *datasource.Database
	metricsMetadata        *metadata.MetricsMetadata
	topMetricsQueryMaxRows int
	statement              func(args statementArgs) statsStatement
}

func newCurrentStatsReader(
	logger *zap.Logger,
	database *datasource.Database,
	metricsMetadata *metadata.MetricsMetadata,
	config ReaderConfig) *currentStatsReader {

	return &currentStatsReader{
		logger:                 logger,
		database:               database,
		metricsMetadata:        metricsMetadata,
		statement:              currentStatsStatement,
		topMetricsQueryMaxRows: config.TopMetricsQueryMaxRows,
	}
}

func (reader *currentStatsReader) Name() string {
	return fmt.Sprintf("%v %v::%v::%v", reader.metricsMetadata.Name, reader.database.DatabaseID().ProjectID(),
		reader.database.DatabaseID().InstanceID(), reader.database.DatabaseID().DatabaseName())
}

func (reader *currentStatsReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	reader.logger.Debug("Executing read method", zap.String("reader", reader.Name()))

	stmt := reader.newPullStatement()

	return reader.pull(ctx, stmt)
}

func (reader *currentStatsReader) newPullStatement() statsStatement {
	args := statementArgs{
		query:                  reader.metricsMetadata.Query,
		topMetricsQueryMaxRows: reader.topMetricsQueryMaxRows,
	}

	return reader.statement(args)
}

func (reader *currentStatsReader) pull(ctx context.Context, stmt statsStatement) ([]*metadata.MetricsDataPoint, error) {
	transaction := reader.database.Client().Single()
	if stmt.stalenessRead || isSafeToUseStaleRead(time.Now().UTC()) {
		transaction = transaction.WithTimestampBound(spanner.ExactStaleness(dataStalenessPeriod))
	}
	rowsIterator := transaction.Query(ctx, stmt.statement)
	defer rowsIterator.Stop()

	var collectedDataPoints []*metadata.MetricsDataPoint

	for {
		row, err := rowsIterator.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return collectedDataPoints, nil
			}
			return nil, fmt.Errorf("query %q failed with error: %w", stmt.statement.SQL, err)
		}

		rowMetricsDataPoints, err := reader.metricsMetadata.RowToMetricsDataPoints(reader.database.DatabaseID(), row)
		if err != nil {
			return nil, fmt.Errorf("query %q failed with error: %w", stmt.statement.SQL, err)
		}

		collectedDataPoints = append(collectedDataPoints, rowMetricsDataPoints...)
	}
}

func isSafeToUseStaleRead(readTimestamp time.Time) bool {
	return (readTimestamp.Second() - dataStalenessSafeThresholdSeconds) >= 0
}
