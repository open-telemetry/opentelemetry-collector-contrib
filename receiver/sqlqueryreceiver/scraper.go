// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"go.uber.org/zap"
)

type sqlQueryScraper struct {
	sqlclient client
	logger    *zap.Logger
	config    *Config
}

func newSqlQueryScraper(
	settings component.ReceiverCreateSettings,
	config *Config,
) *sqlQueryScraper {
	return &sqlQueryScraper{
		logger: settings.Logger,
		config: config,
	}
}

// interface methods
// start starts the scraper by initializing the db client connection.
func (m *sqlQueryScraper) start(_ context.Context, host component.Host) error {
	queryClient := &sqlQueryClient{
		driver:  m.config.Driver,
		connStr: m.config.DataSource,
		queries: m.config.Queries,
	}
	err := queryClient.Connect()
	if err != nil {
		return err
	}
	m.sqlclient = queryClient

	return nil
}

// shutdown closes the db connection
func (m *sqlQueryScraper) shutdown(context.Context) error {
	if m.sqlclient == nil {
		return nil
	}
	return m.sqlclient.Close()
}

// scrape scrapes the defined queries against the db connection
func (m *sqlQueryScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if m.sqlclient == nil {
		return pmetric.Metrics{}, errors.New("failed to connect to http client")
	}

	md := pmetric.NewMetrics()
	rs := md.ResourceMetrics().AppendEmpty()
	rs.SetSchemaUrl(conventions.SchemaURL)
	resourceAttr := rs.Resource().Attributes()
	resourceAttr.UpsertString("driver", m.config.Driver)
	ils := rs.ScopeMetrics().AppendEmpty()

	results, queryErr := m.sqlclient.getQueries()
	if queryErr != nil {
		return pmetric.Metrics{}, queryErr
	}

	// loop through query results (slice of stat{})
	for _, m := range results {
		metric := ils.Metrics().AppendEmpty()
		// set metric metadata
		metric.SetName(m.name)
		metric.SetDataType(pmetric.MetricDataTypeSum)

		// set datapoints
		sum := metric.Sum()
		sum.SetIsMonotonic(m.isMonotonic)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		dp := sum.DataPoints().AppendEmpty()
		dp.SetDoubleVal(m.value)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().UTC()))

		for k, v := range m.dimensions {
			dp.Attributes().UpsertString(k, v)
		}
	}

	return md, nil

}
