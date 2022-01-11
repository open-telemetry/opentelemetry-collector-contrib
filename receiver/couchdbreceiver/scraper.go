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

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

type couchdbScraper struct {
	client client
	config *Config
	logger *zap.Logger
	mb     *metadata.MetricsBuilder
}

func newCouchdbScraper(logger *zap.Logger, config *Config) *couchdbScraper {
	return &couchdbScraper{
		logger: logger,
		config: config,
		mb:     metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
	}
}

func (c *couchdbScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := newCouchDBClient(c.config, host, c.logger)
	if err != nil {
		c.logger.Error("failed to connect to couchdb", zap.Error(err))
		return err
	}
	c.client = httpClient
	return nil
}

func (c *couchdbScraper) scrape(context.Context) (pdata.Metrics, error) {
	if c.client == nil {
		err := errors.New("failed to connect to couchdb client")
		c.logger.Error("Failed to connect to couchdb client",
			zap.String("endpoint", c.config.Endpoint),
			zap.Error(err),
		)
		return pdata.NewMetrics(), err
	}

	return c.getResourceMetrics()
}

func (c *couchdbScraper) getResourceMetrics() (pdata.Metrics, error) {
	stats, err := c.client.GetStats("_local")
	if err != nil {
		c.logger.Error("Failed to fetch couchdb stats",
			zap.String("endpoint", c.config.Endpoint),
			zap.Error(err),
		)
		return pdata.NewMetrics(), err
	}

	md := pdata.NewMetrics()
	c.appendMetrics(stats, md.ResourceMetrics())
	return md, nil
}

func (c *couchdbScraper) appendMetrics(stats map[string]interface{}, rms pdata.ResourceMetricsSlice) {
	now := pdata.NewTimestampFromTime(time.Now())
	rm := pdata.NewResourceMetrics()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/couchdb")

	rm.Resource().Attributes().UpsertString(metadata.A.CouchdbNodeName, c.config.Endpoint)

	c.recordCouchdbAverageRequestTimeDataPoint(now, stats)
	c.recordCouchdbHttpdBulkRequestsDataPoint(now, stats)
	c.recordCouchdbHttpdRequestsDataPoint(now, stats)
	c.recordCouchdbHttpdResponsesDataPoint(now, stats)
	c.recordCouchdbHttpdViewsDataPoint(now, stats)
	c.recordCouchdbDatabaseOpenDataPoint(now, stats)
	c.recordCouchdbFileDescriptorOpenDataPoint(now, stats)
	c.recordCouchdbDatabaseOperationsDataPoint(now, stats)

	c.mb.Emit(ilm.Metrics())
	if ilm.Metrics().Len() > 0 {
		rm.CopyTo(rms.AppendEmpty())
	}
}
