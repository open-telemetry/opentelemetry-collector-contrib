// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

const (
	metricKeySeparator = string(byte(0))
)

type metricsConnector struct {
	lock   sync.Mutex
	config Config

	// Additional dimensions to add to metrics.
	dimensions []dimension

	keyBuf *bytes.Buffer

	metricsConsumer consumer.Metrics
	component.StartFunc
	component.ShutdownFunc

	exceptions map[string]*excVal

	logger *zap.Logger

	// The starting time of the data points.
	startTimestamp pcommon.Timestamp
}

type excVal struct {
	count int
	attrs pcommon.Map
}

func newMetricsConnector(logger *zap.Logger, config component.Config) (*metricsConnector, error) {
	cfg := config.(*Config)

	return &metricsConnector{
		logger:         logger,
		config:         *cfg,
		dimensions:     newDimensions(cfg.Dimensions),
		keyBuf:         bytes.NewBuffer(make([]byte, 0, 1024)),
		startTimestamp: pcommon.NewTimestampFromTime(time.Now()),
		exceptions:     make(map[string]*excVal),
	}, nil
}

// Capabilities implements the consumer interface.
func (c *metricsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate metrics.
func (c *metricsConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
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
				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					if event.Name() == eventNameExc {
						eventAttrs := event.Attributes()

						c.keyBuf.Reset()
						buildKey(c.keyBuf, serviceName, span, c.dimensions, eventAttrs)
						key := c.keyBuf.String()

						attrs := buildDimensionKVs(c.dimensions, serviceName, span, eventAttrs)
						c.addException(key, attrs)
					}
				}
			}
		}
	}
	return c.exportMetrics(ctx)
}

func (c *metricsConnector) exportMetrics(ctx context.Context) error {
	c.lock.Lock()
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("exceptionsconnector")

	if err := c.collectExceptions(ilm); err != nil {
		c.lock.Unlock()
		return err
	}
	c.lock.Unlock()

	if err := c.metricsConsumer.ConsumeMetrics(ctx, m); err != nil {
		c.logger.Error("failed to convert exceptions into metrics", zap.Error(err))
		return err
	}
	return nil
}

// collectExceptions collects the exception metrics data and writes it into the metrics object.
func (c *metricsConnector) collectExceptions(ilm pmetric.ScopeMetrics) error {
	mCalls := ilm.Metrics().AppendEmpty()
	mCalls.SetName("exceptions")
	mCalls.SetEmptySum().SetIsMonotonic(true)
	mCalls.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dps := mCalls.Sum().DataPoints()
	dps.EnsureCapacity(len(c.exceptions))
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for _, val := range c.exceptions {
		dpCalls := dps.AppendEmpty()
		dpCalls.SetStartTimestamp(c.startTimestamp)
		dpCalls.SetTimestamp(timestamp)

		dpCalls.SetIntValue(int64(val.count))

		val.attrs.CopyTo(dpCalls.Attributes())
	}
	return nil
}

func (c *metricsConnector) addException(excKey string, attrs pcommon.Map) {
	exc, ok := c.exceptions[excKey]
	if !ok {
		c.exceptions[excKey] = &excVal{
			count: 1,
			attrs: attrs,
		}
		return
	}
	exc.count++
}

func buildDimensionKVs(dimensions []dimension, serviceName string, span ptrace.Span, eventAttrs pcommon.Map) pcommon.Map {
	dims := pcommon.NewMap()
	dims.EnsureCapacity(3 + len(dimensions))
	dims.PutStr(serviceNameKey, serviceName)
	dims.PutStr(spanKindKey, traceutil.SpanKindStr(span.Kind()))
	dims.PutStr(statusCodeKey, traceutil.StatusCodeStr(span.Status().Code()))
	for _, d := range dimensions {
		if v, ok := getDimensionValue(d, span.Attributes(), eventAttrs); ok {
			v.CopyTo(dims.PutEmpty(d.name))
		}
	}
	return dims
}

// buildKey builds the metric key from the service name and span metadata such as kind, status_code and
// will attempt to add any additional dimensions the user has configured that match the span's attributes
// or resource attributes. If the dimension exists in both, the span's attributes, being the most specific, takes precedence.
//
// The metric key is a simple concatenation of dimension values, delimited by a null character.
func buildKey(dest *bytes.Buffer, serviceName string, span ptrace.Span, optionalDims []dimension, eventAttrs pcommon.Map) {
	concatDimensionValue(dest, serviceName, false)
	concatDimensionValue(dest, span.Name(), true)
	concatDimensionValue(dest, traceutil.SpanKindStr(span.Kind()), true)
	concatDimensionValue(dest, traceutil.StatusCodeStr(span.Status().Code()), true)

	for _, d := range optionalDims {
		if v, ok := getDimensionValue(d, span.Attributes(), eventAttrs); ok {
			concatDimensionValue(dest, v.AsString(), true)
		}
	}
}

func concatDimensionValue(dest *bytes.Buffer, value string, prefixSep bool) {
	if prefixSep {
		dest.WriteString(metricKeySeparator)
	}
	dest.WriteString(value)
}
