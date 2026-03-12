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
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

type couchdbScraper struct {
	client             client
	config             *Config
	settings           component.TelemetrySettings
	mb                 *metadata.MetricsBuilder
	resourceAttributes *couchDbResourceAttributes
}

type couchDbResourceAttributes struct {
	couchDbVersion    string
	serviceInstanceID string
	serviceName       string
}

func newCouchdbScraper(settings receiver.Settings, config *Config) *couchdbScraper {
	return &couchdbScraper{
		settings: settings.TelemetrySettings,
		config:   config,
		mb:       metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		resourceAttributes: &couchDbResourceAttributes{
			couchDbVersion:    "",
			serviceInstanceID: "",
			serviceName:       "",
		},
	}
}

func (c *couchdbScraper) start(ctx context.Context, host component.Host) error {
	httpClient, err := newCouchDBClient(ctx, c.config, host, c.settings)
	if err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}
	c.client = httpClient
	err = c.scrapeResourceAttributes()
	if err != nil {
		return err
	}
	return nil
}

func (c *couchdbScraper) scrapeResourceAttributes() error {
	localNode := "_local"

	rootInfo, err := c.client.GetRootInfo()
	if err != nil {
		c.settings.Logger.Error("Failed to fetch couchdb root info",
			zap.String("endpoint", c.config.Endpoint),
			zap.Error(err),
		)
	} else {
		if uuid, ok := rootInfo["uuid"].(string); ok {
			c.resourceAttributes.serviceInstanceID = uuid
		}
		if version, ok := rootInfo["version"].(string); ok {
			c.resourceAttributes.couchDbVersion = version
		}
	}

	nodeInfo, err := c.client.GetNodeInfo(localNode)
	if err != nil {
		c.settings.Logger.Error("Failed to fetch couchdb node info for the local node", zap.String("endpoint", c.config.Endpoint), zap.Error(err))
	} else if name, ok := nodeInfo["name"].(string); ok {
		c.resourceAttributes.serviceName = name
	}

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

	// Refresh resource attributes if we were unable to get them when starting the scraper
	if c.resourceAttributes.serviceInstanceID == "" {
		err = c.scrapeResourceAttributes()
		if err != nil {
			c.settings.Logger.Error("Unable to refetch couchdb info for resource attributes",
				zap.String("endpoint", c.config.Endpoint),
				zap.Error(err),
			)
		}
	}

	rb := c.mb.NewResourceBuilder()
	rb.SetCouchdbNodeName(c.config.Endpoint)
	rb.SetServiceInstanceID(c.resourceAttributes.serviceInstanceID)
	rb.SetServiceName(c.resourceAttributes.serviceName)
	rb.SetCouchdbVersion(c.resourceAttributes.couchDbVersion)

	return c.mb.Emit(metadata.WithResource(rb.Emit())), errs.Combine()
}
