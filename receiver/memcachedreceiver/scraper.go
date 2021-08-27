// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memcachedreceiver

import (
	"context"
	"time"

	"github.com/grobie/gomemcache/memcache"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper"
)

type memcachedScraper struct {
	client *memcache.Client

	logger *zap.Logger
	config *Config
}

func newMemcachedScraper(
	logger *zap.Logger,
	config *Config,
) scraperhelper.Scraper {
	ms := &memcachedScraper{
		logger: logger,
		config: config,
	}
	return scraperhelper.NewResourceMetricsScraper(config.ID(), ms.scrape)
}

func (r *memcachedScraper) scrape(_ context.Context) (pdata.ResourceMetricsSlice, error) {
	// Init client in scrape method in case there are transient errors in the
	// constructor.
	if r.client == nil {
		var err error
		r.client, err = memcache.New(r.config.Endpoint)
		if err != nil {
			r.client = nil
			return pdata.ResourceMetricsSlice{}, err
		}

		r.client.Timeout = r.config.Timeout
	}

	stats, err := r.client.Stats()
	if err != nil {
		r.logger.Error("Failed to fetch memcached stats", zap.Error(err))
		return pdata.ResourceMetricsSlice{}, err
	}

	now := pdata.NewTimestampFromTime(time.Now())
	metrics := pdata.NewMetrics()
	ilm := metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/memcached")

	for _, stats := range stats {
		for k, v := range stats.Stats {
			switch k {
			case "bytes":
				addIntGauge(ilm.Metrics(), metadata.M.MemcachedBytes.Init, now, parseInt(v))
			case "curr_connections":
				addIntGauge(ilm.Metrics(), metadata.M.MemcachedCurrentConnections.Init, now, parseInt(v))
			case "total_connections":
				addIntSum(ilm.Metrics(), metadata.M.MemcachedTotalConnections.Init, now, parseInt(v))
			case "get_hits":
				addIntSum(ilm.Metrics(), metadata.M.MemcachedGetHits.Init, now, parseInt(v))
			case "get_misses":
				addIntSum(ilm.Metrics(), metadata.M.MemcachedGetMisses.Init, now, parseInt(v))
			}
		}
	}

	return metrics.ResourceMetrics(), nil
}

func addIntGauge(metrics pdata.MetricSlice, initFunc func(pdata.Metric), now pdata.Timestamp, value int64) {
	metric := metrics.AppendEmpty()
	initFunc(metric)
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetIntVal(value)
}

func addIntSum(metrics pdata.MetricSlice, initFunc func(pdata.Metric), now pdata.Timestamp, value int64) {
	metric := metrics.AppendEmpty()
	initFunc(metric)
	dp := metric.Sum().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetIntVal(value)
}
