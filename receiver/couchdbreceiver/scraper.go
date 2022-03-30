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
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

type couchdbScraper struct {
	client   client
	config   *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
}

func newCouchdbScraper(settings component.TelemetrySettings, config *Config) *couchdbScraper {
	return &couchdbScraper{
		settings: settings,
		config:   config,
		mb:       metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
	}
}

func (c *couchdbScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := newCouchDBClient(c.config, host, c.settings)
	if err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}
	c.client = httpClient
	return nil
}

func (c *couchdbScraper) scrape(context.Context) (pdata.Metrics, error) {
	if c.client == nil {
		return pdata.NewMetrics(), errors.New("no client available")
	}

	localNode := "_local"
	stats, err := c.client.GetStats(localNode)
	if err != nil {
		c.settings.Logger.Error("Failed to fetch couchdb stats",
			zap.String("endpoint", c.config.Endpoint),
			zap.Error(err),
		)
		return pdata.NewMetrics(), err
	}

	now := pdata.NewTimestampFromTime(time.Now())

	var errors scrapererror.ScrapeErrors
	c.recordCouchdbAverageRequestTimeDataPoint(now, stats, errors)
	c.recordCouchdbHttpdBulkRequestsDataPoint(now, stats, errors)
	c.recordCouchdbHttpdRequestsDataPoint(now, stats, errors)
	c.recordCouchdbHttpdResponsesDataPoint(now, stats, errors)
	c.recordCouchdbHttpdViewsDataPoint(now, stats, errors)
	c.recordCouchdbDatabaseOpenDataPoint(now, stats, errors)
	c.recordCouchdbFileDescriptorOpenDataPoint(now, stats, errors)
	c.recordCouchdbDatabaseOperationsDataPoint(now, stats, errors)

	return c.mb.Emit(metadata.WithCouchdbNodeName(c.config.Endpoint)), errors.Combine()
}
