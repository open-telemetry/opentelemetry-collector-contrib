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
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
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
	dimensions []dimension

	// The starting time of the data points.
	startTimestamp pcommon.Timestamp

	// Histogram.
	histograms    map[metricKey]*histogramData
	latencyBounds []float64

	keyBuf *bytes.Buffer

	// An LRU cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions *cache.Cache[metricKey, pcommon.Map]
}

type dimension struct {
	name  string
	value *pcommon.Value
}

func newDimensions(cfgDims []Dimension) []dimension {
	if len(cfgDims) == 0 {
		return nil
	}
	dims := make([]dimension, len(cfgDims))
	for i := range cfgDims {
		dims[i].name = cfgDims[i].Name
		if cfgDims[i].Default != nil {
			val := pcommon.NewValueStr(*cfgDims[i].Default)
			dims[i].value = &val
		}
	}
	return dims
}

type histogramData struct {
	count         uint64
	sum           float64
	bucketCounts  []uint64
	exemplarsData []exemplarData
}

func newProcessor(logger *zap.Logger, config component.ProcessorConfig, nextConsumer consumer.Traces) (*processorImp, error) {
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
	metricKeyToDimensionsCache, err := cache.NewCache[metricKey, pcommon.Map](pConfig.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}

	return &processorImp{
		logger:                logger,
		config:                *pConfig,
		startTimestamp:        pcommon.NewTimestampFromTime(time.Now()),
		latencyBounds:         bounds,
		histograms:            make(map[metricKey]*histogramData),
		nextConsumer:          nextConsumer,
		dimensions:            newDimensions(pConfig.Dimensions),
		keyBuf:                bytes.NewBuffer(make([]byte, 0, 1024)),
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
	for k, exp := range exporters[component.DataTypeMetrics] {
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
func (p *processorImp) Shutdown(context.Context) error {
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

	// store metricKeys that relate to the current batch of spans received.
	metricKeys := p.aggregateMetrics(traces)
	m, err := p.buildMetrics(metricKeys)

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
func (p *processorImp) buildMetrics(metricKeys []metricKey) (pmetric.Metrics, error) {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("spanmetricsprocessor")

	if err := p.collectCallMetrics(ilm, metricKeys); err != nil {
		return pmetric.Metrics{}, err
	}

	if err := p.collectLatencyMetrics(ilm, metricKeys); err != nil {
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
func (p *processorImp) collectLatencyMetrics(ilm pmetric.ScopeMetrics, metricKeys []metricKey) error {
	for _, key := range metricKeys {
		hist, ok := p.histograms[key]
		if !ok {
			return fmt.Errorf("histogramData not found in histograms by key %q", key)
		}
		dimensions, err := p.getDimensionsByMetricKey(key)
		if err != nil {
			return err
		}
		mLatency := ilm.Metrics().AppendEmpty()
		mLatency.SetName("latency")
		mLatency.SetUnit("ms")
		mLatency.SetEmptyHistogram().SetAggregationTemporality(p.config.GetAggregationTemporality())

		timestamp := pcommon.NewTimestampFromTime(time.Now())

		dpLatency := mLatency.Histogram().DataPoints().AppendEmpty()
		dpLatency.SetStartTimestamp(p.startTimestamp)
		dpLatency.SetTimestamp(timestamp)
		dpLatency.ExplicitBounds().FromRaw(p.latencyBounds)
		dpLatency.BucketCounts().FromRaw(hist.bucketCounts)
		dpLatency.SetCount(hist.count)
		dpLatency.SetSum(hist.sum)
		setExemplars(hist.exemplarsData, timestamp, dpLatency.Exemplars())
		dimensions.CopyTo(dpLatency.Attributes())
	}
	return nil
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm pmetric.ScopeMetrics, metricKeys []metricKey) error {
	for _, key := range metricKeys {
		hist, ok := p.histograms[key]
		if !ok {
			return fmt.Errorf("histogramData not found in histograms by key %q", key)
		}
		dimensions, err := p.getDimensionsByMetricKey(key)
		if err != nil {
			return err
		}
		mCalls := ilm.Metrics().AppendEmpty()
		mCalls.SetName("calls_total")
		mCalls.SetEmptySum().SetIsMonotonic(true)
		mCalls.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())

		dpCalls := mCalls.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(p.startTimestamp)
		dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCalls.SetIntValue(int64(hist.count))
		dimensions.CopyTo(dpCalls.Attributes())
	}
	return nil
}

// getDimensionsByMetricKey gets dimensions from `metricKeyToDimensions` cache.
func (p *processorImp) getDimensionsByMetricKey(k metricKey) (pcommon.Map, error) {
	if attributeMap, ok := p.metricKeyToDimensions.Get(k); ok {
		return attributeMap, nil
	}
	return pcommon.Map{}, fmt.Errorf("value not found in metricKeyToDimensions cache by key %q", k)
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions the user has configured.
func (p *processorImp) aggregateMetrics(traces ptrace.Traces) []metricKey {
	var metricKeys []metricKey
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		resourceAttr := rspans.Resource().Attributes()
		serviceAttr, ok := resourceAttr.Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}
		serviceName := serviceAttr.Str()
		ilsSlice := rspans.ScopeSpans()
		for j := 0; j < ilsSlice.Len(); j++ {
			ils := ilsSlice.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				// Protect against end timestamps before start timestamps. Assume 0 duration.
				latencyInMilliseconds := float64(0)
				startTime := span.StartTimestamp()
				endTime := span.EndTimestamp()
				if endTime > startTime {
					latencyInMilliseconds = float64(endTime-startTime) / float64(time.Millisecond.Nanoseconds())
				}
				// Always reset the buffer before re-using.
				p.keyBuf.Reset()
				buildKey(p.keyBuf, serviceName, span, p.dimensions, resourceAttr)
				key := metricKey(p.keyBuf.String())
				metricKeys = append(metricKeys, key)
				p.cache(serviceName, span, key, resourceAttr)
				p.updateHistogram(key, latencyInMilliseconds, span.TraceID(), span.SpanID())
			}
		}
	}
	return metricKeys
}

// resetAccumulatedMetrics resets the internal maps used to store created metric data. Also purge the cache for
// metricKeyToDimensions.
func (p *processorImp) resetAccumulatedMetrics() {
	p.histograms = make(map[metricKey]*histogramData)
	p.metricKeyToDimensions.Purge()
}

// updateHistogram adds the histogram sample to the histogram defined by the metric key.
func (p *processorImp) updateHistogram(key metricKey, latency float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	histo, ok := p.histograms[key]
	if !ok {
		histo = &histogramData{
			bucketCounts: make([]uint64, len(p.latencyBounds)+1),
		}
		p.histograms[key] = histo
	}

	histo.sum += latency
	histo.count++
	// Binary search to find the latencyInMilliseconds bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latency)
	histo.bucketCounts[index]++
	histo.exemplarsData = append(histo.exemplarsData, exemplarData{traceID: traceID, spanID: spanID, value: latency})
}

// resetExemplarData resets the entire exemplars map so the next trace will recreate all
// the data structure. An exemplar is a punctual value that exists at specific moment in time
// and should be not considered like a metrics that persist over time.
func (p *processorImp) resetExemplarData() {
	for _, histo := range p.histograms {
		histo.exemplarsData = nil
	}
}

func (p *processorImp) buildDimensionKVs(serviceName string, span ptrace.Span, resourceAttrs pcommon.Map) pcommon.Map {
	dims := pcommon.NewMap()
	dims.EnsureCapacity(4 + len(p.dimensions))
	dims.PutStr(serviceNameKey, serviceName)
	dims.PutStr(operationKey, span.Name())
	dims.PutStr(spanKindKey, traceutil.SpanKindStr(span.Kind()))
	dims.PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))
	for _, d := range p.dimensions {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			v.CopyTo(dims.PutEmpty(d.name))
		}
	}
	return dims
}

func concatDimensionValue(dest *bytes.Buffer, value string, prefixSep bool) {
	if prefixSep {
		dest.WriteString(metricKeySeparator)
	}
	dest.WriteString(value)
}

// buildKey builds the metric key from the service name and span metadata such as operation, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func buildKey(dest *bytes.Buffer, serviceName string, span ptrace.Span, optionalDims []dimension, resourceAttrs pcommon.Map) {
	concatDimensionValue(dest, serviceName, false)
	concatDimensionValue(dest, span.Name(), true)
	concatDimensionValue(dest, traceutil.SpanKindStr(span.Kind()), true)
	concatDimensionValue(dest, traceutil.StatusCodeStr(span.Status().Code()), true)

	for _, d := range optionalDims {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			concatDimensionValue(dest, v.AsString(), true)
		}
	}
}

// getDimensionValue gets the dimension value for the given configured dimension.
// It searches through the span's attributes first, being the more specific;
// falling back to searching in resource attributes if it can't be found in the span.
// Finally, falls back to the configured default value if provided.
//
// The ok flag indicates if a dimension value was fetched in order to differentiate
// an empty string value from a state where no value was found.
func getDimensionValue(d dimension, spanAttr pcommon.Map, resourceAttr pcommon.Map) (v pcommon.Value, ok bool) {
	// The more specific span attribute should take precedence.
	if attr, exists := spanAttr.Get(d.name); exists {
		return attr, true
	}
	if attr, exists := resourceAttr.Get(d.name); exists {
		return attr, true
	}
	// Set the default if configured, otherwise this metric will have no value set for the dimension.
	if d.value != nil {
		return *d.value, true
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
		p.metricKeyToDimensions.Add(k, p.buildDimensionKVs(serviceName, span, resourceAttrs))
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

// setExemplars sets the histogram exemplars.
func setExemplars(exemplarsData []exemplarData, timestamp pcommon.Timestamp, exemplars pmetric.ExemplarSlice) {
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
