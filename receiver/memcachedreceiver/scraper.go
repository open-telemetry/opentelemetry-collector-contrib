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

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
)

type memcachedScraper struct {
	client client
	logger *zap.Logger
	config *Config
}

func newMemcachedScraper(
	logger *zap.Logger,
	config *Config,
) memcachedScraper {
	return memcachedScraper{
		logger: logger,
		config: config,
	}
}

func (r *memcachedScraper) scrape(_ context.Context) (pdata.Metrics, error) {
	// Init client in scrape method in case there are transient errors in the
	// constructor.

	if r.client == nil {
		r.client = &memcachedClient{}
		err := r.client.Init(r.config.Endpoint)
		if err != nil {
			r.client = nil
			return pdata.Metrics{}, err
		}
		r.client.SetTimeout(r.config.Timeout)
	}

	stats, err := r.client.Stats()
	if err != nil {
		r.logger.Error("Failed to fetch memcached stats", zap.Error(err))
		return pdata.Metrics{}, err
	}

	now := pdata.NewTimestampFromTime(time.Now())
	md := pdata.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/memcached")

	commandCount := initMetric(ilm.Metrics(), metadata.M.MemcachedCommands).Sum().DataPoints()
	rUsage := initMetric(ilm.Metrics(), metadata.M.MemcachedRusage).Sum().DataPoints()
	network := initMetric(ilm.Metrics(), metadata.M.MemcachedNetwork).Sum().DataPoints()
	operationCount := initMetric(ilm.Metrics(), metadata.M.MemcachedOperations).Sum().DataPoints()
	hitRatio := initMetric(ilm.Metrics(), metadata.M.MemcachedOperationHitRatio).Gauge().DataPoints()
	bytes := initMetric(ilm.Metrics(), metadata.M.MemcachedBytes).Gauge().DataPoints()
	currConn := initMetric(ilm.Metrics(), metadata.M.MemcachedCurrentConnections).Gauge().DataPoints()
	totalConn := initMetric(ilm.Metrics(), metadata.M.MemcachedTotalConnections).Sum().DataPoints()
	currItems := initMetric(ilm.Metrics(), metadata.M.MemcachedCurrentItems).Sum().DataPoints()
	threads := initMetric(ilm.Metrics(), metadata.M.MemcachedThreads).Gauge().DataPoints()
	evictions := initMetric(ilm.Metrics(), metadata.M.MemcachedEvictions).Sum().DataPoints()

	for _, stats := range stats {
		for k, v := range stats.Stats {
			attributes := pdata.NewAttributeMap()
			switch k {
			case "bytes":
				addToMetric(bytes, attributes, parseInt(v), now)
			case "curr_connections":
				addToMetric(currConn, attributes, parseInt(v), now)
			case "total_connections":
				addToMetric(totalConn, attributes, parseInt(v), now)
			case "cmd_get":
				attributes.Insert(metadata.L.Command, pdata.NewAttributeValueString("get"))
				addToMetric(commandCount, attributes, parseInt(v), now)
			case "cmd_set":
				attributes.Insert(metadata.L.Command, pdata.NewAttributeValueString("set"))
				addToMetric(commandCount, attributes, parseInt(v), now)
			case "cmd_flush":
				attributes.Insert(metadata.L.Command, pdata.NewAttributeValueString("flush"))
				addToMetric(commandCount, attributes, parseInt(v), now)
			case "cmd_touch":
				attributes.Insert(metadata.L.Command, pdata.NewAttributeValueString("touch"))
				addToMetric(commandCount, attributes, parseInt(v), now)
			case "curr_items":
				addToMetric(currItems, attributes, parseFloat(v), now)
			case "threads":
				addToMetric(threads, attributes, parseFloat(v), now)
			case "evictions":
				addToMetric(evictions, attributes, parseInt(v), now)
			case "bytes_read":
				attributes.Insert(metadata.L.Direction, pdata.NewAttributeValueString("received"))
				addToMetric(network, attributes, parseInt(v), now)
			case "bytes_written":
				attributes.Insert(metadata.L.Direction, pdata.NewAttributeValueString("sent"))
				addToMetric(network, attributes, parseInt(v), now)
			case "get_hits":
				attributes.Insert(metadata.L.Operation, pdata.NewAttributeValueString("get"))
				statSlice := stats.Stats
				hits := parseFloat(statSlice["get_hits"])
				misses := parseFloat(statSlice["get_misses"])
				if hits+misses > 0 {
					addToMetric(hitRatio, attributes, (hits / (hits + misses) * 100), now)
				} else {
					addToMetric(hitRatio, attributes, 0, now)
				}
				attributes.Insert(metadata.L.Type, pdata.NewAttributeValueString("hit"))
				addToMetric(operationCount, attributes, parseInt(v), now)
			case "get_misses":
				attributes.Insert(metadata.L.Operation, pdata.NewAttributeValueString("get"))
				attributes.Insert(metadata.L.Type, pdata.NewAttributeValueString("miss"))
				addToMetric(operationCount, attributes, parseInt(v), now)
			case "incr_hits":
				attributes.Insert(metadata.L.Operation, pdata.NewAttributeValueString("increment"))
				statSlice := stats.Stats
				hits := parseFloat(statSlice["incr_hits"])
				misses := parseFloat(statSlice["incr_misses"])
				if hits+misses > 0 {
					addToMetric(hitRatio, attributes, (hits / (hits + misses) * 100), now)
				} else {
					addToMetric(hitRatio, attributes, 0, now)
				}
				attributes.Insert(metadata.L.Type, pdata.NewAttributeValueString("hit"))
				addToMetric(operationCount, attributes, parseInt(v), now)
			case "incr_misses":
				attributes.Insert(metadata.L.Operation, pdata.NewAttributeValueString("increment"))
				attributes.Insert(metadata.L.Type, pdata.NewAttributeValueString("miss"))
				addToMetric(operationCount, attributes, parseInt(v), now)
			case "decr_hits":
				attributes.Insert(metadata.L.Operation, pdata.NewAttributeValueString("decrement"))
				statSlice := stats.Stats
				hits := parseFloat(statSlice["decr_hits"])
				misses := parseFloat(statSlice["decr_misses"])
				if hits+misses > 0 {
					addToMetric(hitRatio, attributes, (hits / (hits + misses) * 100), now)
				} else {
					addToMetric(hitRatio, attributes, 0, now)
				}
				attributes.Insert(metadata.L.Type, pdata.NewAttributeValueString("hit"))
				addToMetric(operationCount, attributes, parseInt(v), now)
			case "decr_misses":
				attributes.Insert(metadata.L.Operation, pdata.NewAttributeValueString("decrement"))
				attributes.Insert(metadata.L.Type, pdata.NewAttributeValueString("miss"))
				addToMetric(operationCount, attributes, parseInt(v), now)
			case "rusage_system":
				attributes.Insert(metadata.L.UsageType, pdata.NewAttributeValueString("system"))
				addToMetric(rUsage, attributes, parseFloat(v), now)
			case "rusage_user":
				attributes.Insert(metadata.L.UsageType, pdata.NewAttributeValueString("user"))
				addToMetric(rUsage, attributes, parseFloat(v), now)
			}
		}
	}

	return md, nil
}

func initMetric(ms pdata.MetricSlice, mi metadata.MetricIntf) pdata.Metric {
	m := ms.AppendEmpty()
	mi.Init(m)
	return m
}

// addToMetric adds a datapoint to a NumberDataPointSlice
func addToMetric(metric pdata.NumberDataPointSlice, attributes pdata.AttributeMap, value interface{}, ts pdata.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(ts)
	switch typedVal := value.(type) {
	case int64:
		dataPoint.SetIntVal(typedVal)
	case float64:
		dataPoint.SetDoubleVal(typedVal)
	}
	if attributes.Len() > 0 {
		attributes.CopyTo(dataPoint.Attributes())
	}
}
