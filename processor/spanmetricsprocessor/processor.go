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
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/internal/cache"
)

const (
	serviceNameKey             = conventions.AttributeServiceName
	instrumentationLibraryName = "spanmetricsprocessor"
	operationKey               = "operation"   // OpenTelemetry non-standard constant.
	spanKindKey                = "span.kind"   // OpenTelemetry non-standard constant.
	statusCodeKey              = "status.code" // OpenTelemetry non-standard constant.
	metricKeySeparator         = string(byte(0))
	traceIDKey                 = "trace_id"

	defaultDimensionsCacheSize         = 1000
	defaultResourceAttributesCacheSize = 1000
)

var (
	maxDuration   = time.Duration(math.MaxInt64)
	maxDurationMs = durationToMillis(maxDuration)

	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000, maxDurationMs,
	}
)

type exemplarData struct {
	traceID pdata.TraceID
	value   float64
}

type metricKey string
type resourceKey string

type processorImp struct {
	lock   sync.RWMutex
	logger *zap.Logger
	config Config

	metricsExporter component.MetricsExporter
	nextConsumer    consumer.Traces

	// Additional dimensions to add to metrics.
	dimensions []Dimension

	// Additional resourceAttributes to add to metrics.
	resourceAttributes []Dimension

	// The starting time of the data points.
	startTime time.Time

	// Call & Error counts.
	// todo do we need to use metricKey(actually conccated attrs), or we can use serviceName instead?
	// i.e. Should we put datapoints with different attributes under the same metrics or not?
	callSum map[resourceKey]map[metricKey]int64

	// Latency histogram.
	latencyCount         map[resourceKey]map[metricKey]uint64
	latencySum           map[resourceKey]map[metricKey]float64
	latencyBucketCounts  map[resourceKey]map[metricKey][]uint64
	latencyBounds        []float64
	latencyExemplarsData map[resourceKey]map[metricKey][]exemplarData

	// An LRU cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions *cache.Cache
	// A cache of resourceattributekey-value maps keyed by a unique identifier formed by a concatenation of its values.
	// todo - move to use the internal LRU cache
	resourceKeyToDimensions *cache.Cache
}

func newProcessor(logger *zap.Logger, config config.Processor, nextConsumer consumer.Traces) (*processorImp, error) {
	logger.Info("Building spanmetricsprocessor")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets)

		// "Catch-all" bucket.
		if bounds[len(bounds)-1] != maxDurationMs {
			bounds = append(bounds, maxDurationMs)
		}
	}

	if err := validateDimensions(pConfig.Dimensions, []string{serviceNameKey, spanKindKey, statusCodeKey}); err != nil {
		return nil, err
	}
	if err := validateDimensions(pConfig.ResourceAttributes, []string{serviceNameKey}); err != nil {
		return nil, err
	}

	metricKeyToDimensionsCache, err := cache.NewCache(pConfig.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}

	resourceKeyToDimensionsCache, err := cache.NewCache(pConfig.ResourceAttributesCacheSize)
	if err != nil {
		return nil, err
	}

	return &processorImp{
		logger:                  logger,
		config:                  *pConfig,
		startTime:               time.Now(),
		callSum:                 make(map[resourceKey]map[metricKey]int64),
		latencyBounds:           bounds,
		latencySum:              make(map[resourceKey]map[metricKey]float64),
		latencyCount:            make(map[resourceKey]map[metricKey]uint64),
		latencyBucketCounts:     make(map[resourceKey]map[metricKey][]uint64),
		latencyExemplarsData:    make(map[resourceKey]map[metricKey][]exemplarData),
		nextConsumer:            nextConsumer,
		dimensions:              pConfig.Dimensions,
		resourceKeyToDimensions: resourceKeyToDimensionsCache,
		metricKeyToDimensions:   metricKeyToDimensionsCache,
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
func validateDimensions(dimensions []Dimension, defaults []string) error {
	labelNames := make(map[string]struct{})
	for i := 0; i < len(defaults); i++ {
		labelNames[defaults[i]] = struct{}{}
		labelNames[sanitize(defaults[i])] = struct{}{}
	}
	labelNames[operationKey] = struct{}{}

	for _, key := range dimensions {
		if _, ok := labelNames[key.Name]; ok {
			return fmt.Errorf("duplicate dimension name %s", key.Name)
		}
		labelNames[key.Name] = struct{}{}

		sanitizedName := sanitize(key.Name)
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
func (p *processorImp) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	p.aggregateMetrics(traces)

	m, err := p.buildMetrics()
	if err != nil {
		return err
	}

	// Firstly, export metrics to avoid being impacted by downstream trace processor errors/latency.
	if err := p.metricsExporter.ConsumeMetrics(ctx, *m); err != nil {
		return err
	}

	p.resetExemplarData()

	// Forward trace data unmodified.
	return p.nextConsumer.ConsumeTraces(ctx, traces)
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() (*pdata.Metrics, error) {
	m := pdata.NewMetrics()
	rms := m.ResourceMetrics()
	for _, key := range p.resourceKeyToDimensions.Keys() {
		p.lock.Lock()
		//resourceAttrKey := key
		cachedResourceAttributesMap, ok := p.resourceKeyToDimensions.Get(key)
		if !ok {
			p.lock.Unlock()
			return nil, errors.New("expected cached resource attributes not found")
		}

		resourceAttributesMap, ok := cachedResourceAttributesMap.(pdata.AttributeMap)
		if !ok {
			p.lock.Unlock()
			return nil, errors.New("expected cached resource attributes type assertion failed")
		}

		// If the service name doesn't exist, we treat it as invalid and do not generate a trace
		if _, ok := resourceAttributesMap.Get(serviceNameKey); !ok {
			p.lock.Unlock()
			continue
		}

		rm := rms.AppendEmpty()

		// Iterate over `AttributeMap` structure defining resource attributes to append to the metric resource and append
		resourceAttributesMap.Range(func(k string, v pdata.AttributeValue) bool {
			rm.Resource().Attributes().Insert(k, v)
			return true
		})

		ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ilm.InstrumentationLibrary().SetName(instrumentationLibraryName)

		// build metrics per resource
		resourceAttrKey, ok := key.(resourceKey)
		if !ok {
			p.lock.Unlock()
			return nil, errors.New("resource key type assertion failed")
		}

		if err := p.collectCallMetrics(ilm, resourceAttrKey); err != nil {
			p.lock.Unlock()
			return nil, err
		}

		if err := p.collectLatencyMetrics(ilm, resourceAttrKey); err != nil {
			p.lock.Unlock()
			return nil, err
		}

		p.lock.Unlock()
	}

	p.metricKeyToDimensions.RemoveEvictedItems()
	p.resourceKeyToDimensions.RemoveEvictedItems()

	// If delta metrics, reset accumulated data
	if p.config.GetAggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
		p.resetAccumulatedMetrics()
	}
	p.resetExemplarData()

	return &m, nil
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm pdata.InstrumentationLibraryMetrics, resAttrKey resourceKey) error {
	for mKey := range p.latencyCount[resAttrKey] {
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetDataType(pdata.MetricDataTypeHistogram)
		mLatency.SetName("latency")
		mLatency.Histogram().SetAggregationTemporality(p.config.GetAggregationTemporality())

		timestamp := pdata.NewTimestampFromTime(time.Now())

		dpLatency := mLatency.Histogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(pdata.NewTimestampFromTime(p.startTime))
		dpLatency.SetTimestamp(timestamp)
		dpLatency.SetExplicitBounds(p.latencyBounds)
		dpLatency.SetBucketCounts(p.latencyBucketCounts[resAttrKey][mKey])
		dpLatency.SetCount(p.latencyCount[resAttrKey][mKey])
		dpLatency.SetSum(p.latencySum[resAttrKey][mKey])

		setLatencyExemplars(p.latencyExemplarsData[resAttrKey][mKey], timestamp, dpLatency.Exemplars())

		dimensions, err := p.getDimensionsByMetricKey(mKey)
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
func (p *processorImp) collectCallMetrics(ilm pdata.InstrumentationLibraryMetrics, resAttrKey resourceKey) error {
	for mKey := range p.callSum[resAttrKey] {
		mCalls := ilm.Metrics().AppendEmpty()
		mCalls.SetDataType(pdata.MetricDataTypeSum)
		mCalls.SetName("calls_total")
		mCalls.Sum().SetIsMonotonic(true)
		mCalls.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpCalls := mCalls.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pdata.NewTimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
		dpCalls.SetIntVal(p.callSum[resAttrKey][mKey])

		dimensions, err := p.getDimensionsByMetricKey(mKey)
		if err != nil {
			p.logger.Error(err.Error())
			return err
		}

		dimensions.CopyTo(dpCalls.Attributes())
	}
	return nil
}

// getDimensionsByMetricKey gets dimensions from `metricKeyToDimensions` cache.
func (p *processorImp) getDimensionsByMetricKey(k metricKey) (*pdata.AttributeMap, error) {
	if item, ok := p.metricKeyToDimensions.Get(k); ok {
		if attributeMap, ok := item.(pdata.AttributeMap); ok {
			return &attributeMap, nil
		}
		return nil, fmt.Errorf("type assertion of metricKeyToDimensions attributes failed, the key is %q", k)
	}

	return nil, fmt.Errorf("value not found in metricKeyToDimensions cache by key %q", k)
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions and resource attributes the user has configured.
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
			p.aggregateMetricsForSpan(serviceName, span, rspans.Resource().Attributes())
		}
	}
}

func (p *processorImp) aggregateMetricsForSpan(serviceName string, span pdata.Span, resourceAttr pdata.AttributeMap) {
	latencyInMilliseconds := float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())

	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latencyInMilliseconds)

	mKey := p.buildMetricKey(span, resourceAttr)
	resourceAttrKey := p.buildResourceAttrKey(serviceName, resourceAttr)

	p.lock.Lock()
	p.cacheMetricKey(span, mKey, resourceAttr)
	p.cacheResourceAttrKey(serviceName, resourceAttr, resourceAttrKey)
	p.updateCallMetrics(resourceAttrKey, mKey)
	p.updateLatencyMetrics(resourceAttrKey, mKey, latencyInMilliseconds, index)
	p.updateLatencyExemplars(resourceAttrKey, mKey, latencyInMilliseconds, span.TraceID())
	p.lock.Unlock()
}

// updateCallMetrics increments the call count for the given metric key.
func (p *processorImp) updateCallMetrics(resourceAttrKey resourceKey, mKey metricKey) {
	if _, ok := p.callSum[resourceAttrKey]; !ok {
		p.callSum[resourceAttrKey] = map[metricKey]int64{mKey: 1}
		return
	}

	if _, ok := p.callSum[resourceAttrKey][mKey]; !ok {
		p.callSum[resourceAttrKey][mKey] = 1
		return
	}

	p.callSum[resourceAttrKey][mKey]++
}

// resetAccumulatedMetrics resets the internal maps used to store created metric data. Also purge the cache for
// metricKeyToDimensions.
func (p *processorImp) resetAccumulatedMetrics() {
	p.callSum = make(map[resourceKey]map[metricKey]int64)
	p.latencyCount = make(map[resourceKey]map[metricKey]uint64)
	p.latencySum = make(map[resourceKey]map[metricKey]float64)
	p.latencyBucketCounts = make(map[resourceKey]map[metricKey][]uint64)
	p.metricKeyToDimensions.Purge()
	p.resourceKeyToDimensions.Purge()
}

// updateLatencyExemplars sets the histogram exemplars for the given metric key and append the exemplar data.
func (p *processorImp) updateLatencyExemplars(resourceAttrKey resourceKey, mKey metricKey, value float64, traceID pdata.TraceID) {
	_, ok := p.latencyExemplarsData[resourceAttrKey]

	if !ok {
		p.latencyExemplarsData[resourceAttrKey] = make(map[metricKey][]exemplarData)
		p.latencyExemplarsData[resourceAttrKey][mKey] = []exemplarData{}
	}

	if _, ok = p.latencyExemplarsData[resourceAttrKey][mKey]; !ok {
		p.latencyExemplarsData[resourceAttrKey][mKey] = []exemplarData{}
	}

	e := exemplarData{
		traceID: traceID,
		value:   value,
	}
	p.latencyExemplarsData[resourceAttrKey][mKey] = append(p.latencyExemplarsData[resourceAttrKey][mKey], e)
}

// resetExemplarData resets the entire exemplars map so the next trace will recreate all
// the data structure. An exemplar is a punctual value that exists at specific moment in time
// and should be not considered like a metrics that persist over time.
func (p *processorImp) resetExemplarData() {
	p.latencyExemplarsData = make(map[resourceKey]map[metricKey][]exemplarData)
}

// updateLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateLatencyMetrics(resourceAttrKey resourceKey, mKey metricKey, latency float64, index int) {
	_, ok := p.latencyBucketCounts[resourceAttrKey]

	if !ok {
		p.latencyBucketCounts[resourceAttrKey] = make(map[metricKey][]uint64)
		p.latencyBucketCounts[resourceAttrKey][mKey] = make([]uint64, len(p.latencyBounds))
	}

	if _, ok = p.latencyBucketCounts[resourceAttrKey][mKey]; !ok {
		p.latencyBucketCounts[resourceAttrKey][mKey] = make([]uint64, len(p.latencyBounds))
	}

	p.latencyBucketCounts[resourceAttrKey][mKey][index]++

	if _, ok := p.latencySum[resourceAttrKey]; ok {
		p.latencySum[resourceAttrKey][mKey] += latency
	} else {
		p.latencySum[resourceAttrKey] = map[metricKey]float64{mKey: latency}
	}

	if _, ok := p.latencyCount[resourceAttrKey]; ok {
		p.latencyCount[resourceAttrKey][mKey]++
	} else {
		p.latencyCount[resourceAttrKey] = map[metricKey]uint64{mKey: 1}
	}
}

func (p *processorImp) buildDimensionKVs(span pdata.Span, optionalDims []Dimension, resourceAttrs pdata.AttributeMap) pdata.AttributeMap {
	dims := pdata.NewAttributeMap()

	dims.UpsertString(operationKey, span.Name())
	dims.UpsertString(spanKindKey, span.Kind().String())
	dims.UpsertString(statusCodeKey, span.Status().Code().String())
	for _, d := range optionalDims {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			dims.Upsert(d.Name, v)
		}
	}
	return dims
}

func extractResourceAttrsByKeys(serviceName string, keys []Dimension, resourceAttrs pdata.AttributeMap) pdata.AttributeMap {
	dims := pdata.NewAttributeMap()
	dims.UpsertString(serviceNameKey, serviceName)
	for _, ra := range keys {
		if attr, ok := resourceAttrs.Get(ra.Name); ok {
			dims.Upsert(ra.Name, attr)
		} else if ra.Default != nil {
			// Set the default if configured, otherwise this metric should have no value set for the resource attribute.
			dims.Upsert(ra.Name, pdata.NewAttributeValueString(*ra.Default))
		}
	}
	return dims
}

func concatDimensionValue(metricKeyBuilder *strings.Builder, value string) {
	// It's worth noting that from pprof benchmarks, WriteString is the most expensive operation of this processor.
	// Specifically, the need to grow the underlying []byte slice to make room for the appended string.
	if metricKeyBuilder.Len() != 0 {
		metricKeyBuilder.WriteString(metricKeySeparator)
	}
	metricKeyBuilder.WriteString(value)
}

// buildMetricKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func (p *processorImp) buildMetricKey(span pdata.Span, resourceAttrs pdata.AttributeMap) metricKey {
	var metricKeyBuilder strings.Builder
	concatDimensionValue(&metricKeyBuilder, span.Name())
	concatDimensionValue(&metricKeyBuilder, span.Kind().String())
	concatDimensionValue(&metricKeyBuilder, span.Status().Code().String())

	for _, d := range p.dimensions {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			concatDimensionValue(&metricKeyBuilder, v.AsString())
		}
	}

	k := metricKey(metricKeyBuilder.String())
	return k
}

// buildResourceAttrKey builds the metric key from the service name and will attempt to add any additional resource attributes
// the user has configured that match the span's attributes
//
// The resource attribute key is a simple concatenation of the service name and the other specified resource attribute
// values, delimited by a null character.
func (p *processorImp) buildResourceAttrKey(serviceName string, resourceAttr pdata.AttributeMap) resourceKey {
	var resourceKeyBuilder strings.Builder
	concatDimensionValue(&resourceKeyBuilder, serviceName)

	for _, ra := range p.resourceAttributes {
		if attr, ok := resourceAttr.Get(ra.Name); ok {

			concatDimensionValue(&resourceKeyBuilder, attr.AsString())
		} else if ra.Default != nil {
			// Set the default if configured, otherwise this metric should have no value set for the resource attribute.
			concatDimensionValue(&resourceKeyBuilder, *ra.Default)
		}
	}

	k := resourceKey(resourceKeyBuilder.String())
	return k
}

// getDimensionValue gets the dimension value for the given configured dimension.
// It searches through the span's attributes first, being the more specific;
// falling back to searching in resource attributes if it can't be found in the span.
// Finally, falls back to the configured default value if provided.
//
// The ok flag indicates if a dimension value was fetched in order to differentiate
// an empty string value from a state where no value was found.
// todo - consider this: Given we are building resource attributes for the metrics, does that make sense to search from
// resource attributes anymore?
func getDimensionValue(d Dimension, spanAttr pdata.AttributeMap, resourceAttr pdata.AttributeMap) (v pdata.AttributeValue, ok bool) {
	// The more specific span attribute should take precedence.
	if attr, exists := spanAttr.Get(d.Name); exists {
		return attr, true
	}
	if attr, exists := resourceAttr.Get(d.Name); exists {
		return attr, true
	}
	// Set the default if configured, otherwise this metric will have no value set for the dimension.
	if d.Default != nil {
		return pdata.NewAttributeValueString(*d.Default), true
	}
	return v, ok
}

// cache the dimension key-value map for the metricKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the metric like so:
// LabelsMap().InitFromMap(p.metricKeyToDimensions[key])
func (p *processorImp) cacheMetricKey(span pdata.Span, k metricKey, resourceAttrs pdata.AttributeMap) {
	p.metricKeyToDimensions.ContainsOrAdd(k, p.buildDimensionKVs(span, p.dimensions, resourceAttrs))
}

// cache the dimension key-value map for the resourceAttrKey if there is a cache miss.
// This enables a lookup of the dimension key-value map when constructing the resource.
func (p *processorImp) cacheResourceAttrKey(serviceName string, resourceAttrs pdata.AttributeMap, k resourceKey) {
	p.resourceKeyToDimensions.ContainsOrAdd(k, extractResourceAttrsByKeys(serviceName, p.resourceAttributes, resourceAttrs))
}

// copied from prometheus-go-metric-exporter
// sanitize replaces non-alphanumeric characters with underscores in s.
func sanitize(s string) string {
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
	if s[0] == '_' {
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
func setLatencyExemplars(exemplarsData []exemplarData, timestamp pdata.Timestamp, exemplars pdata.ExemplarSlice) {
	es := pdata.NewExemplarSlice()
	es.EnsureCapacity(len(exemplarsData))

	for _, ed := range exemplarsData {
		value := ed.value
		traceID := ed.traceID

		exemplar := es.AppendEmpty()

		if traceID.IsEmpty() {
			continue
		}

		exemplar.SetDoubleVal(value)
		exemplar.SetTimestamp(timestamp)
		exemplar.FilteredAttributes().Insert(traceIDKey, pdata.NewAttributeValueString(traceID.HexString()))
	}

	es.CopyTo(exemplars)
}
