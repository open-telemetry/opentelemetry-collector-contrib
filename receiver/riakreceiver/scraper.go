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

package riakreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
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
func newScraper(logger *zap.Logger, cfg *Config, settings component.TelemetrySettings) *riakScraper {
	return &riakScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings,
		mb:       metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
	}
}

// start starts the scraper by creating a new HTTP Client on the scraper
func (r *riakScraper) start(ctx context.Context, host component.Host) (err error) {
	r.client, err = newClient(r.cfg, host, r.settings, r.logger)
	return
}

// scrape collects metrics from the Riak API
func (r *riakScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	// Validate we don't attempt to scrape without initializing the client
	if r.client == nil {
		return pdata.NewMetrics(), errors.New("client not initialized")
	}

	// Get stats for processing
	stats, err := r.client.GetStats(ctx)
	if err != nil {
		return pdata.NewMetrics(), err
	}

	return r.collectStats(stats)
}

// collectStats collects metrics
func (r *riakScraper) collectStats(stat *model.Stats) (pdata.Metrics, error) {
	now := pdata.NewTimestampFromTime(time.Now())
	var errors scrapererror.ScrapeErrors
	//scrape node.operation.count metric
	r.mb.RecordRiakNodeOperationCountDataPoint(now, stat.NodeGets, metadata.AttributeRequest.Get)
	r.mb.RecordRiakNodeOperationCountDataPoint(now, stat.NodePuts, metadata.AttributeRequest.Put)

	//scrape node.operation.time.mean metric
	r.mb.RecordRiakNodeOperationTimeMeanDataPoint(now, stat.NodeGetFsmTimeMean, metadata.AttributeRequest.Get)
	r.mb.RecordRiakNodeOperationTimeMeanDataPoint(now, stat.NodePutFsmTimeMean, metadata.AttributeRequest.Put)

	//scrape node.read_repair.count metric
	r.mb.RecordRiakNodeReadRepairCountDataPoint(now, stat.ReadRepairs)

	//scrape node.memory.limit metric
	r.mb.RecordRiakMemoryLimitDataPoint(now, stat.MemAllocated)

	//scrape vnode.operation.count metric
	r.mb.RecordRiakVnodeOperationCountDataPoint(now, stat.VnodeGets, metadata.AttributeRequest.Get)
	r.mb.RecordRiakVnodeOperationCountDataPoint(now, stat.VnodePuts, metadata.AttributeRequest.Put)

	//scrape vnode.index.operation.count metric
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexReads, metadata.AttributeOperation.Read)
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexWrites, metadata.AttributeOperation.Write)
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexDeletes, metadata.AttributeOperation.Delete)

	return r.mb.Emit(metadata.WithRiakNodeName(stat.Node)), errors.Combine()
}
