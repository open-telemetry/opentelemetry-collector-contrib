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

package otel2influx // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/otel2influx"

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"
)

type OtelTracesToLineProtocol struct {
	logger          common.Logger
	dependencyGraph DependencyGraph
	w               InfluxWriter
}

func NewOtelTracesToLineProtocol(logger common.Logger, w InfluxWriter) (*OtelTracesToLineProtocol, error) {
	// TODO make dependency graph optional
	// TODO add other dependency graph schema(ta)
	dependencyGraph, err := NewJaegerDependencyGraph(logger, 1000, 100, w)
	if err != nil {
		return nil, err
	}

	return &OtelTracesToLineProtocol{
		logger:          logger,
		dependencyGraph: dependencyGraph,
		w:               w,
	}, nil
}

func (c *OtelTracesToLineProtocol) Start(ctx context.Context, host component.Host) error {
	c.logger.Debug("starting otel traces to lp")
	return c.dependencyGraph.Start(ctx, host)
}

func (c *OtelTracesToLineProtocol) Shutdown(ctx context.Context) error {
	return c.dependencyGraph.Shutdown(ctx)
}

func (c *OtelTracesToLineProtocol) WriteTraces(ctx context.Context, td ptrace.Traces) error {
	batch := c.w.NewBatch()
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpans := td.ResourceSpans().At(i)
		resourceFields := convertResourceFields(resourceSpans.Resource())
		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			scopeFields := convertScopeFields(scopeSpans.Scope())
			for k, v := range resourceFields {
				scopeFields[k] = v
			}
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)
				c.dependencyGraph.ReportSpan(ctx, span, resourceSpans.Resource())
				if err := c.writeSpan(ctx, span, scopeFields, batch); err != nil {
					return fmt.Errorf("failed to convert OTLP span to line protocol: %w", err)
				}
			}
		}
	}
	return batch.FlushBatch(ctx)
}

func (c *OtelTracesToLineProtocol) writeSpan(ctx context.Context, span ptrace.Span, scopeFields map[string]interface{}, batch InfluxWriterBatch) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var rerr error
			switch v := r.(type) {
			case error:
				rerr = v
			case string:
				rerr = errors.New(v)
			default:
				rerr = fmt.Errorf("%+v", r)
			}
			err = multierr.Combine(err, rerr)
		}

		if err != nil && !consumererror.IsPermanent(err) {
			c.logger.Debug(err.Error())
			err = nil
		}
	}()

	traceID := span.TraceID()
	if traceID.IsEmpty() {
		return errors.New("span has no trace ID")
	}
	spanID := span.SpanID()
	if spanID.IsEmpty() {
		return errors.New("span has no span ID")
	}

	measurement := common.MeasurementSpans
	tags := map[string]string{
		common.AttributeTraceID: hex.EncodeToString(traceID[:]),
		common.AttributeSpanID:  hex.EncodeToString(spanID[:]),
	}
	fields := make(map[string]interface{}, len(scopeFields)+span.Attributes().Len()+9)
	for k, v := range scopeFields {
		fields[k] = v
	}

	if traceState := span.TraceState().AsRaw(); traceState != "" {
		fields[common.AttributeTraceState] = traceState
	}
	if parentSpanID := span.ParentSpanID(); !parentSpanID.IsEmpty() {
		fields[common.AttributeParentSpanID] = hex.EncodeToString(parentSpanID[:])
	}
	if name := span.Name(); name != "" {
		fields[common.AttributeName] = name
	}
	if kind := span.Kind(); kind != ptrace.SpanKindUnspecified {
		fields[common.AttributeSpanKind] = kind.String()
	}

	ts := span.StartTimestamp().AsTime()
	if ts.IsZero() {
		return errors.New("span has no timestamp")
	}

	if endTime := span.EndTimestamp().AsTime(); !endTime.IsZero() {
		fields[common.AttributeEndTimeUnixNano] = endTime.UnixNano()
		fields[common.AttributeDurationNano] = endTime.Sub(ts).Nanoseconds()
	}

	droppedAttributesCount := uint64(span.DroppedAttributesCount())
	if span.Attributes().Len() > 0 {
		marshalledAttributes, marshalErr := json.Marshal(span.Attributes().AsRaw())
		if marshalErr != nil {
			c.logger.Debug("failed to marshal attributes to JSON", marshalErr)
			droppedAttributesCount += uint64(span.Attributes().Len())
		} else {
			fields[common.AttributeAttributes] = string(marshalledAttributes)
		}
	}
	if droppedAttributesCount > 0 {
		fields[common.AttributeDroppedAttributesCount] = droppedAttributesCount
	}

	droppedEventsCount := uint64(span.DroppedEventsCount())
	for i := 0; i < span.Events().Len(); i++ {
		if err = c.writeSpanEvent(ctx, traceID, spanID, span.Events().At(i), batch); err != nil {
			droppedEventsCount++
			c.logger.Debug("invalid span event", err)
		}
	}
	if droppedEventsCount > 0 {
		fields[common.AttributeDroppedEventsCount] = droppedEventsCount
	}

	droppedLinksCount := uint64(span.DroppedLinksCount())
	for i := 0; i < span.Links().Len(); i++ {
		if err = c.writeSpanLink(ctx, traceID, spanID, ts, span.Links().At(i), batch); err != nil {
			droppedLinksCount++
			c.logger.Debug("invalid span link", err)
		}
	}
	if droppedLinksCount > 0 {
		fields[common.AttributeDroppedLinksCount] = droppedLinksCount
	}

	status := span.Status()
	switch status.Code() {
	case ptrace.StatusCodeUnset:
	case ptrace.StatusCodeOk, ptrace.StatusCodeError:
		fields[semconv.OtelStatusCode] = status.Code().String()
	default:
		c.logger.Debug("status code not recognized", "code", status.Code())
	}
	if message := status.Message(); message != "" {
		fields[semconv.OtelStatusDescription] = message
	}

	if err := batch.WritePoint(ctx, measurement, tags, fields, ts, common.InfluxMetricValueTypeUntyped); err != nil {
		return fmt.Errorf("failed to write point for span: %w", err)
	}

	return nil
}

func (c *OtelTracesToLineProtocol) writeSpanEvent(ctx context.Context, traceID pcommon.TraceID, spanID pcommon.SpanID, spanEvent ptrace.SpanEvent, batch InfluxWriterBatch) error {
	fields := make(map[string]interface{}, 2)
	if name := spanEvent.Name(); name != "" {
		fields[common.AttributeName] = name
	}

	droppedAttributesCount := uint64(spanEvent.DroppedAttributesCount())
	attributes := make(map[string]interface{}, spanEvent.Attributes().Len())
	spanEvent.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			droppedAttributesCount++
			c.logger.Debug("span event attribute key is empty")
		} else if v, err := AttributeValueToInfluxFieldValue(v); err != nil {
			droppedAttributesCount++
			c.logger.Debug("invalid span event attribute value", err)
		} else {
			attributes[k] = v
		}
		return true
	})
	if len(attributes) > 0 {
		marshalledAttributes, err := json.Marshal(attributes)
		if err != nil {
			c.logger.Debug("failed to marshal attributes to JSON", err)
			droppedAttributesCount += uint64(len(attributes))
		} else {
			fields[common.AttributeAttributes] = string(marshalledAttributes)
		}
	}
	if droppedAttributesCount > 0 {
		fields[common.AttributeDroppedAttributesCount] = droppedAttributesCount
	}

	tags := map[string]string{
		common.AttributeTraceID: hex.EncodeToString(traceID[:]),
		common.AttributeSpanID:  hex.EncodeToString(spanID[:]),
	}

	err := batch.WritePoint(ctx, common.MeasurementLogs, tags, fields, spanEvent.Timestamp().AsTime(), common.InfluxMetricValueTypeUntyped)
	if err != nil {
		return fmt.Errorf("failed to write point for span event: %w", err)
	}
	return nil
}

func (c *OtelTracesToLineProtocol) writeSpanLink(ctx context.Context, traceID pcommon.TraceID, spanID pcommon.SpanID, ts time.Time, spanLink ptrace.SpanLink, batch InfluxWriterBatch) error {
	fields := make(map[string]interface{}, 2)

	linkedTraceID := spanLink.TraceID()
	if linkedTraceID.IsEmpty() {
		return errors.New("span link has no trace ID")
	}
	linkedSpanID := spanLink.SpanID()
	if linkedSpanID.IsEmpty() {
		return errors.New("span link has no span ID")
	}

	tags := map[string]string{
		common.AttributeTraceID:       hex.EncodeToString(traceID[:]),
		common.AttributeSpanID:        hex.EncodeToString(spanID[:]),
		common.AttributeLinkedTraceID: hex.EncodeToString(linkedTraceID[:]),
		common.AttributeLinkedSpanID:  hex.EncodeToString(linkedSpanID[:]),
	}

	if traceState := spanLink.TraceState().AsRaw(); traceState != "" {
		fields[common.AttributeTraceState] = traceState
	}

	droppedAttributesCount := uint64(spanLink.DroppedAttributesCount())
	attributes := make(map[string]interface{}, spanLink.Attributes().Len())
	spanLink.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			droppedAttributesCount++
			c.logger.Debug("span link attribute key is empty")
		} else if v, err := AttributeValueToInfluxFieldValue(v); err != nil {
			droppedAttributesCount++
			c.logger.Debug("invalid span link attribute value", err)
		} else {
			attributes[k] = v
		}
		return true
	})
	if len(attributes) > 0 {
		marshalledAttributes, err := json.Marshal(attributes)
		if err != nil {
			c.logger.Debug("failed to marshal attributes to JSON", err)
			droppedAttributesCount += uint64(len(attributes))
		} else {
			fields[common.AttributeAttributes] = string(marshalledAttributes)
		}
	}
	if droppedAttributesCount > 0 {
		fields[common.AttributeDroppedAttributesCount] = droppedAttributesCount
	}

	if err := batch.WritePoint(ctx, common.MeasurementSpanLinks, tags, fields, ts, common.InfluxMetricValueTypeUntyped); err != nil {
		return fmt.Errorf("failed to write point for span link: %w", err)
	}
	return nil
}
