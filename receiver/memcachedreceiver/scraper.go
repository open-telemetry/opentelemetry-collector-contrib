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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/simple"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
)

type memcachedScraper struct {
	client *memcache.Client

	logger *zap.Logger
	config *Config
}

func newMemcachedScraper(
	logger *zap.Logger,
	config *Config,
) scraperhelper.ResourceMetricsScraper {
	ms := &memcachedScraper{
		logger: logger,
		config: config,
	}
	return scraperhelper.NewResourceMetricsScraper(config.Name(), ms.scrape)
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

	metrics := simple.Metrics{
		Metrics:                    pdata.NewMetrics(),
		Timestamp:                  time.Now(),
		MetricFactoriesByName:      metadata.M.FactoriesByName(),
		InstrumentationLibraryName: "otelcol/memcached",
	}

	stats, err := r.client.Stats()
	if err != nil {
		r.logger.Error("Failed to fetch memcached stats", zap.Error(err))
		return pdata.ResourceMetricsSlice{}, err
	}

	for _, stats := range stats {
		for k, v := range stats.Stats {
			switch k {
			case "bytes":
				metrics.AddGaugeDataPoint(metadata.M.MemcachedBytes.Name(), parseInt(v))
			case "curr_connections":
				metrics.AddGaugeDataPoint(metadata.M.MemcachedCurrentConnections.Name(), parseInt(v))
			case "total_connections":
				metrics.AddSumDataPoint(metadata.M.MemcachedTotalConnections.Name(), parseInt(v))
			case "get_hits":
				metrics.AddSumDataPoint(metadata.M.MemcachedGetHits.Name(), parseInt(v))
			case "get_misses":
				metrics.AddSumDataPoint(metadata.M.MemcachedGetMisses.Name(), parseInt(v))
			}
		}
	}

	return metrics.Metrics.ResourceMetrics(), nil
}
