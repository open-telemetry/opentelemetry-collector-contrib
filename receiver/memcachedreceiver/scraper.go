// Copyright 2020, ObservIQ
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

	rms := pdata.NewResourceMetricsSlice()
	ilm := rms.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/memcached")
	now := pdata.TimestampFromTime(time.Now())

	commandCount := initMetric(ilm.Metrics(), metadata.M.MemcachedCommandCount).IntSum().DataPoints()
	rUsage := initMetric(ilm.Metrics(), metadata.M.MemcachedRusage).Gauge().DataPoints()
	network := initMetric(ilm.Metrics(), metadata.M.MemcachedNetwork).IntSum().DataPoints()
	operationCount := initMetric(ilm.Metrics(), metadata.M.MemcachedOperationCount).IntSum().DataPoints()
	hitRatio := initMetric(ilm.Metrics(), metadata.M.MemcachedOperationHitRatio).Gauge().DataPoints()
	bytes := initMetric(ilm.Metrics(), metadata.M.MemcachedBytes).IntGauge().DataPoints()
	currConn := initMetric(ilm.Metrics(), metadata.M.MemcachedCurrentConnections).IntGauge().DataPoints()
	totalConn := initMetric(ilm.Metrics(), metadata.M.MemcachedTotalConnections).IntSum().DataPoints()
	currItems := initMetric(ilm.Metrics(), metadata.M.MemcachedCurrentItems).Gauge().DataPoints()
	threads := initMetric(ilm.Metrics(), metadata.M.MemcachedThreads).Gauge().DataPoints()
	evictions := initMetric(ilm.Metrics(), metadata.M.MemcachedEvictionCount).IntSum().DataPoints()

	for _, stats := range stats {
		for k, v := range stats.Stats {
			labels := pdata.NewAttributeMap()
			switch k {
			case "bytes":
				addToIntMetric(bytes, labels, parseInt(v), now)
			case "curr_connections":
				addToIntMetric(currConn, labels, parseInt(v), now)
			case "total_connections":
				addToIntMetric(totalConn, labels, parseInt(v), now)
			case "cmd_get":
				labels.Insert(metadata.L.Command, "get")
				addToIntMetric(commandCount, labels, parseInt(v), now)
			case "cmd_set":
				labels.Insert(metadata.L.Command, "set")
				addToIntMetric(commandCount, labels, parseInt(v), now)
			case "cmd_flush":
				labels.Insert(metadata.L.Command, "flush")
				addToIntMetric(commandCount, labels, parseInt(v), now)
			case "cmd_touch":
				labels.Insert(metadata.L.Command, "touch")
				addToIntMetric(commandCount, labels, parseInt(v), now)
			case "curr_items":
				addToMetric(currItems, labels, parseFloat(v), now)
			case "threads":
				addToMetric(threads, labels, parseFloat(v), now)
			case "evictions":
				addToIntMetric(evictions, labels, parseInt(v), now)
			case "bytes_read":
				labels.Insert(metadata.L.Direction, "received")
				addToIntMetric(network, labels, parseInt(v), now)
			case "bytes_written":
				labels.Insert(metadata.L.Direction, "sent")
				addToIntMetric(network, labels, parseInt(v), now)
			case "get_hits":
				labels.Insert(metadata.L.Operation, "get")
				statSlice := stats.Stats
				hits := parseFloat(statSlice["get_hits"])
				misses := parseFloat(statSlice["get_misses"])
				if hits+misses > 0 {
					addToMetric(hitRatio, labels, (hits / (hits + misses) * 100), now)
				} else {
					addToMetric(hitRatio, labels, 0, now)
				}
				labels.Insert(metadata.L.Type, "hit")
				addToIntMetric(operationCount, labels, parseInt(v), now)
			case "get_misses":
				labels.Insert(metadata.L.Operation, "get")
				labels.Insert(metadata.L.Type, "miss")
				addToIntMetric(operationCount, labels, parseInt(v), now)
			case "incr_hits":
				labels.Insert(metadata.L.Operation, "increment")
				statSlice := stats.Stats
				hits := parseFloat(statSlice["incr_hits"])
				misses := parseFloat(statSlice["incr_misses"])
				if hits+misses > 0 {
					addToMetric(hitRatio, labels, (hits / (hits + misses) * 100), now)
				} else {
					addToMetric(hitRatio, labels, 0, now)
				}
				labels.Insert(metadata.L.Type, "hit")
				addToIntMetric(operationCount, labels, parseInt(v), now)
			case "incr_misses":
				labels.Insert(metadata.L.Operation, "increment")
				labels.Insert(metadata.L.Type, "miss")
				addToIntMetric(operationCount, labels, parseInt(v), now)
			case "decr_hits":
				labels.Insert(metadata.L.Operation, "decrement")
				statSlice := stats.Stats
				hits := parseFloat(statSlice["decr_hits"])
				misses := parseFloat(statSlice["decr_misses"])
				if hits+misses > 0 {
					addToMetric(hitRatio, labels, (hits / (hits + misses) * 100), now)
				} else {
					addToMetric(hitRatio, labels, 0, now)
				}
				labels.Insert(metadata.L.Type, "hit")
				addToIntMetric(operationCount, labels, parseInt(v), now)
			case "decr_misses":
				labels.Insert(metadata.L.Operation, "decrement")
				labels.Insert(metadata.L.Type, "miss")
				addToIntMetric(operationCount, labels, parseInt(v), now)
			case "rusage_system":
				labels.Insert(metadata.L.UsageType, "system")
				addToMetric(rUsage, labels, parseFloat(v), now)
			case "rusage_user":
				labels.Insert(metadata.L.UsageType, "user")
				addToMetric(rUsage, labels, parseFloat(v), now)
			}
		}
	}

	return rms, nil
}

func initMetric(ms pdata.MetricSlice, mi metadata.MetricIntf) pdata.Metric {
	m := ms.AppendEmpty()
	mi.Init(m)
	return m
}

// addToMetric adds and labels a double gauge datapoint to a metricslice.
func addToMetric(metric pdata.NumberDataPointSlice, labels pdata.StringMap, value float64, ts pdata.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(ts)
	dataPoint.SetValue(value)
	if labels.Len() > 0 {
		labels.CopyTo(dataPoint.LabelsMap())
	}
}

// addToIntMetric adds and labels a int sum datapoint to metricslice.
func addToIntMetric(metric pdata.IntDataPointSlice, labels pdata.StringMap, value int64, ts pdata.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(ts)
	dataPoint.SetValue(value)
	if labels.Len() > 0 {
		labels.CopyTo(dataPoint.LabelsMap())
	}
}
