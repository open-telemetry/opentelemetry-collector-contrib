// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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

	metricLatency    = "latency"
	metricCallsTotal = "calls_total"

	defaultDimensionsCacheSize = 1000
)

var defaultLatencyHistogramBucketsMs = []float64{
	2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000,
}

type exemplar struct {
	traceID pcommon.TraceID
	spanID  pcommon.SpanID
	value   float64
}

type metricKey string

type processorImp struct {
	lock   sync.Mutex
	logger *zap.Logger
	config Config

	metricsConsumer consumer.Metrics
	tracesConsumer  consumer.Traces

	// Additional dimensions to add to metrics.
	dimensions []dimension

	// The starting time of the data points.
	startTimestamp pcommon.Timestamp

	// Histogram.
	histograms    map[metricKey]*histogram
	latencyBounds []float64

	keyBuf *bytes.Buffer

	// An LRU cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "operation": "/bar", "status_code": "OK" }}
	metricKeyToDimensions *cache.Cache[metricKey, pcommon.Map]

	ticker  *clock.Ticker
	done    chan struct{}
	started bool

	shutdownOnce sync.Once
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

type histogram struct {
	attributes pcommon.Map

	count        uint64
	sum          float64
	bucketCounts []uint64
	exemplars    []exemplar

	latencyBounds []float64
}

// observe a measurement and adds an exemplar.
func (h *histogram) observe(latencyMs float64, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	h.sum += latencyMs
	h.count++
	// Binary search to find the latencyMs bucket index.
	index := sort.SearchFloat64s(h.latencyBounds, latencyMs)
	h.bucketCounts[index]++
	h.exemplars = append(h.exemplars, exemplar{traceID: traceID, spanID: spanID, value: latencyMs})
}

func newProcessor(logger *zap.Logger, config component.Config, ticker *clock.Ticker) (*processorImp, error) {
	logger.Info("Building spanmetrics")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets)
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
		histograms:            make(map[metricKey]*histogram),
		dimensions:            newDimensions(pConfig.Dimensions),
		keyBuf:                bytes.NewBuffer(make([]byte, 0, 1024)),
		metricKeyToDimensions: metricKeyToDimensionsCache,
		ticker:                ticker,
		done:                  make(chan struct{}),
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

// Start implements the component.Component interface.
func (p *processorImp) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting spanmetricsprocessor")
	exporters := host.GetExporters() //nolint:staticcheck

	var availableMetricsExporters []string

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[component.DataTypeMetrics] {
		metricsExp, ok := exp.(exporter.Metrics)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.String())
		}

		availableMetricsExporters = append(availableMetricsExporters, k.String())

		p.logger.Debug("Looking for spanmetrics exporter from available exporters",
			zap.String("spanmetrics-exporter", p.config.MetricsExporter),
			zap.Any("available-exporters", availableMetricsExporters),
		)
		if k.String() == p.config.MetricsExporter {
			p.metricsConsumer = metricsExp
			p.logger.Info("Found exporter", zap.String("spanmetrics-exporter", p.config.MetricsExporter))
			break
		}
	}
	if p.metricsConsumer == nil {
		return fmt.Errorf("failed to find metrics exporter: '%s'; please configure metrics_exporter from one of: %+v",
			p.config.MetricsExporter, availableMetricsExporters)
	}

	p.started = true
	go func() {
		for {
			select {
			case <-p.done:
				return
			case <-p.ticker.C:
				p.exportMetrics(ctx)
			}
		}
	}()

	return nil
}

// Shutdown implements the component.Component interface.
func (p *processorImp) Shutdown(context.Context) error {
	p.shutdownOnce.Do(func() {
		p.logger.Info("Shutting down spanmetricsprocessor")
		if p.started {
			p.logger.Info("Stopping ticker")
			p.ticker.Stop()
			p.done <- struct{}{}
			p.started = false
		}
	})
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
	p.lock.Lock()
	p.aggregateMetrics(traces)
	p.lock.Unlock()

	// Forward trace data unmodified and propagate trace pipeline errors, if any.
	return p.tracesConsumer.ConsumeTraces(ctx, traces)
}

func (p *processorImp) exportMetrics(ctx context.Context) {
	p.lock.Lock()

	m := p.buildMetrics()

	// Exemplars are only relevant to this batch of traces, so must be cleared within the lock,
	// regardless of error while building metrics, before the next batch of spans is received.
	p.resetExemplars()

	// If delta metrics, reset accumulated data
	if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		p.histograms = make(map[metricKey]*histogram)
		p.metricKeyToDimensions.Purge()
	} else {
		p.metricKeyToDimensions.RemoveEvictedItems()
	}

	// This component no longer needs to read the metrics once built, so it is safe to unlock.
	p.lock.Unlock()

	if err := p.metricsConsumer.ConsumeMetrics(ctx, m); err != nil {
		p.logger.Error("Failed ConsumeMetrics", zap.Error(err))
		return
	}
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() pmetric.Metrics {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("spanmetricsprocessor")

	p.collectCallMetrics(ilm)
	p.collectLatencyMetrics(ilm)

	return m
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm pmetric.ScopeMetrics) {
	mLatency := ilm.Metrics().AppendEmpty()
	mLatency.SetName(buildMetricName(p.config.Namespace, metricLatency))
	mLatency.SetUnit("ms")
	mLatency.SetEmptyHistogram().SetAggregationTemporality(p.config.GetAggregationTemporality())
	dps := mLatency.Histogram().DataPoints()
	dps.EnsureCapacity(len(p.histograms))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, hist := range p.histograms {
		dpLatency := dps.AppendEmpty()
		dpLatency.SetStartTimestamp(p.startTimestamp)
		dpLatency.SetTimestamp(timestamp)
		dpLatency.ExplicitBounds().FromRaw(p.latencyBounds)
		dpLatency.BucketCounts().FromRaw(hist.bucketCounts)
		dpLatency.SetCount(hist.count)
		dpLatency.SetSum(hist.sum)
		setExemplars(hist.exemplars, timestamp, dpLatency.Exemplars())
		hist.attributes.CopyTo(dpLatency.Attributes())
	}
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm pmetric.ScopeMetrics) {
	mCalls := ilm.Metrics().AppendEmpty()
	mCalls.SetName(buildMetricName(p.config.Namespace, metricCallsTotal))
	mCalls.SetEmptySum().SetIsMonotonic(true)
	mCalls.Sum().SetAggregationTemporality(p.config.GetAggregationTemporality())
	dps := mCalls.Sum().DataPoints()
	dps.EnsureCapacity(len(p.histograms))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, hist := range p.histograms {
		dpCalls := dps.AppendEmpty()
		dpCalls.SetStartTimestamp(p.startTimestamp)
		dpCalls.SetTimestamp(timestamp)
		dpCalls.SetIntValue(int64(hist.count))
		hist.attributes.CopyTo(dpCalls.Attributes())
	}
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions the user has configured.
func (p *processorImp) aggregateMetrics(traces ptrace.Traces) {
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
				latencyMs := float64(0)
				startTime := span.StartTimestamp()
				endTime := span.EndTimestamp()
				if endTime > startTime {
					latencyMs = float64(endTime-startTime) / float64(time.Millisecond.Nanoseconds())
				}

				// Always reset the buffer before re-using.
				p.keyBuf.Reset()
				buildKey(p.keyBuf, serviceName, span, p.dimensions, resourceAttr)
				key := metricKey(p.keyBuf.String())

				attributes, ok := p.metricKeyToDimensions.Get(key)
				if !ok {
					attributes = p.buildAttributes(serviceName, span, resourceAttr)
					p.metricKeyToDimensions.Add(key, attributes)
				}

				h := p.getOrCreateHistogram(key, attributes)
				h.observe(latencyMs, span.TraceID(), span.SpanID())
			}
		}
	}
}

func (p *processorImp) getOrCreateHistogram(k metricKey, attr pcommon.Map) *histogram {
	h, ok := p.histograms[k]
	if !ok {
		h = &histogram{
			attributes:    attr,
			bucketCounts:  make([]uint64, len(p.latencyBounds)+1),
			latencyBounds: p.latencyBounds,
			exemplars:     []exemplar{},
		}
		p.histograms[k] = h
	}

	return h
}

// resetExemplars resets the entire exemplars map so the next trace will recreate all
// the data structure. An exemplar is a punctual value that exists at specific moment in time
// and should be not considered like a metrics that persist over time.
func (p *processorImp) resetExemplars() {
	for _, histo := range p.histograms {
		histo.exemplars = nil
	}
}

func (p *processorImp) buildAttributes(serviceName string, span ptrace.Span, resourceAttrs pcommon.Map) pcommon.Map {
	attr := pcommon.NewMap()
	attr.EnsureCapacity(4 + len(p.dimensions))
	attr.PutStr(serviceNameKey, serviceName)
	attr.PutStr(operationKey, span.Name())
	attr.PutStr(spanKindKey, traceutil.SpanKindStr(span.Kind()))
	attr.PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))
	for _, d := range p.dimensions {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			v.CopyTo(attr.PutEmpty(d.name))
		}
	}
	return attr
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
func setExemplars(exemplarsData []exemplar, timestamp pcommon.Timestamp, exemplars pmetric.ExemplarSlice) {
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

// buildMetricName builds the namespace prefix for the metric name.
func buildMetricName(namespace string, name string) string {
	if namespace != "" {
		return namespace + "." + name
	}
	return name
}
