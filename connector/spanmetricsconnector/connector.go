// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/jonboulle/clockwork"
	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	utilattri "github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

const (
	serviceNameKey                 = conventions.AttributeServiceName
	spanNameKey                    = "span.name"                          // OpenTelemetry non-standard constant.
	spanKindKey                    = "span.kind"                          // OpenTelemetry non-standard constant.
	statusCodeKey                  = "status.code"                        // OpenTelemetry non-standard constant.
	instrumentationScopeNameKey    = "span.instrumentation.scope.name"    // OpenTelemetry non-standard constant.
	instrumentationScopeVersionKey = "span.instrumentation.scope.version" // OpenTelemetry non-standard constant.
	metricKeySeparator             = string(byte(0))

	defaultDimensionsCacheSize      = 1000
	defaultResourceMetricsCacheSize = 1000

	metricNameDuration = "duration"
	metricNameCalls    = "calls"
	metricNameEvents   = "events"

	defaultUnit = metrics.Milliseconds
)

type connectorImp struct {
	lock   sync.Mutex
	logger *zap.Logger
	config Config

	metricsConsumer consumer.Metrics

	// Additional dimensions to add to metrics.
	dimensions []utilattri.Dimension

	resourceMetrics *cache.Cache[resourceKey, *resourceMetrics]

	resourceMetricsKeyAttributes map[string]struct{}

	keyBuf *bytes.Buffer

	// An LRU cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "span.name": "/bar", "status_code": "OK" }}
	metricKeyToDimensions *cache.Cache[metrics.Key, pcommon.Map]

	clock   clockwork.Clock
	ticker  clockwork.Ticker
	done    chan struct{}
	started bool

	shutdownOnce sync.Once

	// Event dimensions to add to the events metric.
	eDimensions []utilattri.Dimension

	events EventsConfig

	// Tracks the last TimestampUnixNano for delta metrics so that they represent an uninterrupted series. Unused for cumulative span metrics.
	lastDeltaTimestamps *simplelru.LRU[metrics.Key, pcommon.Timestamp]
}

type resourceMetrics struct {
	histograms metrics.HistogramMetrics
	sums       metrics.SumMetrics
	events     metrics.SumMetrics
	attributes pcommon.Map
	// lastSeen captures when the last data points for this resource were recorded.
	lastSeen time.Time
}

func newDimensions(cfgDims []Dimension) []utilattri.Dimension {
	if len(cfgDims) == 0 {
		return nil
	}
	dims := make([]utilattri.Dimension, len(cfgDims))
	for i := range cfgDims {
		dims[i].Name = cfgDims[i].Name
		if cfgDims[i].Default != nil {
			val := pcommon.NewValueStr(*cfgDims[i].Default)
			dims[i].Value = &val
		}
	}
	return dims
}

func newConnector(logger *zap.Logger, config component.Config, clock clockwork.Clock) (*connectorImp, error) {
	logger.Info("Building spanmetrics connector")
	cfg := config.(*Config)

	metricKeyToDimensionsCache, err := cache.NewCache[metrics.Key, pcommon.Map](cfg.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}

	resourceMetricsCache, err := cache.NewCache[resourceKey, *resourceMetrics](cfg.ResourceMetricsCacheSize)
	if err != nil {
		return nil, err
	}

	resourceMetricsKeyAttributes := make(map[string]struct{}, len(cfg.ResourceMetricsKeyAttributes))
	var s struct{}
	for _, attr := range cfg.ResourceMetricsKeyAttributes {
		resourceMetricsKeyAttributes[attr] = s
	}

	var lastDeltaTimestamps *simplelru.LRU[metrics.Key, pcommon.Timestamp]
	if cfg.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		lastDeltaTimestamps, err = simplelru.NewLRU[metrics.Key, pcommon.Timestamp](cfg.GetDeltaTimestampCacheSize(), func(k metrics.Key, _ pcommon.Timestamp) {
			logger.Info("Evicting cached delta timestamp", zap.String("key", string(k)))
		})
		if err != nil {
			return nil, err
		}
	}

	return &connectorImp{
		logger:                       logger,
		config:                       *cfg,
		resourceMetrics:              resourceMetricsCache,
		resourceMetricsKeyAttributes: resourceMetricsKeyAttributes,
		dimensions:                   newDimensions(cfg.Dimensions),
		keyBuf:                       bytes.NewBuffer(make([]byte, 0, 1024)),
		metricKeyToDimensions:        metricKeyToDimensionsCache,
		lastDeltaTimestamps:          lastDeltaTimestamps,
		clock:                        clock,
		ticker:                       clock.NewTicker(cfg.MetricsFlushInterval),
		done:                         make(chan struct{}),
		eDimensions:                  newDimensions(cfg.Events.Dimensions),
		events:                       cfg.Events,
	}, nil
}

func initHistogramMetrics(cfg Config) metrics.HistogramMetrics {
	if cfg.Histogram.Disable {
		return nil
	}
	if cfg.Histogram.Exponential != nil {
		maxSize := structure.DefaultMaxSize
		if cfg.Histogram.Exponential.MaxSize != 0 {
			maxSize = cfg.Histogram.Exponential.MaxSize
		}
		return metrics.NewExponentialHistogramMetrics(maxSize, cfg.Exemplars.MaxPerDataPoint)
	}

	var bounds []float64
	if cfg.Histogram.Explicit != nil && cfg.Histogram.Explicit.Buckets != nil {
		bounds = durationsToUnits(cfg.Histogram.Explicit.Buckets, unitDivider(cfg.Histogram.Unit))
	} else {
		switch cfg.Histogram.Unit {
		case metrics.Milliseconds:
			bounds = defaultHistogramBucketsMs
		case metrics.Seconds:
			bounds = make([]float64, len(defaultHistogramBucketsMs))
			for i, v := range defaultHistogramBucketsMs {
				bounds[i] = v / float64(time.Second.Milliseconds())
			}
		}
	}

	return metrics.NewExplicitHistogramMetrics(bounds, cfg.Exemplars.MaxPerDataPoint)
}

// unitDivider returns a unit divider to convert nanoseconds to milliseconds or seconds.
func unitDivider(u metrics.Unit) int64 {
	return map[metrics.Unit]int64{
		metrics.Seconds:      time.Second.Nanoseconds(),
		metrics.Milliseconds: time.Millisecond.Nanoseconds(),
	}[u]
}

func durationsToUnits(vs []time.Duration, unitDivider int64) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = float64(v.Nanoseconds()) / float64(unitDivider)
	}
	return vsm
}

// Start implements the component.Component interface.
func (p *connectorImp) Start(ctx context.Context, _ component.Host) error {
	p.logger.Info("Starting spanmetrics connector")

	p.started = true
	go func() {
		for {
			select {
			case <-p.done:
				return
			case <-p.ticker.Chan():
				p.exportMetrics(ctx)
			}
		}
	}()

	return nil
}

// Shutdown implements the component.Component interface.
func (p *connectorImp) Shutdown(context.Context) error {
	p.shutdownOnce.Do(func() {
		p.logger.Info("Shutting down spanmetrics connector")
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
func (p *connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate metrics.
func (p *connectorImp) ConsumeTraces(_ context.Context, traces ptrace.Traces) error {
	p.lock.Lock()
	p.aggregateMetrics(traces)
	p.lock.Unlock()
	return nil
}

func (p *connectorImp) exportMetrics(ctx context.Context) {
	p.lock.Lock()

	m := p.buildMetrics()
	p.resetState()

	// This component no longer needs to read the metrics once built, so it is safe to unlock.
	p.lock.Unlock()

	if err := p.metricsConsumer.ConsumeMetrics(ctx, m); err != nil {
		p.logger.Error("Failed ConsumeMetrics", zap.Error(err))
		return
	}
}

// buildMetrics collects the computed raw metrics data and builds OTLP metrics.
func (p *connectorImp) buildMetrics() pmetric.Metrics {
	m := pmetric.NewMetrics()
	timestamp := pcommon.NewTimestampFromTime(p.clock.Now())

	p.resourceMetrics.ForEach(func(_ resourceKey, rawMetrics *resourceMetrics) {
		rm := m.ResourceMetrics().AppendEmpty()
		rawMetrics.attributes.CopyTo(rm.Resource().Attributes())

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("spanmetricsconnector")

		/**
		 * To represent an uninterrupted stream of metrics as per the spec, the (StartTimestamp, Timestamp)'s of successive data points should be:
		 * - For cumulative metrics: (T1, T2), (T1, T3), (T1, T4) ...
		 * - For delta metrics: (T1, T2), (T2, T3), (T3, T4) ...
		 */
		deltaMetricKeys := make(map[metrics.Key]bool)
		timeStampGenerator := func(mk metrics.Key, startTime pcommon.Timestamp) pcommon.Timestamp {
			if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
				if lastTimestamp, ok := p.lastDeltaTimestamps.Get(mk); ok {
					startTime = lastTimestamp
				}
				// Collect lastDeltaTimestamps keys that need to be updated. Metrics can share the same key, so defer the update.
				deltaMetricKeys[mk] = true
			}
			return startTime
		}

		metricsNamespace := p.config.Namespace
		if legacyMetricNamesFeatureGate.IsEnabled() && metricsNamespace == DefaultNamespace {
			metricsNamespace = ""
		}

		sums := rawMetrics.sums
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(buildMetricName(metricsNamespace, metricNameCalls))
		sums.BuildMetrics(metric, timestamp, timeStampGenerator, p.config.GetAggregationTemporality())

		if !p.config.Histogram.Disable {
			histograms := rawMetrics.histograms
			metric = sm.Metrics().AppendEmpty()
			metric.SetName(buildMetricName(metricsNamespace, metricNameDuration))
			metric.SetUnit(p.config.Histogram.Unit.String())
			histograms.BuildMetrics(metric, timestamp, timeStampGenerator, p.config.GetAggregationTemporality())
		}

		events := rawMetrics.events
		if p.events.Enabled {
			metric = sm.Metrics().AppendEmpty()
			metric.SetName(buildMetricName(metricsNamespace, metricNameEvents))
			events.BuildMetrics(metric, timestamp, timeStampGenerator, p.config.GetAggregationTemporality())
		}

		for mk := range deltaMetricKeys {
			// For delta metrics, cache the current data point's timestamp, which will be the start timestamp for the next data points in the series
			p.lastDeltaTimestamps.Add(mk, timestamp)
		}
	})

	return m
}

func (p *connectorImp) resetState() {
	// If delta metrics, reset accumulated data
	if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		p.resourceMetrics.Purge()
		p.metricKeyToDimensions.Purge()
	} else {
		p.resourceMetrics.RemoveEvictedItems()
		p.metricKeyToDimensions.RemoveEvictedItems()

		// If none of these features are enabled then we can skip the remaining operations.
		// Enabling either of these features requires to go over resource metrics and do operation on each.
		if p.config.Histogram.Disable && p.config.MetricsExpiration == 0 && !p.config.Exemplars.Enabled {
			return
		}

		now := p.clock.Now()
		p.resourceMetrics.ForEach(func(k resourceKey, m *resourceMetrics) {
			// Exemplars are only relevant to this batch of traces, so must be cleared within the lock
			if p.config.Exemplars.Enabled {
				m.sums.ClearExemplars()
				m.events.ClearExemplars()
				if !p.config.Histogram.Disable {
					m.histograms.ClearExemplars()
				}
			}

			// If metrics expiration is configured, remove metrics that haven't been seen for longer than the expiration period.
			if p.config.MetricsExpiration > 0 {
				if now.Sub(m.lastSeen) >= p.config.MetricsExpiration {
					p.resourceMetrics.Remove(k)
				}
			}
		})
	}
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
//
// Metrics are grouped by resource attributes.
// Each metric is identified by a key that is built from the service name
// and span metadata such as name, kind, status_code and any additional
// dimensions the user has configured.
func (p *connectorImp) aggregateMetrics(traces ptrace.Traces) {
	startTimestamp := pcommon.NewTimestampFromTime(p.clock.Now())
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		resourceAttr := rspans.Resource().Attributes()
		serviceAttr, ok := resourceAttr.Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}

		rm := p.getOrCreateResourceMetrics(resourceAttr)
		sums := rm.sums
		histograms := rm.histograms
		events := rm.events

		unitDivider := unitDivider(p.config.Histogram.Unit)
		serviceName := serviceAttr.Str()
		ilsSlice := rspans.ScopeSpans()
		for j := 0; j < ilsSlice.Len(); j++ {
			ils := ilsSlice.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				// Protect against end timestamps before start timestamps. Assume 0 duration.
				duration := float64(0)
				startTime := span.StartTimestamp()
				endTime := span.EndTimestamp()
				if endTime > startTime {
					duration = float64(endTime-startTime) / float64(unitDivider)
				}
				key := p.buildKey(serviceName, span, p.dimensions, resourceAttr)

				attributes, ok := p.metricKeyToDimensions.Get(key)
				if !ok {
					attributes = p.buildAttributes(serviceName, span, resourceAttr, p.dimensions, ils.Scope())
					p.metricKeyToDimensions.Add(key, attributes)
				}
				if !p.config.Histogram.Disable {
					// aggregate histogram metrics
					h := histograms.GetOrCreate(key, attributes, startTimestamp)
					p.addExemplar(span, duration, h)
					h.Observe(duration)
				}
				// aggregate sums metrics
				s := sums.GetOrCreate(key, attributes, startTimestamp)
				if p.config.Exemplars.Enabled && !span.TraceID().IsEmpty() {
					s.AddExemplar(span.TraceID(), span.SpanID(), duration)
				}
				s.Add(1)

				// aggregate events metrics
				if p.events.Enabled {
					for l := 0; l < span.Events().Len(); l++ {
						event := span.Events().At(l)
						eDimensions := p.dimensions
						eDimensions = append(eDimensions, p.eDimensions...)

						rscAndEventAttrs := pcommon.NewMap()
						rscAndEventAttrs.EnsureCapacity(resourceAttr.Len() + event.Attributes().Len())
						resourceAttr.CopyTo(rscAndEventAttrs)
						event.Attributes().CopyTo(rscAndEventAttrs)

						eKey := p.buildKey(serviceName, span, eDimensions, rscAndEventAttrs)
						eAttributes, ok := p.metricKeyToDimensions.Get(eKey)
						if !ok {
							eAttributes = p.buildAttributes(serviceName, span, rscAndEventAttrs, eDimensions, ils.Scope())
							p.metricKeyToDimensions.Add(eKey, eAttributes)
						}
						e := events.GetOrCreate(eKey, eAttributes, startTimestamp)
						if p.config.Exemplars.Enabled && !span.TraceID().IsEmpty() {
							e.AddExemplar(span.TraceID(), span.SpanID(), duration)
						}
						e.Add(1)
					}
				}
			}
		}
	}
}

func (p *connectorImp) addExemplar(span ptrace.Span, duration float64, h metrics.Histogram) {
	if !p.config.Exemplars.Enabled {
		return
	}
	if span.TraceID().IsEmpty() {
		return
	}

	h.AddExemplar(span.TraceID(), span.SpanID(), duration)
}

type resourceKey [16]byte

func (p *connectorImp) createResourceKey(attr pcommon.Map) resourceKey {
	if len(p.resourceMetricsKeyAttributes) == 0 {
		return pdatautil.MapHash(attr)
	}
	m := pcommon.NewMap()
	attr.CopyTo(m)
	m.RemoveIf(func(k string, _ pcommon.Value) bool {
		_, ok := p.resourceMetricsKeyAttributes[k]
		return !ok
	})
	return pdatautil.MapHash(m)
}

func (p *connectorImp) getOrCreateResourceMetrics(attr pcommon.Map) *resourceMetrics {
	key := p.createResourceKey(attr)
	v, ok := p.resourceMetrics.Get(key)
	if !ok {
		v = &resourceMetrics{
			histograms: initHistogramMetrics(p.config),
			sums:       metrics.NewSumMetrics(p.config.Exemplars.MaxPerDataPoint),
			events:     metrics.NewSumMetrics(p.config.Exemplars.MaxPerDataPoint),
			attributes: attr,
		}
		p.resourceMetrics.Add(key, v)
	}

	// If expiration is enabled, track the last seen time.
	if p.config.MetricsExpiration > 0 {
		v.lastSeen = p.clock.Now()
	}

	return v
}

// contains checks if string slice contains a string value
func contains(elements []string, value string) bool {
	for _, element := range elements {
		if value == element {
			return true
		}
	}
	return false
}

func (p *connectorImp) buildAttributes(serviceName string, span ptrace.Span, resourceAttrs pcommon.Map, dimensions []utilattri.Dimension, instrumentationScope pcommon.InstrumentationScope) pcommon.Map {
	attr := pcommon.NewMap()
	attr.EnsureCapacity(4 + len(dimensions))
	if !contains(p.config.ExcludeDimensions, serviceNameKey) {
		attr.PutStr(serviceNameKey, serviceName)
	}
	if !contains(p.config.ExcludeDimensions, spanNameKey) {
		attr.PutStr(spanNameKey, span.Name())
	}
	if !contains(p.config.ExcludeDimensions, spanKindKey) {
		attr.PutStr(spanKindKey, traceutil.SpanKindStr(span.Kind()))
	}
	if !contains(p.config.ExcludeDimensions, statusCodeKey) {
		attr.PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))
	}

	for _, d := range dimensions {
		if v, ok := utilattri.GetDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			v.CopyTo(attr.PutEmpty(d.Name))
		}
	}

	if contains(p.config.IncludeInstrumentationScope, instrumentationScope.Name()) && instrumentationScope.Name() != "" {
		attr.PutStr(instrumentationScopeNameKey, instrumentationScope.Name())
		if instrumentationScope.Version() != "" {
			attr.PutStr(instrumentationScopeVersionKey, instrumentationScope.Version())
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

// buildKey builds the metric key from the service name and span metadata such as name, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource/event attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func (p *connectorImp) buildKey(serviceName string, span ptrace.Span, optionalDims []utilattri.Dimension, resourceOrEventAttrs pcommon.Map) metrics.Key {
	p.keyBuf.Reset()
	if !contains(p.config.ExcludeDimensions, serviceNameKey) {
		concatDimensionValue(p.keyBuf, serviceName, false)
	}
	if !contains(p.config.ExcludeDimensions, spanNameKey) {
		concatDimensionValue(p.keyBuf, span.Name(), true)
	}
	if !contains(p.config.ExcludeDimensions, spanKindKey) {
		concatDimensionValue(p.keyBuf, traceutil.SpanKindStr(span.Kind()), true)
	}
	if !contains(p.config.ExcludeDimensions, statusCodeKey) {
		concatDimensionValue(p.keyBuf, traceutil.StatusCodeStr(span.Status().Code()), true)
	}

	for _, d := range optionalDims {
		if v, ok := utilattri.GetDimensionValue(d, span.Attributes(), resourceOrEventAttrs); ok {
			concatDimensionValue(p.keyBuf, v.AsString(), true)
		}
	}

	return metrics.Key(p.keyBuf.String())
}

// buildMetricName builds the namespace prefix for the metric name.
func buildMetricName(namespace string, name string) string {
	if namespace != "" {
		return namespace + "." + name
	}
	return name
}
