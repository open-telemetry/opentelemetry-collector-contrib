// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanmetricsprocessor

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"
)

const (
	serviceNameKey     = conventions.AttributeServiceName
	operationKey       = "operation" // is there a constant we can refer to?
	spanKindKey        = tracetranslator.TagSpanKind
	statusCodeKey      = tracetranslator.TagStatusCode
	metricKeySeparator = string(byte(0))
)

var (
	maxDuration   = time.Duration(math.MaxInt64)
	maxDurationMs = float64(maxDuration.Milliseconds())

	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000, maxDurationMs,
	}
)

// dimKV represents the dimension key-value pairs for a metric.
type dimKV map[string]string

type metricKey string

type processorImp struct {
	lock   sync.RWMutex
	logger *zap.Logger
	config Config

	metricsExporter component.MetricsExporter
	nextConsumer    consumer.TracesConsumer

	// Additional dimensions to add to metrics.
	dimensions []Dimension

	// The starting time of the data points.
	startTime time.Time

	// Call & Error counts.
	callSum map[metricKey]int64

	// Latency histogram.
	latencyCount        map[metricKey]uint64
	latencySum          map[metricKey]float64
	latencyBucketCounts map[metricKey][]uint64
	latencyBounds       []float64

	// A cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions map[metricKey]dimKV
}

func newProcessor(logger *zap.Logger, config configmodels.Exporter, nextConsumer consumer.TracesConsumer) *processorImp {
	logger.Info("Building spanmetricsprocessor")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets, func(duration time.Duration) float64 {
			return float64(duration.Milliseconds())
		})

		// "Catch-all" bucket.
		if bounds[len(bounds)-1] != maxDurationMs {
			bounds = append(bounds, maxDurationMs)
		}
	}

	return &processorImp{
		logger:                logger,
		config:                *pConfig,
		startTime:             time.Now(),
		callSum:               make(map[metricKey]int64),
		latencyBounds:         bounds,
		latencySum:            make(map[metricKey]float64),
		latencyCount:          make(map[metricKey]uint64),
		latencyBucketCounts:   make(map[metricKey][]uint64),
		nextConsumer:          nextConsumer,
		dimensions:            pConfig.Dimensions,
		metricKeyToDimensions: make(map[metricKey]dimKV),
	}
}

func mapDurationsToMillis(vs []time.Duration, f func(duration time.Duration) float64) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

// Start implements the component.Component interface.
func (p *processorImp) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting spanmetricsprocessor")
	exporters := host.GetExporters()

	var availableMetricsExporters []string

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[configmodels.MetricsDataType] {
		metricsExp, ok := exp.(component.MetricsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.Name())
		}

		availableMetricsExporters = append(availableMetricsExporters, k.Name())

		p.logger.Debug("Looking for spanmetrics exporter from available exporters",
			zap.String("spanmetrics-exporter", p.config.MetricsExporter),
			zap.Any("available-exporters", availableMetricsExporters),
		)
		if k.Name() == p.config.MetricsExporter {
			p.metricsExporter = metricsExp
			p.logger.Info("Found exporter", zap.String("spanmetrics-exporter", p.config.MetricsExporter))
			break
		}
	}
	if p.metricsExporter == nil {
		return fmt.Errorf("failed to find metrics exporter: '%s'; please configure metrics_exporter from one of: %+v",
			p.config.MetricsExporter, availableMetricsExporters)
	}
	p.logger.Info("Started spanmetricsprocessor")
	return nil
}

// Shutdown implements the component.Component interface.
func (p *processorImp) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down spanmetricsprocessor")
	return nil
}

// GetCapabilities implements the component.Processor interface.
func (p *processorImp) GetCapabilities() component.ProcessorCapabilities {
	p.logger.Info("GetCapabilities for spanmetricsprocessor")
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}

// ConsumeTraces implements the consumer.TracesConsumer interface.
// It aggregates the trace data to generate metrics, forwarding these metrics to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
func (p *processorImp) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	p.logger.Info("Consuming trace data")

	p.aggregateMetrics(traces)

	m := p.buildMetrics()

	// Firstly, export metrics to avoid being impacted by downstream trace processor errors/latency.
	if err := p.metricsExporter.ConsumeMetrics(ctx, *m); err != nil {
		return err
	}

	// Forward trace data unmodified.
	if err := p.nextConsumer.ConsumeTraces(ctx, traces); err != nil {
		return err
	}

	return nil
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() *pdata.Metrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.InstrumentationLibrary().SetName("spanmetricsprocessor")

	p.lock.RLock()
	p.collectCallMetrics(&ilm)
	p.collectLatencyMetrics(&ilm)
	p.lock.RUnlock()

	rm := pdata.NewResourceMetrics()
	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Append(ilm)

	m := pdata.NewMetrics()
	m.ResourceMetrics().Append(rm)
	return &m
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm *pdata.InstrumentationLibraryMetrics) {
	for key := range p.latencyCount {
		dpLatency := pdata.NewIntHistogramDataPoint()
		dpLatency.SetStartTime(pdata.TimeToUnixNano(p.startTime))
		dpLatency.SetTimestamp(pdata.TimeToUnixNano(time.Now()))
		dpLatency.SetExplicitBounds(p.latencyBounds)
		dpLatency.SetBucketCounts(p.latencyBucketCounts[key])
		dpLatency.SetCount(p.latencyCount[key])
		dpLatency.SetSum(int64(p.latencySum[key]))

		dpLatency.LabelsMap().InitFromMap(p.metricKeyToDimensions[key])

		mLatency := pdata.NewMetric()
		mLatency.SetDataType(pdata.MetricDataTypeIntHistogram)
		mLatency.SetName("latency")
		mLatency.IntHistogram().DataPoints().Append(dpLatency)
		mLatency.IntHistogram().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		ilm.Metrics().Append(mLatency)
	}
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm *pdata.InstrumentationLibraryMetrics) {
	for key := range p.callSum {
		dpCalls := pdata.NewIntDataPoint()
		dpCalls.SetStartTime(pdata.TimeToUnixNano(p.startTime))
		dpCalls.SetTimestamp(pdata.TimeToUnixNano(time.Now()))
		dpCalls.SetValue(p.callSum[key])

		dpCalls.LabelsMap().InitFromMap(p.metricKeyToDimensions[key])

		mCalls := pdata.NewMetric()
		mCalls.SetDataType(pdata.MetricDataTypeIntSum)
		mCalls.SetName("calls")
		mCalls.IntSum().DataPoints().Append(dpCalls)
		mCalls.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		ilm.Metrics().Append(mCalls)
	}
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions the user has configured.
func (p *processorImp) aggregateMetrics(traces pdata.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		r := rspans.Resource()

		attr, ok := r.Attributes().Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}
		serviceName := attr.StringVal()
		p.aggregateMetricsForServiceSpans(rspans, serviceName)
	}
}

func (p *processorImp) aggregateMetricsForServiceSpans(rspans pdata.ResourceSpans, serviceName string) {
	ilsSlice := rspans.InstrumentationLibrarySpans()
	for j := 0; j < ilsSlice.Len(); j++ {
		ils := ilsSlice.At(j)
		spans := ils.Spans()
		for k := 0; k < spans.Len(); k++ {
			span := spans.At(k)
			p.aggregateMetricsForSpan(serviceName, span)
		}
	}
}

func (p *processorImp) aggregateMetricsForSpan(serviceName string, span pdata.Span) {
	latencyInMilliseconds := float64(span.EndTime()-span.StartTime()) / float64(time.Millisecond.Nanoseconds())

	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latencyInMilliseconds)

	key := buildKey(serviceName, span, p.dimensions)

	p.lock.Lock()
	p.cache(serviceName, span, key)
	p.updateCallMetrics(key)
	p.updateLatencyMetrics(key, latencyInMilliseconds, index)
	p.lock.Unlock()
}

// updateCallMetrics increments the call count for the given metric key.
func (p *processorImp) updateCallMetrics(key metricKey) {
	p.callSum[key]++
}

// updateLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateLatencyMetrics(key metricKey, latency float64, index int) {
	if _, ok := p.latencyBucketCounts[key]; !ok {
		p.latencyBucketCounts[key] = make([]uint64, len(p.latencyBounds))
	}
	p.latencySum[key] += latency
	p.latencyCount[key]++
	p.latencyBucketCounts[key][index]++
}

func buildDimensionKVs(serviceName string, span pdata.Span, optionalDims []Dimension) dimKV {
	dims := make(dimKV)
	dims[serviceNameKey] = serviceName
	dims[operationKey] = span.Name()
	dims[spanKindKey] = span.Kind().String()
	dims[statusCodeKey] = span.Status().Code().String()
	spanAttr := span.Attributes()
	var value string
	for _, d := range optionalDims {
		// Set the default if configured, otherwise this metric will have no value set for the dimension.
		if d.Default != nil {
			value = *d.Default
		}
		if attr, ok := spanAttr.Get(d.Name); ok {
			value = tracetranslator.AttributeValueToString(attr, false)
		}
		dims[d.Name] = value
	}
	return dims
}

func concatDimensionValue(metricKeyBuilder *strings.Builder, value string, prefixSep bool) {
	// It's worth noting that from pprof benchmarks, WriteString is the most expensive operation of this processor.
	// Specifically, the need to grow the underlying []byte slice to make room for the appended string.
	if prefixSep {
		metricKeyBuilder.WriteString(metricKeySeparator)
	}
	metricKeyBuilder.WriteString(value)
}

// buildKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// any additional dimensions the user has configured.
// The metric key is a simple concatenation of dimension values.
func buildKey(serviceName string, span pdata.Span, optionalDims []Dimension) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, serviceName, false)
	concatDimensionValue(&metricKeyBuilder, span.Name(), true)
	concatDimensionValue(&metricKeyBuilder, span.Kind().String(), true)
	concatDimensionValue(&metricKeyBuilder, span.Status().Code().String(), true)

	spanAttr := span.Attributes()
	var value string
	for _, d := range optionalDims {
		// Set the default if configured, otherwise this metric will have no value set for the dimension.
		if d.Default != nil {
			value = *d.Default
		}
		if attr, ok := spanAttr.Get(d.Name); ok {
			value = tracetranslator.AttributeValueToString(attr, false)
		}
		concatDimensionValue(&metricKeyBuilder, value, true)
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//   LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) cache(serviceName string, span pdata.Span, k metricKey) {
	if _, ok := p.metricKeyToDimensions[k]; !ok {
		p.metricKeyToDimensions[k] = buildDimensionKVs(serviceName, span, p.dimensions)
	}
}
