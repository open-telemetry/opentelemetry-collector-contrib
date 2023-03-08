// Copyright The OpenTelemetry Authors
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

package spanmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	serviceNameKey     = conventions.AttributeServiceName
	spanNameKey        = "span.name"   // OpenTelemetry non-standard constant.
	spanKindKey        = "span.kind"   // OpenTelemetry non-standard constant.
	statusCodeKey      = "status.code" // OpenTelemetry non-standard constant.
	metricKeySeparator = string(byte(0))

	defaultDimensionsCacheSize = 1000

	metricNameLatency = "latency"
	metricNameCalls   = "calls"

	defaultUnit = "ms"
)

var unitDividers = map[string]func() int64{
	"s": func() int64 {
		return time.Second.Nanoseconds()
	},
	"ms": func() int64 {
		return time.Millisecond.Nanoseconds()
	},
}

type connectorImp struct {
	lock   sync.Mutex
	logger *zap.Logger
	config Config

	metricsConsumer consumer.Metrics

	// Additional dimensions to add to metrics.
	dimensions []dimension

	// The starting time of the data points.
	startTimestamp pcommon.Timestamp

	// Metrics
	histograms metrics.HistogramMetrics
	sums       metrics.SumMetrics

	// unit divider, used to convert nanoseconds to milliseconds or seconds
	unitDivider int64

	keyBuf *bytes.Buffer

	// An LRU cache of dimension key-value maps keyed by a unique identifier formed by a concatenation of its values:
	// e.g. { "foo/barOK": { "serviceName": "foo", "span.name": "/bar", "status_code": "OK" }}
	metricKeyToDimensions *cache.Cache[metrics.Key, pcommon.Map]

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

func newConnector(logger *zap.Logger, config component.Config, ticker *clock.Ticker) (*connectorImp, error) {
	logger.Info("Building spanmetrics connector")
	cfg := config.(*Config)

	metricKeyToDimensionsCache, err := cache.NewCache[metrics.Key, pcommon.Map](cfg.DimensionsCacheSize)
	if err != nil {
		return nil, err
	}

	unitDivider := unitDividers[cfg.Histogram.Unit]
	var histograms metrics.HistogramMetrics
	if cfg.Histogram.Exponential != nil {
		maxSize := cfg.Histogram.Exponential.MaxSize
		if cfg.Histogram.Exponential.MaxSize == 0 {
			maxSize = structure.DefaultMaxSize
		}
		histograms = metrics.NewExponentialHistogramMetrics(maxSize)
	} else {
		bounds := defaultHistogramBucketsMs
		// TODO remove deprecated `latency_histogram_buckets`
		if cfg.LatencyHistogramBuckets != nil {
			logger.Warn("latency_histogram_buckets is deprecated. " +
				"Use `histogram: explicit: buckets` to set histogram buckets")
			bounds = durationsToUnits(cfg.LatencyHistogramBuckets, unitDivider())
		}
		if cfg.Histogram.Explicit != nil && cfg.Histogram.Explicit.Buckets != nil {
			bounds = durationsToUnits(cfg.Histogram.Explicit.Buckets, unitDivider())
		}
		histograms = metrics.NewExplicitHistogramMetrics(bounds)
	}

	return &connectorImp{
		logger:                logger,
		config:                *cfg,
		startTimestamp:        pcommon.NewTimestampFromTime(time.Now()),
		histograms:            histograms,
		sums:                  metrics.NewSumMetrics(),
		unitDivider:           unitDivider(),
		dimensions:            newDimensions(cfg.Dimensions),
		keyBuf:                bytes.NewBuffer(make([]byte, 0, 1024)),
		metricKeyToDimensions: metricKeyToDimensionsCache,
		ticker:                ticker,
		done:                  make(chan struct{}),
	}, nil
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
			case <-p.ticker.C:
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
// It aggregates the trace data to generate metrics, forwarding these metrics to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
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

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *connectorImp) buildMetrics() pmetric.Metrics {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("spanmetricsconnector")

	p.buildCallsMetrics(ilm)
	p.buildLatencyMetrics(ilm)

	return m
}

// buildCallsMetrics collects the raw call count metrics and builds
// a explicit or exponential buckets histogram scope metric.
func (p *connectorImp) buildLatencyMetrics(ilm pmetric.ScopeMetrics) {
	m := ilm.Metrics().AppendEmpty()
	m.SetName(buildMetricName(p.config.Namespace, metricNameLatency))
	m.SetUnit(p.config.Histogram.Unit)

	p.histograms.BuildMetrics(m, p.startTimestamp, p.config.GetAggregationTemporality())
}

// buildCallsMetrics collects the raw call count metrics and builds
// a sum scope metric.
func (p *connectorImp) buildCallsMetrics(ilm pmetric.ScopeMetrics) {
	m := ilm.Metrics().AppendEmpty()
	m.SetName(buildMetricName(p.config.Namespace, metricNameCalls))

	p.sums.BuildMetrics(m, p.startTimestamp, p.config.GetAggregationTemporality())
}

func (p *connectorImp) resetState() {
	// If delta metrics, reset accumulated data
	if p.config.GetAggregationTemporality() == pmetric.AggregationTemporalityDelta {
		p.histograms.Reset(false)
		p.sums.Reset()
		p.metricKeyToDimensions.Purge()
	} else {
		p.metricKeyToDimensions.RemoveEvictedItems()

		// Exemplars are only relevant to this batch of traces, so must be cleared within the lock
		p.histograms.Reset(true)
	}
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as name, kind, status_code and any additional
// dimensions the user has configured.
func (p *connectorImp) aggregateMetrics(traces ptrace.Traces) {
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
				latency := float64(0)
				startTime := span.StartTimestamp()
				endTime := span.EndTimestamp()
				if endTime > startTime {
					latency = float64(endTime-startTime) / float64(p.unitDivider)
				}
				key := p.buildKey(serviceName, span, p.dimensions, resourceAttr)

				attributes, ok := p.metricKeyToDimensions.Get(key)
				if !ok {
					attributes = p.buildAttributes(serviceName, span, resourceAttr)
					p.metricKeyToDimensions.Add(key, attributes)
				}

				// aggregate histogram metrics
				h := p.histograms.GetOrCreate(key, attributes)
				h.Observe(latency)
				if !span.TraceID().IsEmpty() {
					h.AddExemplar(span.TraceID(), span.SpanID(), latency)
				}

				// aggregate sums metrics
				s := p.sums.GetOrCreate(key, attributes)
				s.Add(1)
			}
		}
	}
}

func (p *connectorImp) buildAttributes(serviceName string, span ptrace.Span, resourceAttrs pcommon.Map) pcommon.Map {
	attr := pcommon.NewMap()
	attr.EnsureCapacity(4 + len(p.dimensions))
	attr.PutStr(serviceNameKey, serviceName)
	attr.PutStr(spanNameKey, span.Name())
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

// buildKey builds the metric key from the service name and span metadata such as name, kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func (p *connectorImp) buildKey(serviceName string, span ptrace.Span, optionalDims []dimension, resourceAttrs pcommon.Map) metrics.Key {
	p.keyBuf.Reset()
	concatDimensionValue(p.keyBuf, serviceName, false)
	concatDimensionValue(p.keyBuf, span.Name(), true)
	concatDimensionValue(p.keyBuf, traceutil.SpanKindStr(span.Kind()), true)
	concatDimensionValue(p.keyBuf, traceutil.StatusCodeStr(span.Status().Code()), true)

	for _, d := range optionalDims {
		if v, ok := getDimensionValue(d, span.Attributes(), resourceAttrs); ok {
			concatDimensionValue(p.keyBuf, v.AsString(), true)
		}
	}

	return metrics.Key(p.keyBuf.String())
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

// buildMetricName builds the namespace prefix for the metric name.
func buildMetricName(namespace string, name string) string {
	if namespace != "" {
		return namespace + "." + name
	}
	return name
}
