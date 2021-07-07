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
	start := pdata.TimestampFromTime(time.Now())
	metrics := pdata.NewMetrics()
	ilm := metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/memcached")

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

	now := pdata.TimestampFromTime(time.Now())

	cmdMetric := ilm.Metrics().AppendEmpty()
	metadata.M.MemcachedCommandCount.Init(cmdMetric)
	commandCount := cmdMetric.IntSum().DataPoints()

	networkMetric := ilm.Metrics().AppendEmpty()
	metadata.M.MemcachedNetwork.Init(networkMetric)
	network := networkMetric.IntSum().DataPoints()

	operationCntMetric := ilm.Metrics().AppendEmpty()
	metadata.M.MemcachedOperationCount.Init(operationCntMetric)
	operationCount := operationCntMetric.IntSum().DataPoints()

	hitPercentMetric := ilm.Metrics().AppendEmpty()
	metadata.M.MemcachedOperationHitRatio.Init(hitPercentMetric)
	hitPercentage := hitPercentMetric.DoubleGauge().DataPoints()

	rusageMetric := ilm.Metrics().AppendEmpty()
	metadata.M.MemcachedRusage.Init(rusageMetric)
	rusage := rusageMetric.DoubleGauge().DataPoints()

	for _, stats := range stats {
		for k, v := range stats.Stats {
			switch k {
			case "bytes":
				addIntGauge(ilm.Metrics(), metadata.M.MemcachedBytes.Init, start, now, parseInt(v))
			case "curr_connections":
				addIntGauge(ilm.Metrics(), metadata.M.MemcachedCurrentConnections.Init, start, now, parseInt(v))
			case "total_connections":
				addSum(ilm.Metrics(), metadata.M.MemcachedTotalConnections.Init, start, now, parseInt(v))
			case "cmd_get":
				addLabeledIntMetric(commandCount, []string{metadata.L.Command}, []string{"get"}, start, now, parseInt(v))
			case "cmd_set":
				addLabeledIntMetric(commandCount, []string{metadata.L.Command}, []string{"set"}, start, now, parseInt(v))
			case "cmd_flush":
				addLabeledIntMetric(commandCount, []string{metadata.L.Command}, []string{"flush"}, start, now, parseInt(v))
			case "cmd_meta":
				addLabeledIntMetric(commandCount, []string{metadata.L.Command}, []string{"meta"}, start, now, parseInt(v))
			case "cmd_touch":
				addLabeledIntMetric(commandCount, []string{metadata.L.Command}, []string{"touch"}, start, now, parseInt(v))
			case "curr_items":
				addDoubleGauge(ilm.Metrics(), metadata.M.MemcachedCurrentItems.Init, start, now, parseFloat(v))
			case "evictions":
				addSum(ilm.Metrics(), metadata.M.MemcachedEvictionCount.Init, start, now, parseInt(v))
			case "bytes_read":
				addLabeledIntMetric(network, []string{metadata.L.Direction}, []string{"sent"}, start, now, parseInt(v))
			case "bytes_written":
				addLabeledIntMetric(network, []string{metadata.L.Direction}, []string{"received"}, start, now, parseInt(v))
			case "get_hits":
				addLabeledIntMetric(operationCount, []string{metadata.L.Type, metadata.L.Operation}, []string{"hit", "get"}, start, now, parseInt(v))
				hitVal := parseFloat(v)
				missVal := parseFloat(stats.Stats["get_misses"])
				metricVal := hitVal / (missVal + hitVal) * 100
				addLabeledDoubleMetric(hitPercentage, []string{metadata.L.Operation}, []string{"get"}, start, now, metricVal)
			case "get_misses":
				addLabeledIntMetric(operationCount, []string{metadata.L.Type, metadata.L.Operation}, []string{"miss", "get"}, start, now, parseInt(v))
			case "incr_hits":
				addLabeledIntMetric(operationCount, []string{metadata.L.Type, metadata.L.Operation}, []string{"hit", "increment"}, start, now, parseInt(v))
				hitVal := parseFloat(v)
				missVal := parseFloat(stats.Stats["incr_misses"])
				metricVal := hitVal / (missVal + hitVal) * 100
				addLabeledDoubleMetric(hitPercentage, []string{metadata.L.Operation}, []string{"increment"}, start, now, metricVal)
			case "incr_misses":
				addLabeledIntMetric(operationCount, []string{metadata.L.Type, metadata.L.Operation}, []string{"miss", "increment"}, start, now, parseInt(v))
			case "decr_hits":
				addLabeledIntMetric(operationCount, []string{metadata.L.Type, metadata.L.Operation}, []string{"hit", "decrement"}, start, now, parseInt(v))
				hitVal := parseFloat(v)
				missVal := parseFloat(stats.Stats["decr_misses"])
				metricVal := hitVal / (missVal + hitVal) * 100
				addLabeledDoubleMetric(hitPercentage, []string{metadata.L.Operation}, []string{"decrement"}, start, now, metricVal)
			case "decr_misses":
				addLabeledIntMetric(operationCount, []string{metadata.L.Type, metadata.L.Operation}, []string{"miss", "decrement"}, start, now, parseInt(v))
			case "rusage_system":
				addLabeledDoubleMetric(rusage, []string{metadata.L.UsageType}, []string{"system"}, start, now, parseFloat(v))
			case "rusage_user":
				addLabeledDoubleMetric(rusage, []string{metadata.L.UsageType}, []string{"user"}, start, now, parseFloat(v))
			case "threads":
				addDoubleGauge(ilm.Metrics(), metadata.M.MemcachedThreads.Init, start, now, parseFloat(v))
			}
		}
	}

	return metrics.ResourceMetrics(), nil
}

func addIntGauge(metrics pdata.MetricSlice, initFunc func(pdata.Metric), start pdata.Timestamp, now pdata.Timestamp, value int64) {
	metric := metrics.AppendEmpty()
	initFunc(metric)
	dp := metric.IntGauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(now)
	dp.SetValue(value)
}

func addDoubleGauge(metrics pdata.MetricSlice, initFunc func(pdata.Metric), start pdata.Timestamp, now pdata.Timestamp, value float64) {
	metric := metrics.AppendEmpty()
	initFunc(metric)
	dp := metric.DoubleGauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(now)
	dp.SetValue(value)
}

func addSum(metrics pdata.MetricSlice, initFunc func(pdata.Metric), start pdata.Timestamp, now pdata.Timestamp, value int64) {
	metric := metrics.AppendEmpty()
	initFunc(metric)
	dp := metric.IntSum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(now)
	dp.SetValue(value)
}

func addLabeledIntMetric(dps pdata.IntDataPointSlice, labelType []string, label []string, start pdata.Timestamp, now pdata.Timestamp, value int64) {
	dp := dps.AppendEmpty()
	for i := range labelType {
		dp.LabelsMap().Upsert(labelType[i], label[i])
	}
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(now)
	dp.SetValue(value)
}

func addLabeledDoubleMetric(dps pdata.DoubleDataPointSlice, labelType []string, label []string, start pdata.Timestamp, now pdata.Timestamp, value float64) {
	dp := dps.AppendEmpty()
	for i := range labelType {
		dp.LabelsMap().Upsert(labelType[i], label[i])
	}
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(now)
	dp.SetValue(value)
}
