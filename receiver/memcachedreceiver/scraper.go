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
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
)

type memcachedScraper struct {
	client client
	logger *zap.Logger
	config *Config
	now    pdata.Timestamp
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

func (r *memcachedScraper) start(_ context.Context, host component.Host) error {
	r.client = newMemcachedClient(r.config)
	return nil
}

func (r *memcachedScraper) scrape(_ context.Context) (pdata.Metrics, error) {
	// Init client in scrape method in case there are transient errors in the
	// constructor.
	err := r.client.Init()
	if err != nil {
		return pdata.Metrics{}, err
	}

	allServerStats, err := r.client.Stats()
	if err != nil {
		r.logger.Error("Failed to fetch memcached stats", zap.Error(err))
		return pdata.Metrics{}, err
	}

	r.now = pdata.NewTimestampFromTime(time.Now())
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

	for _, stats := range allServerStats {
		for k, v := range stats.Stats {
			attributes := pdata.NewAttributeMap()
			switch k {
			case "bytes":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(bytes, attributes, parsedV)
				}
			case "curr_connections":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(currConn, attributes, parsedV)
				}
			case "total_connections":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(totalConn, attributes, parsedV)
				}
			case "cmd_get":
				attributes.Insert(metadata.A.Command, pdata.NewAttributeValueString("get"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV)
				}
			case "cmd_set":
				attributes.Insert(metadata.A.Command, pdata.NewAttributeValueString("set"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV)
				}
			case "cmd_flush":
				attributes.Insert(metadata.A.Command, pdata.NewAttributeValueString("flush"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV)
				}
			case "cmd_touch":
				attributes.Insert(metadata.A.Command, pdata.NewAttributeValueString("touch"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV)
				}
			case "curr_items":
				if parsedV, ok := r.parseFloat(k, v); ok {
					r.addToDoubleMetric(currItems, attributes, parsedV)
				}

			case "threads":
				if parsedV, ok := r.parseFloat(k, v); ok {
					r.addToDoubleMetric(threads, attributes, parsedV)
				}

			case "evictions":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(evictions, attributes, parsedV)
				}
			case "bytes_read":
				attributes.Insert(metadata.A.Direction, pdata.NewAttributeValueString("received"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(network, attributes, parsedV)
				}
			case "bytes_written":
				attributes.Insert(metadata.A.Direction, pdata.NewAttributeValueString("sent"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(network, attributes, parsedV)
				}
			case "get_hits":
				attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString("get"))
				attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("hit"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV)
				}
			case "get_misses":
				attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString("get"))
				attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("miss"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV)
				}
			case "incr_hits":
				attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString("increment"))
				attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("hit"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV)
				}
			case "incr_misses":
				attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString("increment"))
				attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("miss"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV)
				}
			case "decr_hits":
				attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString("decrement"))
				attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("hit"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV)
				}
			case "decr_misses":
				attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString("decrement"))
				attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("miss"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV)
				}
			case "rusage_system":
				attributes.Insert(metadata.A.State, pdata.NewAttributeValueString("system"))
				if parsedV, ok := r.parseFloat(k, v); ok {
					r.addToDoubleMetric(rUsage, attributes, parsedV)
				}

			case "rusage_user":
				attributes.Insert(metadata.A.State, pdata.NewAttributeValueString("user"))
				if parsedV, ok := r.parseFloat(k, v); ok {
					r.addToDoubleMetric(rUsage, attributes, parsedV)
				}
			}
		}

		// Calculated Metrics
		r.calculateHitRatio("increment", "incr_hits", "incr_misses", stats.Stats, hitRatio)
		r.calculateHitRatio("decrement", "decr_hits", "decr_misses", stats.Stats, hitRatio)
		r.calculateHitRatio("get", "get_hits", "get_misses", stats.Stats, hitRatio)
	}
	return md, nil
}

func initMetric(ms pdata.MetricSlice, mi metadata.MetricIntf) pdata.Metric {
	m := ms.AppendEmpty()
	mi.Init(m)
	return m
}

func (r *memcachedScraper) calculateHitRatio(operation, hitKey, missKey string, stats map[string]string, hitRatioMetric pdata.NumberDataPointSlice) {
	attributes := pdata.NewAttributeMap()
	attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString(operation))
	hits, hitOk := r.parseFloat(hitKey, stats[hitKey])
	misses, missOk := r.parseFloat(missKey, stats[missKey])
	if !missOk || !hitOk {
		return
	}

	if misses+hits == 0 {
		r.addToDoubleMetric(hitRatioMetric, attributes, 0)
		return
	}
	r.addToDoubleMetric(hitRatioMetric, attributes, (hits / (hits + misses) * 100))
}

// parseInt converts string to int64.
func (r *memcachedScraper) parseInt(key, value string) (int64, bool) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		r.logInvalid("int", key, value)
		return 0, false
	}
	return i, true
}

// parseFloat converts string to float64.
func (r *memcachedScraper) parseFloat(key, value string) (float64, bool) {
	i, err := strconv.ParseFloat(value, 64)
	if err != nil {
		r.logInvalid("float", key, value)
		return 0, false
	}
	return i, true
}

func (r *memcachedScraper) logInvalid(expectedType, key, value string) {
	r.logger.Info(
		"invalid value",
		zap.String("expectedType", expectedType),
		zap.String("key", key),
		zap.String("value", value),
	)
}

func (r *memcachedScraper) addToDoubleMetric(metric pdata.NumberDataPointSlice, attributes pdata.AttributeMap, value float64) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(r.now)
	dataPoint.SetDoubleVal(value)
	if attributes.Len() > 0 {
		attributes.CopyTo(dataPoint.Attributes())
	}
}

func (r *memcachedScraper) addToIntMetric(metric pdata.NumberDataPointSlice, attributes pdata.AttributeMap, value int64) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(r.now)
	dataPoint.SetIntVal(value)
	if attributes.Len() > 0 {
		attributes.CopyTo(dataPoint.Attributes())
	}
}
