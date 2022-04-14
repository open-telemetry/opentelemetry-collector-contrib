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

package memcachedreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver"

import (
	"context"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
)

type memcachedScraper struct {
	logger    *zap.Logger
	config    *Config
	newClient newMemcachedClientFunc
}

func newMemcachedScraper(
	logger *zap.Logger,
	config *Config,
) memcachedScraper {
	return memcachedScraper{
		logger:    logger,
		config:    config,
		newClient: newMemcachedClient,
	}
}

func (r *memcachedScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	// Init client in scrape method in case there are transient errors in the
	// constructor.
	statsClient, err := r.newClient(r.config.Endpoint, r.config.Timeout)
	if err != nil {
		r.logger.Error("Failed to estalbish client", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	allServerStats, err := statsClient.Stats()
	if err != nil {
		r.logger.Error("Failed to fetch memcached stats", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	md := pmetric.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("otelcol/memcached")

	commandCount := initMetric(ilm.Metrics(), metadata.M.MemcachedCommands).Sum().DataPoints()
	rUsage := initMetric(ilm.Metrics(), metadata.M.MemcachedCPUUsage).Sum().DataPoints()
	network := initMetric(ilm.Metrics(), metadata.M.MemcachedNetwork).Sum().DataPoints()
	operationCount := initMetric(ilm.Metrics(), metadata.M.MemcachedOperations).Sum().DataPoints()
	hitRatio := initMetric(ilm.Metrics(), metadata.M.MemcachedOperationHitRatio).Gauge().DataPoints()
	bytes := initMetric(ilm.Metrics(), metadata.M.MemcachedBytes).Gauge().DataPoints()
	currConn := initMetric(ilm.Metrics(), metadata.M.MemcachedConnectionsCurrent).Sum().DataPoints()
	totalConn := initMetric(ilm.Metrics(), metadata.M.MemcachedConnectionsTotal).Sum().DataPoints()
	currItems := initMetric(ilm.Metrics(), metadata.M.MemcachedCurrentItems).Sum().DataPoints()
	threads := initMetric(ilm.Metrics(), metadata.M.MemcachedThreads).Sum().DataPoints()
	evictions := initMetric(ilm.Metrics(), metadata.M.MemcachedEvictions).Sum().DataPoints()

	for _, stats := range allServerStats {
		for k, v := range stats.Stats {
			attributes := pcommon.NewMap()
			switch k {
			case "bytes":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(bytes, attributes, parsedV, now)
				}
			case "curr_connections":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(currConn, attributes, parsedV, now)
				}
			case "total_connections":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(totalConn, attributes, parsedV, now)
				}
			case "cmd_get":
				attributes.Insert(metadata.A.Command, pcommon.NewValueString("get"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV, now)
				}
			case "cmd_set":
				attributes.Insert(metadata.A.Command, pcommon.NewValueString("set"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV, now)
				}
			case "cmd_flush":
				attributes.Insert(metadata.A.Command, pcommon.NewValueString("flush"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV, now)
				}
			case "cmd_touch":
				attributes.Insert(metadata.A.Command, pcommon.NewValueString("touch"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(commandCount, attributes, parsedV, now)
				}
			case "curr_items":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(currItems, attributes, parsedV, now)
				}

			case "threads":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(threads, attributes, parsedV, now)
				}

			case "evictions":
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(evictions, attributes, parsedV, now)
				}
			case "bytes_read":
				attributes.Insert(metadata.A.Direction, pcommon.NewValueString("received"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(network, attributes, parsedV, now)
				}
			case "bytes_written":
				attributes.Insert(metadata.A.Direction, pcommon.NewValueString("sent"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(network, attributes, parsedV, now)
				}
			case "get_hits":
				attributes.Insert(metadata.A.Operation, pcommon.NewValueString("get"))
				attributes.Insert(metadata.A.Type, pcommon.NewValueString("hit"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV, now)
				}
			case "get_misses":
				attributes.Insert(metadata.A.Operation, pcommon.NewValueString("get"))
				attributes.Insert(metadata.A.Type, pcommon.NewValueString("miss"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV, now)
				}
			case "incr_hits":
				attributes.Insert(metadata.A.Operation, pcommon.NewValueString("increment"))
				attributes.Insert(metadata.A.Type, pcommon.NewValueString("hit"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV, now)
				}
			case "incr_misses":
				attributes.Insert(metadata.A.Operation, pcommon.NewValueString("increment"))
				attributes.Insert(metadata.A.Type, pcommon.NewValueString("miss"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV, now)
				}
			case "decr_hits":
				attributes.Insert(metadata.A.Operation, pcommon.NewValueString("decrement"))
				attributes.Insert(metadata.A.Type, pcommon.NewValueString("hit"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV, now)
				}
			case "decr_misses":
				attributes.Insert(metadata.A.Operation, pcommon.NewValueString("decrement"))
				attributes.Insert(metadata.A.Type, pcommon.NewValueString("miss"))
				if parsedV, ok := r.parseInt(k, v); ok {
					r.addToIntMetric(operationCount, attributes, parsedV, now)
				}
			case "rusage_system":
				attributes.Insert(metadata.A.State, pcommon.NewValueString("system"))
				if parsedV, ok := r.parseFloat(k, v); ok {
					r.addToDoubleMetric(rUsage, attributes, parsedV, now)
				}

			case "rusage_user":
				attributes.Insert(metadata.A.State, pcommon.NewValueString("user"))
				if parsedV, ok := r.parseFloat(k, v); ok {
					r.addToDoubleMetric(rUsage, attributes, parsedV, now)
				}
			}
		}

		// Calculated Metrics
		attributes := pcommon.NewMap()
		attributes.Insert(metadata.A.Operation, pcommon.NewValueString("increment"))
		parsedHit, okHit := r.parseInt("incr_hits", stats.Stats["incr_hits"])
		parsedMiss, okMiss := r.parseInt("incr_misses", stats.Stats["incr_misses"])
		if okHit && okMiss {
			r.addToDoubleMetric(hitRatio, attributes, calculateHitRatio(parsedHit, parsedMiss), now)
		}

		attributes = pcommon.NewMap()
		attributes.Insert(metadata.A.Operation, pcommon.NewValueString("decrement"))
		parsedHit, okHit = r.parseInt("decr_hits", stats.Stats["decr_hits"])
		parsedMiss, okMiss = r.parseInt("decr_misses", stats.Stats["decr_misses"])
		if okHit && okMiss {
			r.addToDoubleMetric(hitRatio, attributes, calculateHitRatio(parsedHit, parsedMiss), now)
		}

		attributes = pcommon.NewMap()
		attributes.Insert(metadata.A.Operation, pcommon.NewValueString("get"))
		parsedHit, okHit = r.parseInt("get_hits", stats.Stats["get_hits"])
		parsedMiss, okMiss = r.parseInt("get_misses", stats.Stats["get_misses"])
		if okHit && okMiss {
			r.addToDoubleMetric(hitRatio, attributes, calculateHitRatio(parsedHit, parsedMiss), now)
		}
	}
	return md, nil
}

func initMetric(ms pmetric.MetricSlice, mi metadata.MetricIntf) pmetric.Metric {
	m := ms.AppendEmpty()
	mi.Init(m)
	return m
}

func calculateHitRatio(misses, hits int64) float64 {
	if misses+hits == 0 {
		return 0
	}
	hitsFloat := float64(hits)
	missesFloat := float64(misses)
	return (hitsFloat / (hitsFloat + missesFloat) * 100)
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

func (r *memcachedScraper) addToDoubleMetric(metric pmetric.NumberDataPointSlice, attributes pcommon.Map, value float64, now pcommon.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(now)
	dataPoint.SetDoubleVal(value)
	if attributes.Len() > 0 {
		attributes.CopyTo(dataPoint.Attributes())
	}
}

func (r *memcachedScraper) addToIntMetric(metric pmetric.NumberDataPointSlice, attributes pcommon.Map, value int64, now pcommon.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(now)
	dataPoint.SetIntVal(value)
	if attributes.Len() > 0 {
		attributes.CopyTo(dataPoint.Attributes())
	}
}
