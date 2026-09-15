// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"

import (
	"bytes"
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

const (
	metricKeySeparator = string(byte(0))
)

type metricsConnector struct {
	lock   sync.Mutex
	config Config

	// Additional dimensions to add to metrics.
	dimensions []pdatautil.Dimension

	keyBuf *bytes.Buffer

	metricsConsumer consumer.Metrics
	component.StartFunc
	component.ShutdownFunc

	exceptions map[string]*exception

	logger *zap.Logger

	// The starting time of the data points.
	startTimestamp pcommon.Timestamp
}

type exception struct {
	count     int
	attrs     pcommon.Map
	exemplars pmetric.ExemplarSlice
}

func newMetricsConnector(logger *zap.Logger, config component.Config) *metricsConnector {
	cfg := config.(*Config)

	return &metricsConnector{
		logger:         logger,
		config:         *cfg,
		dimensions:     newDimensions(cfg.Dimensions),
		keyBuf:         bytes.NewBuffer(make([]byte, 0, 1024)),
		startTimestamp: pcommon.NewTimestampFromTime(time.Now()),
		exceptions:     make(map[string]*exception),
	}
}

// Capabilities implements the consumer interface.
func (*metricsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
// It aggregates the trace data to generate metrics.
func (c *metricsConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		resourceAttr := rspans.Resource().Attributes()
		serviceAttr, ok := resourceAttr.Get(string(conventions.ServiceNameKey))
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
						spanName := span.Name()
						spanKind := traceutil.SpanKindStr(span.Kind())
						statusCode := traceutil.StatusCodeStr(span.Status().Code())

						c.keyBuf.Reset()
						buildKey(c.keyBuf, serviceName, spanName, spanKind, statusCode, c.dimensions, span.Attributes(), eventAttrs, resourceAttr)
						key := c.keyBuf.String()

						attrs := buildDimensionKVs(c.dimensions, serviceName, spanName, spanKind, statusCode, span.Attributes(), eventAttrs, resourceAttr)
						exc := c.addException(key, attrs)
						c.addExemplar(exc, span.TraceID(), span.SpanID())
					}
				}
			}
		}
	}
	return c.exportMetrics(ctx)
}

// ConsumeLogs implements the consumer.Logs interface: aggregates records with
// event.name == "exception" into the same "exceptions" metric ConsumeTraces produces. There's no
// span to read span.name/span.kind/status.code from, so those dimensions are omitted.
func (c *metricsConnector) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rlogs := logs.ResourceLogs().At(i)
		resourceAttr := rlogs.Resource().Attributes()
		serviceAttr, ok := resourceAttr.Get(string(conventions.ServiceNameKey))
		if !ok {
			continue
		}
		serviceName := serviceAttr.Str()
		slSlice := rlogs.ScopeLogs()
		for j := 0; j < slSlice.Len(); j++ {
			records := slSlice.At(j).LogRecords()
			for k := 0; k < records.Len(); k++ {
				lr := records.At(k)
				if lr.EventName() != eventNameExc {
					continue
				}
				lrAttrs := lr.Attributes()

				c.keyBuf.Reset()
				buildKey(c.keyBuf, serviceName, "", "", "", c.dimensions, lrAttrs, resourceAttr)
				key := c.keyBuf.String()

				attrs := buildDimensionKVs(c.dimensions, serviceName, "", "", "", lrAttrs, resourceAttr)
				exc := c.addException(key, attrs)
				c.addExemplar(exc, lr.TraceID(), lr.SpanID())
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
	for _, exc := range c.exceptions {
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(c.startTimestamp)
		dp.SetTimestamp(timestamp)
		dp.SetIntValue(int64(exc.count))
		for i := 0; i < exc.exemplars.Len(); i++ {
			exc.exemplars.At(i).SetTimestamp(timestamp)
		}
		dp.Exemplars().EnsureCapacity(exc.exemplars.Len())
		exc.exemplars.CopyTo(dp.Exemplars())
		exc.attrs.CopyTo(dp.Attributes())
		// Reset the exemplars for the next batch of spans.
		exc.exemplars = pmetric.NewExemplarSlice()
	}
	return nil
}

func (c *metricsConnector) addException(excKey string, attrs pcommon.Map) *exception {
	exc, ok := c.exceptions[excKey]
	if !ok {
		c.exceptions[excKey] = &exception{
			count:     1,
			attrs:     attrs,
			exemplars: pmetric.NewExemplarSlice(),
		}
		return c.exceptions[excKey]
	}
	exc.count++
	return exc
}

func (c *metricsConnector) addExemplar(exc *exception, traceID pcommon.TraceID, spanID pcommon.SpanID) {
	if !c.config.Exemplars.Enabled || traceID.IsEmpty() {
		return
	}
	e := exc.exemplars.AppendEmpty()
	e.SetTraceID(traceID)
	e.SetSpanID(spanID)
	e.SetDoubleValue(float64(exc.count))
}

// buildDimensionKVs builds the dimensions/attributes for an exception metric data point.
// spanName/spanKind/statusCode are omitted when empty (logs-sourced exceptions have none).
func buildDimensionKVs(dimensions []pdatautil.Dimension, serviceName, spanName, spanKind, statusCode string, attrSets ...pcommon.Map) pcommon.Map {
	dims := pcommon.NewMap()
	dims.EnsureCapacity(4 + len(dimensions))
	dims.PutStr(serviceNameKey, serviceName)
	if spanName != "" {
		dims.PutStr(spanNameKey, spanName)
	}
	if spanKind != "" {
		dims.PutStr(spanKindKey, spanKind)
	}
	if statusCode != "" {
		dims.PutStr(statusCodeKey, statusCode)
	}
	for _, d := range dimensions {
		if v, ok := pdatautil.GetDimensionValue(d, attrSets...); ok {
			v.CopyTo(dims.PutEmpty(d.Name))
		}
	}
	return dims
}

// buildKey builds the metric key: service name, span metadata, then any configured dimensions
// found in attrSets (searched in order, earlier sets take precedence). Values are concatenated,
// delimited by a null character. spanName/spanKind/statusCode are omitted when empty.
func buildKey(dest *bytes.Buffer, serviceName, spanName, spanKind, statusCode string, optionalDims []pdatautil.Dimension, attrSets ...pcommon.Map) {
	concatDimensionValue(dest, serviceName, false)
	if spanName != "" {
		concatDimensionValue(dest, spanName, true)
	}
	if spanKind != "" {
		concatDimensionValue(dest, spanKind, true)
	}
	if statusCode != "" {
		concatDimensionValue(dest, statusCode, true)
	}

	for _, d := range optionalDims {
		if v, ok := pdatautil.GetDimensionValue(d, attrSets...); ok {
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
