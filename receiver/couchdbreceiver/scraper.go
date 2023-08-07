// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

type couchdbScraper struct {
	client   client
	config   *Config
	settings component.TelemetrySettings
	rb       *metadata.ResourceBuilder
	mb       *metadata.MetricsBuilder
}

func newCouchdbScraper(settings receiver.CreateSettings, config *Config) *couchdbScraper {
	return &couchdbScraper{
		settings: settings.TelemetrySettings,
		config:   config,
		rb:       metadata.NewResourceBuilder(config.ResourceAttributes),
		mb:       metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
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

func (c *couchdbScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if c.client == nil {
		return pmetric.NewMetrics(), errors.New("no client available")
	}

	localNode := "_local"
	stats, err := c.client.GetStats(localNode)
	if err != nil {
		c.settings.Logger.Error("Failed to fetch couchdb stats",
			zap.String("endpoint", c.config.Endpoint),
			zap.Error(err),
		)
		return pmetric.NewMetrics(), err
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	errs := &scrapererror.ScrapeErrors{}

	c.recordCouchdbAverageRequestTimeDataPoint(now, stats, errs)
	c.recordCouchdbHttpdBulkRequestsDataPoint(now, stats, errs)
	c.recordCouchdbHttpdRequestsDataPoint(now, stats, errs)
	c.recordCouchdbHttpdResponsesDataPoint(now, stats, errs)
	c.recordCouchdbHttpdViewsDataPoint(now, stats, errs)
	c.recordCouchdbDatabaseOpenDataPoint(now, stats, errs)
	c.recordCouchdbFileDescriptorOpenDataPoint(now, stats, errs)
	c.recordCouchdbDatabaseOperationsDataPoint(now, stats, errs)

	c.rb.SetCouchdbNodeName(c.config.Endpoint)
	return c.mb.Emit(metadata.WithResource(c.rb.Emit())), errs.Combine()
}
