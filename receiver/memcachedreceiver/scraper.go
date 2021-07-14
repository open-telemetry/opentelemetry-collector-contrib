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

	commandCount := initLabeledIntSum(ilm.Metrics(), metadata.M.MemcachedCommandCount.Name())
	rUsage := initLabeledDoubleGauge(ilm.Metrics(), metadata.M.MemcachedRusage.Name())
	network := initLabeledIntSum(ilm.Metrics(), metadata.M.MemcachedNetwork.Name())
	operationCount := initLabeledIntSum(ilm.Metrics(), metadata.M.MemcachedOperationCount.Name())
	hitRatio := initLabeledDoubleGauge(ilm.Metrics(), metadata.M.MemcachedOperationHitRatio.Name())
	for _, stats := range stats {
		for k, v := range stats.Stats {
			labels := pdata.NewStringMap()
			switch k {
			case "bytes":
				addIntGauge(ilm.Metrics(), metadata.M.MemcachedBytes.Name(), now, labels, parseInt(v))
			case "curr_connections":
				addIntGauge(ilm.Metrics(), metadata.M.MemcachedCurrentConnections.Name(), now, labels, parseInt(v))
			case "total_connections":
				addIntSum(ilm.Metrics(), metadata.M.MemcachedTotalConnections.Name(), now, labels, parseInt(v))
			case "cmd_get":
				labels.Insert(metadata.L.Command, "get")
				addToIntLabeledMetric(commandCount, now, labels, parseInt(v))
			case "cmd_set":
				labels.Insert(metadata.L.Command, "set")
				addToIntLabeledMetric(commandCount, now, labels, parseInt(v))
			case "cmd_flush":
				labels.Insert(metadata.L.Command, "flush")
				addToIntLabeledMetric(commandCount, now, labels, parseInt(v))
			case "cmd_touch":
				labels.Insert(metadata.L.Command, "touch")
				addToIntLabeledMetric(commandCount, now, labels, parseInt(v))
			case "curr_items":
				addDoubleGauge(ilm.Metrics(), metadata.M.MemcachedCurrentItems.Name(), now, labels, parseFloat(v))
			case "threads":
				addDoubleGauge(ilm.Metrics(), metadata.M.MemcachedThreads.Name(), now, labels, parseFloat(v))
			case "evictions":
				addIntSum(ilm.Metrics(), metadata.M.MemcachedEvictionCount.Name(), now, labels, parseInt(v))
			case "bytes_read":
				labels.Insert(metadata.L.Direction, "received")
				addToIntLabeledMetric(network, now, labels, parseInt(v))
			case "bytes_written":
				labels.Insert(metadata.L.Direction, "sent")
				addToIntLabeledMetric(network, now, labels, parseInt(v))
			case "get_hits":
				labels.Insert(metadata.L.Operation, "get")
				statSlice := stats.Stats
				hits := parseFloat(statSlice["get_hits"])
				misses := parseFloat(statSlice["get_misses"])
				if hits+misses > 0 {
					addToDoubleLabeledMetric(hitRatio, now, labels, (hits / (hits + misses) * 100))
				} else {
					addToDoubleLabeledMetric(hitRatio, now, labels, 0)
				}
				labels.Insert(metadata.L.Type, "hit")
				addToIntLabeledMetric(operationCount, now, labels, parseInt(v))
			case "get_misses":
				labels.Insert(metadata.L.Operation, "get")
				labels.Insert(metadata.L.Type, "miss")
				addToIntLabeledMetric(operationCount, now, labels, parseInt(v))
			case "incr_hits":
				labels.Insert(metadata.L.Operation, "increment")
				statSlice := stats.Stats
				hits := parseFloat(statSlice["incr_hits"])
				misses := parseFloat(statSlice["incr_misses"])
				if hits+misses > 0 {
					addToDoubleLabeledMetric(hitRatio, now, labels, (hits / (hits + misses) * 100))
				} else {
					addToDoubleLabeledMetric(hitRatio, now, labels, 0)
				}
				labels.Insert(metadata.L.Type, "hit")
				addToIntLabeledMetric(operationCount, now, labels, parseInt(v))
			case "incr_misses":
				labels.Insert(metadata.L.Operation, "increment")
				labels.Insert(metadata.L.Type, "miss")
				addToIntLabeledMetric(operationCount, now, labels, parseInt(v))
			case "decr_hits":
				labels.Insert(metadata.L.Operation, "decrement")
				statSlice := stats.Stats
				hits := parseFloat(statSlice["decr_hits"])
				misses := parseFloat(statSlice["decr_misses"])
				if hits+misses > 0 {
					addToDoubleLabeledMetric(hitRatio, now, labels, (hits / (hits + misses) * 100))
				} else {
					addToDoubleLabeledMetric(hitRatio, now, labels, 0)
				}
				labels.Insert(metadata.L.Type, "hit")
				addToIntLabeledMetric(operationCount, now, labels, parseInt(v))
			case "decr_misses":
				labels.Insert(metadata.L.Operation, "decrement")
				labels.Insert(metadata.L.Type, "miss")
				addToIntLabeledMetric(operationCount, now, labels, parseInt(v))
			case "rusage_system":
				labels.Insert(metadata.L.UsageType, "system")
				addToDoubleLabeledMetric(rUsage, now, labels, parseFloat(v))
			case "rusage_user":
				labels.Insert(metadata.L.UsageType, "user")
				addToDoubleLabeledMetric(rUsage, now, labels, parseFloat(v))
			}
		}
	}

	return rms, nil
}

func addIntGauge(ms pdata.MetricSlice, name string, now pdata.Timestamp, labels pdata.StringMap, value int64) {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDataType(pdata.MetricDataTypeIntGauge)
	dp := m.IntGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetValue(value)
	labels.CopyTo(dp.LabelsMap())
}

func addDoubleGauge(ms pdata.MetricSlice, name string, now pdata.Timestamp, labels pdata.StringMap, value float64) {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDataType(pdata.MetricDataTypeDoubleGauge)
	dp := m.DoubleGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetValue(value)
	labels.CopyTo(dp.LabelsMap())
}

func addIntSum(ms pdata.MetricSlice, name string, now pdata.Timestamp, labels pdata.StringMap, value int64) {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDataType(pdata.MetricDataTypeIntSum)
	dp := m.IntSum().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetValue(value)
	labels.CopyTo(dp.LabelsMap())
}

func initLabeledIntSum(ms pdata.MetricSlice, name string) pdata.IntDataPointSlice {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDataType(pdata.MetricDataTypeIntSum)
	return m.IntSum().DataPoints()
}

func initLabeledDoubleGauge(ms pdata.MetricSlice, name string) pdata.DoubleDataPointSlice {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDataType(pdata.MetricDataTypeDoubleGauge)
	return m.DoubleGauge().DataPoints()
}

func addToDoubleLabeledMetric(metric pdata.DoubleDataPointSlice, now pdata.Timestamp, labels pdata.StringMap, value float64) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(now)
	dataPoint.SetValue(value)
	labels.CopyTo(dataPoint.LabelsMap())
}

func addToIntLabeledMetric(metric pdata.IntDataPointSlice, now pdata.Timestamp, labels pdata.StringMap, value int64) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(now)
	dataPoint.SetValue(value)
	labels.CopyTo(dataPoint.LabelsMap())
}
