// Copyright The OpenTelemetry Authors
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

package statsreader // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	// Max duration of data backfilling(if enabled).
	// Different backends can support or not support such option.
	// Since, the initial intent was to work mainly with Prometheus backend,
	// this constant was set to 1 hour - max allowed interval by Prometheus.
	backfillIntervalDuration = time.Hour
)

type intervalStatsReader struct {
	currentStatsReader
	timestampsGenerator *timestampsGenerator
	lastPullTimestamp   time.Time
}

func newIntervalStatsReader(
	logger *zap.Logger,
	database *datasource.Database,
	metricsMetadata *metadata.MetricsMetadata,
	config ReaderConfig) *intervalStatsReader {

	reader := currentStatsReader{
		logger:                 logger,
		database:               database,
		metricsMetadata:        metricsMetadata,
		statement:              intervalStatsStatement,
		topMetricsQueryMaxRows: config.TopMetricsQueryMaxRows,
	}
	tsGenerator := &timestampsGenerator{
		backfillEnabled: config.BackfillEnabled,
		difference:      time.Minute,
	}

	return &intervalStatsReader{
		currentStatsReader:  reader,
		timestampsGenerator: tsGenerator,
	}
}

func (reader *intervalStatsReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	reader.logger.Debug("Executing read method", zap.String("reader", reader.Name()))

	// Generating pull timestamps
	pullTimestamps := reader.timestampsGenerator.pullTimestamps(reader.lastPullTimestamp, time.Now().UTC())

	var collectedDataPoints []*metadata.MetricsDataPoint

	// Pulling metrics for each generated pull timestamp
	timestampsAmount := len(pullTimestamps)
	for i, pullTimestamp := range pullTimestamps {
		stmt := reader.newPullStatement(pullTimestamp)
		// Latest timestamp for backfilling must be read from actual data(not stale)
		if i == (timestampsAmount-1) && reader.isBackfillExecution() {
			stmt.stalenessRead = false
		}
		dataPoints, err := reader.pull(ctx, stmt)
		if err != nil {
			return nil, err
		}

		collectedDataPoints = append(collectedDataPoints, dataPoints...)
	}

	reader.lastPullTimestamp = pullTimestamps[timestampsAmount-1]

	return collectedDataPoints, nil
}

func (reader *intervalStatsReader) newPullStatement(pullTimestamp time.Time) statsStatement {
	args := statementArgs{
		query:                  reader.metricsMetadata.Query,
		topMetricsQueryMaxRows: reader.topMetricsQueryMaxRows,
		pullTimestamp:          pullTimestamp,
		stalenessRead:          reader.isBackfillExecution(),
	}

	return reader.statement(args)
}

func (reader *intervalStatsReader) isBackfillExecution() bool {
	return reader.timestampsGenerator.isBackfillExecution(reader.lastPullTimestamp)
}
