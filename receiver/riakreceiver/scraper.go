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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/models"
)

const instrumentationLibraryName = "otelcol/riak"

var errClientNotInit = errors.New("client not initialized")

// rabbitmqScraper handles scraping of Riak metrics
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
	metrics := pdata.NewMetrics()
	now := pdata.NewTimestampFromTime(time.Now())
	rms := metrics.ResourceMetrics()

	// Validate we don't attempt to scrape without initializing the client
	if r.client == nil {
		return metrics, errors.New("client not initialized")
	}

	// Get stats for processing
	stats, err := r.client.GetStats(ctx)

	if err != nil {
		return metrics, err
	}

	r.collectStats(stats, now, rms)

	r.logger.Warn(stats.Node)

	return metrics, nil
}

// collectStats collects metrics
func (r *riakScraper) collectStats(stat *models.Stats, now pdata.Timestamp, rms pdata.ResourceMetricsSlice) {
	resourceMetric := rms.AppendEmpty()
	resourceAttrs := resourceMetric.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.RiakNodeName, stat.Node)

	ilms := resourceMetric.InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	//scrape node.operation.count metric
	r.mb.RecordRiakNodeOperationCountDataPoint(now, stat.NodeGets, metadata.AttributeOperation.Get)
	r.mb.RecordRiakNodeOperationCountDataPoint(now, stat.NodePuts, metadata.AttributeOperation.Put)

	//scrape node.operation.time.mean metric
	r.mb.RecordRiakNodeOperationTimeMeanDataPoint(now, stat.NodeGetFsmTimeMean, metadata.AttributeOperation.Get)
	r.mb.RecordRiakNodeOperationTimeMeanDataPoint(now, stat.NodePutFsmTimeMean, metadata.AttributeOperation.Put)

	//scrape node.read_repair.count metric
	r.mb.RecordRiakNodeReadRepairCountDataPoint(now, stat.ReadRepairs)

	//scrape node.memory.limit metric
	r.mb.RecordRiakMemoryLimitDataPoint(now, stat.MemAllocated)

	//scrape vnode.operation.count metric
	r.mb.RecordRiakVnodeOperationCountDataPoint(now, stat.VnodeGets, metadata.AttributeOperation.Get)
	r.mb.RecordRiakVnodeOperationCountDataPoint(now, stat.VnodePuts, metadata.AttributeOperation.Put)

	//scrape vnode.index.operation.count metric
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexReads, metadata.AttributeOperation.Read)
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexWrites, metadata.AttributeOperation.Write)
	r.mb.RecordRiakVnodeIndexOperationCountDataPoint(now, stat.VnodeIndexDeletes, metadata.AttributeOperation.Delete)

	r.mb.Emit(ilms.Metrics())
}
