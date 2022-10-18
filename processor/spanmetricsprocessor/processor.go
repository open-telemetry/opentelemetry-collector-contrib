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

package spanmetricsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor"

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/internal/cache"
)

const (
	serviceNameKey     = conventions.AttributeServiceName
	operationKey       = "operation"   // OpenTelemetry non-standard constant.
	spanKindKey        = "span.kind"   // OpenTelemetry non-standard constant.
	statusCodeKey      = "status.code" // OpenTelemetry non-standard constant.
	metricKeySeparator = string(byte(0))

	defaultDimensionsCacheSize = 1000
)

var (
	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000,
	}
)

type exemplarData struct {
	traceID pcommon.TraceID
	spanID  pcommon.SpanID
	value   float64
}

type metricKey string

type processorImp struct {
	lock   sync.Mutex
	logger *zap.Logger
	config Config

	metricsExporter component.MetricsExporter
	nextConsumer    consumer.Traces

	// Additional dimensions to add to metrics.
	dimensions []Dimension

	// The starting time of the data points.
	startTime time.Time

	// Call & Error counts.
	callSum map[metricKey]int64

	// Latency histogram.
	latencyCount         map[metricKey]uint64
	latencySum           map[metricKey]float64
	latencyBucketCounts  map[metricKey][]uint64
	latencyBounds        []float64
	latencyExemplarsData map[metricKey][]exemplarData

	// An LRU cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions *cache.Cache
}

func newProcessor(logger *zap.Logger, config config.Processor, nextConsumer consumer.Traces) (*processorImp, error) {
	logger.Info("Building spanmetricsprocessor")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets)
	}

	if err := validateDimensions(pConfig.Dimensions, pConfig.skipSanitizeLabel); err != nil {
		return nil, err
	}

	if pConfig.DimensionsCacheSize <= 0 {
		return nil, fmt.Errorf(
			"invalid cache size: %v, the maximum number of the items in the cache should be positive",
			pConfig.DimensionsCacheSize,
		)
	}
	metricKeyToDimensionsCache, err := cache.NewCache(pConfig.DimensionsCacheSize)
	if err != nil {
		return nil, err
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
		latencyExemplarsData:  make(map[metricKey][]exemplarData),
		nextConsumer:          nextConsumer,
		dimensions:            pConfig.Dimensions,
		metricKeyToDimensions: metricKeyToDimensionsCache,
	}, nil
}

// durationToMillis converts the given duration to the number of milliseconds it represents.
// Note that this can return sub-millisecond (i.e. < 1ms) values as well.
func durationToMillis(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(time.Millisecond.Nanoseconds())
}

func mapDurationsToMillis(vs []time.Duration) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = durationToMillis(v)
	}
	return vsm
}

// validateDimensions checks duplicates for reserved dimensions and additional dimensions. Considering
// the usage of Prometheus related exporters, we also validate the dimensions after sanitization.
func validateDimensions(dimensions []Dimension, skipSanitizeLabel bool) error {
	labelNames := make(map[string]struct{})
	for _, key := range []string{serviceNameKey, spanKindKey, statusCodeKey} {
		labelNames[key] = struct{}{}
		labelNames[sanitize(key, skipSanitizeLabel)] = struct{}{}
	}
	labelNames[operationKey] = struct{}{}

	for _, key := range dimensions {
		if _, ok := labelNames[key.Name]; ok {
			return fmt.Errorf("duplicate dimension name %s", key.Name)
		}
		labelNames[key.Name] = struct{}{}

		sanitizedName := sanitize(key.Name, skipSanitizeLabel)
		if sanitizedName == key.Name {
			continue
		}
		if _, ok := labelNames[sanitizedName]; ok {
			return fmt.Errorf("duplicate dimension name %s after sanitization", sanitizedName)
		}
		labelNames[sanitizedName] = struct{}{}
	}

	return nil
}

// Start implements the component.Component interface.
func (p *processorImp) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting spanmetricsprocessor")
	exporters := host.GetExporters()

	var availableMetricsExporters []string

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[config.MetricsDataType] {
		metricsExp, ok := exp.(component.MetricsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.String())
		}

		availableMetricsExporters = append(availableMetricsExporters, k.String())

		p.logger.Debug("Looking for spanmetrics exporter from available exporters",
			zap.String("spanmetrics-exporter", p.config.MetricsExporter),
			zap.Any("available-exporters", availableMetricsExporters),
		)
		if k.String() == p.config.MetricsExporter {
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

// Capabilities implements the consumer interface.
func (p *processorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate metrics, forwarding these metrics to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
func (p *processorImp) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	// Forward trace data unmodified and propagate both metrics and trace pipeline errors, if any.
	return multierr.Combine(p.tracesToMetrics(ctx, traces), p.nextConsumer.ConsumeTraces(ctx, traces))
}

func (p *processorImp) tracesToMetrics(ctx context.Context, traces ptrace.Traces) error {
	p.lock.Lock()

	p.aggregateMetrics(traces)
	m, err := p.buildMetrics()

	// Exemplars are only relevant to this batch of traces, so must be cleared within the lock,
	// regardless of error while building metrics, before the next batch of spans is received.
	p.resetExemplarData()

	// This component no longer needs to read the metrics once built, so it is safe to unlock.
	p.lock.Unlock()

	if err != nil {
		return err
	}

	if err = p.metricsExporter.ConsumeMetrics(ctx, m); err != nil {
		return err
	}

	return nil
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() (pmetric.Metrics, error) {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("spanmetricsprocessor")

	if err := p.collectCallMetrics(ilm); err != nil {
		return pmetric.Metrics{}, err
	}

	if err := p.collectLatencyMetrics(ilm); err != nil {
		return pmetric.Metrics{}, err
	}

	p.metricKeyToDimensions.RemoveEvictedItems()

	// If delta metrics, reset accumulated data
	if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		p.resetAccumulatedMetrics()
	}
	p.resetExemplarData()

	return m, nil
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.latencyCount {
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetName("latency")
		mLatency.SetUnit("ms")
		mLatency.SetEmptyHistogram().SetAggregationTemporality(p.config.GetAggregationTemporality())

		timestamp := pcommon.NewTimestampFromTime(time.Now())

		dpLatency := mLatency.Histogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpLatency.SetTimestamp(timestamp)
		dpLatency.ExplicitBounds().FromRaw(p.latencyBounds)
		dpLatency.BucketCounts().FromRaw(p.latencyBucketCounts[key])
		dpLatency.SetCount(p.latencyCount[key])
		dpLatency.SetSum(p.latencySum[key])

		setLatencyExemplars(p.latencyExemplarsData[key], timestamp, dpLatency.Exemplars())

		dimensions, err := p.getDimensionsByMetricKey(key)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpLatency.Attributes())
	}
	return nil
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.callSum {
		mCalls := ilm.Metrics().AppendEmpty()
		mCalls.SetName("calls_total")
		mCalls.SetEmptySum().SetIsMonotonic(true)
		mCalls.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpCalls := mCalls.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCalls.SetIntValue(p.callSum[key])

		dimensions, err := p.getDimensionsByMetricKey(key)
		if err != nil {
			return err
		}

		dimensions.CopyTo(dpCalls.Attributes())
	}
	return nil
}

// getDimensionsByMetricKey gets dimensions from `metricKeyToDimensions` cache.
func (p *processorImp) getDimensionsByMetricKey(k metricKey) (*pcommon.Map, error) {
	if item, ok := p.metricKeyToDimensions.Get(k); ok {
		if attributeMap, ok := item.(pcommon.Map); ok {
			return &attributeMap, nil
		}
		return nil, fmt.Errorf("type assertion of metricKeyToDimensions attributes failed, the key is %q", k)
	}

	return nil, fmt.Errorf("value not found in metricKeyToDimensions cache by key %q", k)
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions the user has configured.
func (p *processorImp) aggregateMetrics(traces ptrace.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		r := rspans.Resource()

		attr, ok := r.Attributes().Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}
		serviceName := attr.Str()
		p.aggregateMetricsForServiceSpans(rspans, serviceName)
	}
}

func (p *processorImp) aggregateMetricsForServiceSpans(rspans ptrace.ResourceSpans, serviceName string) {
	ilsSlice := rspans.ScopeSpans()
	for j := 0; j < ilsSlice.Len(); j++ {
		ils := ilsSlice.At(j)
		spans := ils.Spans()
		for k := 0; k < spans.Len(); k++ {
			span := spans.At(k)
			p.aggregateMetricsForSpan(serviceName, span, rspans.Resource().Attributes())
		}
	}
}

func (p *processorImp) aggregateMetricsForSpan(serviceName string, span ptrace.Span, resourceAttr pcommon.Map) {
	// Protect against end timestamps before start timestamps. Assume 0 duration.
	latencyInMilliseconds := float64(0)
	startTime := span.StartTimestamp()
	endTime := span.EndTimestamp()
	if endTime > startTime {
		latencyInMilliseconds = float64(endTime-startTime) / float64(time.Millisecond.Nanoseconds())
	}

	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latencyInMilliseconds)

	key := buildKey(serviceName, span, p.dimensions, resourceAttr)

	p.cache(serviceName, span, key, resourceAttr)
	p.updateCallMetrics(key)
	p.updateLatencyMetrics(key, latencyInMilliseconds, index)
	p.updateLatencyExemplars(key, latencyInMilliseconds, span.TraceID(), span.SpanID())
}

// updateCallMetrics increments the call count for the given metric key.
func (p *processorImp) updateCallMetrics(key metricKey) {
	p.callSum[key]++
}

// resetAccumulatedMetrics resets the internal maps used to store created metric data. Also purge the cache for
// metricKeyToDimensions.
func (p *processorImp) resetAccumulatedMetrics() {
	p.callSum = make(map[metricKey]int64)
	p.latencyCount = make(map[metricKey]uint64)
	p.latencySum = make(map[metricKey]float64)
	p.latencyBucketCounts = make(map[metricKey][]uint64)
	p.metricKeyToDimensions.Purge()
}

// updateLatencyExemplars sets the histogram exemplars for the given metric key and append the exemplar data.
func (p *processorImp) updateLatencyExemplars(key metricKey, value float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	if _, ok := p.latencyExemplarsData[key]; !ok {
		p.latencyExemplarsData[key] = []exemplarData{}
	}

	e := exemplarData{
		traceID: traceID,
		spanID:  spanID,
		value:   value,
	}
	p.latencyExemplarsData[key] = append(p.latencyExemplarsData[key], e)
}

// resetExemplarData resets the entire exemplars map so the next trace will recreate all
// the data structure. An exemplar is a punctual value that exists at specific moment in time
// and should be not considered like a metrics that persist over time.
func (p *processorImp) resetExemplarData() {
	p.latencyExemplarsData = make(map[metricKey][]exemplarData)
}

// updateLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateLatencyMetrics(key metricKey, latency float64, index int) {
	if _, ok := p.latencyBucketCounts[key]; !ok {
		p.latencyBucketCounts[key] = make([]uint64, len(p.latencyBounds)+1)
	}
	p.latencySum[key] += latency
	p.latencyCount[key]++
	p.latencyBucketCounts[key][index]++
}

func (p *processorImp) buildDimensionKVs(serviceName string, span ptrace.Span, optionalDims []Dimension, resourceAttrs pcommon.Map) pcommon.Map {
	dims := pcommon.NewMap()
	dims.PutStr(serviceNameKey, serviceName)
	dims.PutStr(operationKey, span.Name())
	dims.PutStr(spanKindKey, span.Kind().String())
	dims.PutStr(statusCodeKey, span.Status().Code().String())
	for _, d := range optionalDims {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			v.CopyTo(dims.PutEmpty(d.Name))
		}
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
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func buildKey(serviceName string, span ptrace.Span, optionalDims []Dimension, resourceAttrs pcommon.Map) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, serviceName, false)
	concatDimensionValue(&metricKeyBuilder, span.Name(), true)
	concatDimensionValue(&metricKeyBuilder, span.Kind().String(), true)
	concatDimensionValue(&metricKeyBuilder, span.Status().Code().String(), true)

	for _, d := range optionalDims {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			concatDimensionValue(&metricKeyBuilder, v.AsString(), true)
		}
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

// getDimensionValue gets the dimension value for the given configured dimension.
// It searches through the span's attributes first, being the more specific;
// falling back to searching in resource attributes if it can't be found in the span.
// Finally, falls back to the configured default value if provided.
//
// The ok flag indicates if a dimension value was fetched in order to differentiate
// an empty string value from a state where no value was found.
func getDimensionValue(d Dimension, spanAttr pcommon.Map, resourceAttr pcommon.Map) (v pcommon.Value, ok bool) {
	// The more specific span attribute should take precedence.
	if attr, exists := spanAttr.Get(d.Name); exists {
		return attr, true
	}
	if attr, exists := resourceAttr.Get(d.Name); exists {
		return attr, true
	}
	// Set the default if configured, otherwise this metric will have no value set for the dimension.
	if d.Default != nil {
		return pcommon.NewValueStr(*d.Default), true
	}
	return v, ok
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
//
//	LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) cache(serviceName string, span ptrace.Span, k metricKey, resourceAttrs pcommon.Map) {
	// Use Get to ensure any existing key has its recent-ness updated.
	if _, has := p.metricKeyToDimensions.Get(k); !has {
		p.metricKeyToDimensions.Add(k, p.buildDimensionKVs(serviceName, span, p.dimensions, resourceAttrs))
	}
}

// copied from prometheus-go-metric-exporter
// sanitize replaces non-alphanumeric characters with underscores in s.
func sanitize(s string, skipSanitizeLabel bool) string {
	if len(s) == 0 {
		return s
	}

	// Note: No length limit for label keys because Prometheus doesn't
	// define a length limit, thus we should NOT be truncating label keys.
	// See https://github.com/orijtech/prometheus-go-metrics-exporter/issues/4.
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	// replace labels starting with _ only when skipSanitizeLabel is disabled
	if !skipSanitizeLabel && strings.HasPrefix(s, "_") {
		s = "key" + s
	}
	// labels starting with __ are reserved in prometheus
	if strings.HasPrefix(s, "__") {
		s = "key" + s
	}
	return s
}

// copied from prometheus-go-metric-exporter
// sanitizeRune converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}

// setLatencyExemplars sets the histogram exemplars.
func setLatencyExemplars(exemplarsData []exemplarData, timestamp pcommon.Timestamp, exemplars pmetric.ExemplarSlice) {
	es := pmetric.NewExemplarSlice()
	es.EnsureCapacity(len(exemplarsData))

	for _, ed := range exemplarsData {
		value := ed.value
		traceID := ed.traceID
		spanID := ed.spanID

		exemplar := es.AppendEmpty()

		if traceID.IsEmpty() {
			continue
		}

		exemplar.SetDoubleValue(value)
		exemplar.SetTimestamp(timestamp)
		exemplar.SetTraceID(traceID)
		exemplar.SetSpanID(spanID)
	}

	es.CopyTo(exemplars)
}
