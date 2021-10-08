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
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type currentStatsReader struct {
	logger                 *zap.Logger
	database               *datasource.Database
	metricsMetadata        *metadata.MetricsMetadata
	topMetricsQueryMaxRows int
	statement              func(args statementArgs) spanner.Statement
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

func (reader *currentStatsReader) Read(ctx context.Context) ([]pdata.Metrics, error) {
	reader.logger.Debug("Executing read method", zap.String("reader", reader.Name()))

	stmt := reader.newPullStatement()

	return reader.pull(ctx, stmt)
}

func (reader *currentStatsReader) newPullStatement() spanner.Statement {
	args := statementArgs{
		query:                  reader.metricsMetadata.Query,
		topMetricsQueryMaxRows: reader.topMetricsQueryMaxRows,
	}

	return reader.statement(args)
}

func (reader *currentStatsReader) pull(ctx context.Context, stmt spanner.Statement) ([]pdata.Metrics, error) {
	rowsIterator := reader.database.Client().Single().WithTimestampBound(spanner.ExactStaleness(15*time.Second)).Query(ctx, stmt)
	defer rowsIterator.Stop()

	var collectedMetrics []pdata.Metrics

	for {
		row, err := rowsIterator.Next()
		if err != nil {
			if err == iterator.Done {
				return collectedMetrics, nil
			}
			return nil, fmt.Errorf("query %q failed with error: %w", stmt.SQL, err)
		}

		rowMetrics, err := reader.metricsMetadata.RowToMetrics(reader.database.DatabaseID(), row)
		if err != nil {
			return nil, fmt.Errorf("query %q failed with error: %w", stmt.SQL, err)
		}

		collectedMetrics = append(collectedMetrics, rowMetrics...)
	}
}
