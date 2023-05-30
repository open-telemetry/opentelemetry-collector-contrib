// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package riakreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/model"
)

var errClientNotInit = errors.New("client not initialized")

// riakScraper handles scraping of Riak metrics
type riakScraper struct {
	logger   *zap.Logger
	cfg      *Config
	settings component.TelemetrySettings
	client   client
	mb       *metadata.MetricsBuilder
}

// newScraper creates a new scraper
func newScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *riakScraper {
	return &riakScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

// start starts the scraper by creating a new HTTP Client on the scraper
func (r *riakScraper) start(ctx context.Context, host component.Host) (err error) {
	r.client, err = newClient(r.cfg, host, r.settings, r.logger)
	return
}

// scrape collects metrics from the Riak API
func (r *riakScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	// Validate we don't attempt to scrape without initializing the client
	if r.client == nil {
		return pmetric.NewMetrics(), errors.New("client not initialized")
	}

	// Get stats for processing
	stats, err := r.client.GetStats(ctx)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	return r.collectStats(stats)
}

// collectStats collects metrics
func (r *riakScraper) collectStats(stat *model.Stats) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	var errors scrapererror.ScrapeErrors
	// scrape node.operation.count metric
	r.mb.RecordRiakNodeOperationCountDataPoint(now, stat.NodeGets, metadata.AttributeRequestGet)
	r.mb.RecordRiakNodeOperationCountDataPoint(now, stat.NodePuts, metadata.AttributeRequestPut)

	// scrape node.operation.time.mean metric
	r.mb.RecordRiakNodeOperationTimeMeanDataPoint(now, stat.NodeGetFsmTimeMean, metadata.AttributeRequestGet)
	r.mb.RecordRiakNodeOperationTimeMeanDataPoint(now, stat.NodePutFsmTimeMean, metadata.AttributeRequestPut)

	// scrape node.read_repair.count metric
	r.mb.RecordRiakNodeReadRepairCountDataPoint(now, stat.ReadRepairs)

	// scrape node.memory.limit metric
	r.mb.RecordRiakMemoryLimitDataPoint(now, stat.MemAllocated)

	// scrape vnode.operation.count metric
	r.mb.RecordRiakVnodeOperationCountDataPoint(now, stat.VnodeGets, metadata.AttributeRequestGet)
	r.mb.RecordRiakVnodeOperationCountDataPoint(now, stat.VnodePuts, metadata.AttributeRequestPut)

	// scrape vnode.index.operation.count metric
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexReads, metadata.AttributeOperationRead)
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexWrites, metadata.AttributeOperationWrite)
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexDeletes, metadata.AttributeOperationDelete)

	return r.mb.Emit(metadata.WithRiakNodeName(stat.Node)), errors.Combine()
}
