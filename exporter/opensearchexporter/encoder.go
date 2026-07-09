// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/pool"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/serializer"
)

var errInvalidTypeForBodyMapMode = errors.New("invalid log record body type for 'bodymap' mapping mode")

type mappingModel interface {
	encodeLog(resource pcommon.Resource,
		scope pcommon.InstrumentationScope,
		schemaURL string,
		record plog.LogRecord) ([]byte, error)
	encodeTrace(resource pcommon.Resource,
		scope pcommon.InstrumentationScope,
		schemaURL string,
		record ptrace.Span) ([]byte, error)
}

type bodyMapMappingModel struct {
	bufferPool *pool.BufferPool
}

func (*bodyMapMappingModel) encodeTrace(
	_ pcommon.Resource,
	_ pcommon.InstrumentationScope,
	_ string,
	_ ptrace.Span,
) ([]byte, error) {
	return nil, fmt.Errorf("mapping mode '%s' does not support encoding traces", MappingBodyMap.String())
}

func (m *bodyMapMappingModel) encodeLog(
	_ pcommon.Resource,
	_ pcommon.InstrumentationScope,
	_ string,
	record plog.LogRecord,
) ([]byte, error) {
	body := record.Body()
	if body.Type() != pcommon.ValueTypeMap {
		return nil, fmt.Errorf("%w: %q", errInvalidTypeForBodyMapMode, body.Type().String())
	}
	pooledBuf := m.bufferPool.NewPooledBuffer()
	defer pooledBuf.Recycle()

	serializer.Map(body.Map(), pooledBuf.Buffer)
	// Copy bytes to avoid holding reference to pooled buffer
	result := make([]byte, pooledBuf.Buffer.Len())
	copy(result, pooledBuf.Buffer.Bytes())
	return result, nil
}

// encodeModel supports multiple encoding OpenTelemetry signals to multiple schemas.
type encodeModel struct {
	dedup             bool
	dedot             bool
	sso               bool
	otelV1            bool
	flattenAttributes bool
	timestampField    string
	unixTime          bool

	dataset   string
	namespace string
}

func (m *encodeModel) encodeLog(resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	record plog.LogRecord,
) ([]byte, error) {
	if m.otelV1 {
		return m.encodeLogOTelV1(resource, scope, schemaURL, record)
	}
	if m.sso {
		return m.encodeLogSSO(resource, scope, schemaURL, record)
	}

	return m.encodeLogDataModel(resource, record)
}

// encodeLogSSO encodes a plog.LogRecord following the Simple Schema for Observability.
// See: https://github.com/opensearch-project/opensearch-catalog/tree/main/docs/schema/observability
func (m *encodeModel) encodeLogSSO(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	record plog.LogRecord,
) ([]byte, error) {
	sso := ssoRecord{}
	sso.Attributes = record.Attributes().AsRaw()
	sso.Body = record.Body().AsString()
	sso.EventName = record.EventName()

	now := time.Now()
	ts := record.Timestamp().AsTime()
	sso.ObservedTimestamp = &now
	sso.Timestamp = &ts

	sso.Resource = attributesToMapString(resource.Attributes())
	sso.SchemaURL = schemaURL
	sso.SpanID = record.SpanID().String()
	sso.TraceID = record.TraceID().String()

	ds := dataStream{}
	if m.dataset != "" {
		ds.Dataset = m.dataset
	}

	if m.namespace != "" {
		ds.Namespace = m.namespace
	}

	if ds != (dataStream{}) {
		ds.Type = "record"
		sso.Attributes["data_stream"] = ds
	}

	sso.InstrumentationScope.Name = scope.Name()
	sso.InstrumentationScope.Version = scope.Version()
	sso.InstrumentationScope.SchemaURL = schemaURL
	sso.InstrumentationScope.Attributes = scope.Attributes().AsRaw()

	sso.Severity.Text = record.SeverityText()
	sso.Severity.Number = int64(record.SeverityNumber())

	return json.Marshal(sso)
}

// encodeLogDataModel encodes a plog.LogRecord following the Log Data Model.
// See: https://github.com/open-telemetry/oteps/blob/main/text/logs/0097-log-data-model.md
func (m *encodeModel) encodeLogDataModel(resource pcommon.Resource, record plog.LogRecord) ([]byte, error) {
	var document objmodel.Document
	if m.flattenAttributes {
		document = objmodel.DocumentFromAttributes(resource.Attributes())
	} else {
		document.AddAttributes("Attributes", resource.Attributes())
	}
	timestampField := "@timestamp"

	if m.timestampField != "" {
		timestampField = m.timestampField
	}

	if m.unixTime {
		document.AddInt(timestampField, epochMilliTimestamp(record))
	} else {
		document.AddTimestamp(timestampField, record.Timestamp())
	}
	document.AddTraceID("TraceId", record.TraceID())
	document.AddSpanID("SpanId", record.SpanID())
	document.AddInt("TraceFlags", int64(record.Flags()))
	document.AddString("SeverityText", record.SeverityText())
	document.AddInt("SeverityNumber", int64(record.SeverityNumber()))
	document.AddString("EventName", record.EventName())
	document.AddAttribute("Body", record.Body())
	if m.flattenAttributes {
		document.AddAttributes("", record.Attributes())
	} else {
		document.AddAttributes("Attributes", record.Attributes())
	}

	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}

// encodeTrace encodes a ptrace.Span following the Simple Schema For Observability
// See: https://github.com/opensearch-project/opensearch-catalog/tree/main/docs/schema/observability
func (m *encodeModel) encodeTrace(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	span ptrace.Span,
) ([]byte, error) {
	if m.otelV1 {
		return m.encodeTraceOTelV1(resource, scope, schemaURL, span)
	}

	sso := ssoSpan{}
	sso.Attributes = span.Attributes().AsRaw()
	sso.DroppedAttributesCount = span.DroppedAttributesCount()
	sso.DroppedEventsCount = span.DroppedEventsCount()
	sso.DroppedLinksCount = span.DroppedLinksCount()
	sso.EndTime = span.EndTimestamp().AsTime()
	sso.Kind = span.Kind().String()
	sso.Name = span.Name()
	sso.ParentSpanID = span.ParentSpanID().String()
	sso.Resource = attributesToMapString(resource.Attributes())
	sso.SpanID = span.SpanID().String()
	sso.StartTime = span.StartTimestamp().AsTime()
	sso.Status.Code = span.Status().Code().String()
	sso.Status.Message = span.Status().Message()
	sso.TraceID = span.TraceID().String()
	sso.TraceState = span.TraceState().AsRaw()

	if span.Events().Len() > 0 {
		sso.Events = make([]ssoSpanEvent, span.Events().Len())
		for i := 0; i < span.Events().Len(); i++ {
			e := span.Events().At(i)
			ssoEvent := &sso.Events[i]
			ssoEvent.Attributes = e.Attributes().AsRaw()
			ssoEvent.DroppedAttributesCount = e.DroppedAttributesCount()
			ssoEvent.Name = e.Name()
			ts := e.Timestamp().AsTime()
			if ts.Unix() != 0 {
				ssoEvent.Timestamp = &ts
			} else {
				now := time.Now()
				ssoEvent.ObservedTimestamp = &now
			}
		}
	}

	ds := dataStream{}
	if m.dataset != "" {
		ds.Dataset = m.dataset
	}

	if m.namespace != "" {
		ds.Namespace = m.namespace
	}

	if ds != (dataStream{}) {
		ds.Type = "span"
		sso.Attributes["data_stream"] = ds
	}

	sso.InstrumentationScope.Name = scope.Name()
	sso.InstrumentationScope.DroppedAttributesCount = scope.DroppedAttributesCount()
	sso.InstrumentationScope.Version = scope.Version()
	sso.InstrumentationScope.SchemaURL = schemaURL
	sso.InstrumentationScope.Attributes = scope.Attributes().AsRaw()

	if span.Links().Len() > 0 {
		sso.Links = make([]ssoSpanLinks, span.Links().Len())
		for i := 0; i < span.Links().Len(); i++ {
			link := span.Links().At(i)
			ssoLink := &sso.Links[i]
			ssoLink.Attributes = link.Attributes().AsRaw()
			ssoLink.DroppedAttributesCount = link.DroppedAttributesCount()
			ssoLink.TraceID = link.TraceID().String()
			ssoLink.TraceState = link.TraceState().AsRaw()
			ssoLink.SpanID = link.SpanID().String()
		}
	}
	return json.Marshal(sso)
}

// encodeLogOTelV1 encodes a plog.LogRecord following the Data Prepper OTel v1 logs schema.
func (*encodeModel) encodeLogOTelV1(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	record plog.LogRecord,
) ([]byte, error) {
	ts := record.Timestamp().AsTime()
	doc := otelV1LogRecord{
		Timestamp:              ts,
		Time:                   ts,
		ObservedTime:           record.ObservedTimestamp().AsTime(),
		Body:                   record.Body().AsString(),
		EventName:              record.EventName(),
		TraceID:                record.TraceID().String(),
		SpanID:                 record.SpanID().String(),
		Flags:                  int64(record.Flags()),
		DroppedAttributesCount: record.DroppedAttributesCount(),
		Attributes:             record.Attributes().AsRaw(),
		Severity: otelV1Severity{
			Number: int32(record.SeverityNumber()),
			Text:   record.SeverityText(),
		},
		Resource: otelV1Resource{
			Attributes:             resource.Attributes().AsRaw(),
			DroppedAttributesCount: resource.DroppedAttributesCount(),
			SchemaURL:              schemaURL,
		},
		InstrumentationScope: otelV1Scope{
			Name:                   scope.Name(),
			Version:                scope.Version(),
			SchemaURL:              schemaURL,
			Attributes:             scope.Attributes().AsRaw(),
			DroppedAttributesCount: scope.DroppedAttributesCount(),
		},
	}
	return json.Marshal(doc)
}

// encodeTraceOTelV1 encodes a ptrace.Span following the Data Prepper OTel v1 traces schema.
func (*encodeModel) encodeTraceOTelV1(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	span ptrace.Span,
) ([]byte, error) {
	startTime := span.StartTimestamp().AsTime()
	endTime := span.EndTimestamp().AsTime()
	durationInNanos := endTime.UnixNano() - startTime.UnixNano()
	statusCode := int32(span.Status().Code())

	doc := otelV1Span{
		TraceID:                span.TraceID().String(),
		SpanID:                 span.SpanID().String(),
		ParentSpanID:           span.ParentSpanID().String(),
		Name:                   span.Name(),
		Kind:                   span.Kind().String(),
		TraceState:             span.TraceState().AsRaw(),
		StartTime:              startTime,
		EndTime:                endTime,
		Timestamp:              startTime,
		Time:                   startTime,
		DurationInNanos:        durationInNanos,
		DroppedAttributesCount: span.DroppedAttributesCount(),
		DroppedEventsCount:     span.DroppedEventsCount(),
		DroppedLinksCount:      span.DroppedLinksCount(),
		Attributes:             span.Attributes().AsRaw(),
		Status: otelV1SpanStatus{
			Code:    statusCode,
			Message: span.Status().Message(),
		},
		Resource: otelV1Resource{
			Attributes:             resource.Attributes().AsRaw(),
			DroppedAttributesCount: resource.DroppedAttributesCount(),
			SchemaURL:              schemaURL,
		},
		InstrumentationScope: otelV1Scope{
			Name:                   scope.Name(),
			Version:                scope.Version(),
			SchemaURL:              schemaURL,
			Attributes:             scope.Attributes().AsRaw(),
			DroppedAttributesCount: scope.DroppedAttributesCount(),
		},
	}

	// Extract serviceName from resource attributes
	if sn, ok := resource.Attributes().Get("service.name"); ok {
		doc.ServiceName = sn.AsString()
	}

	// Root span: populate traceGroup fields
	if span.ParentSpanID().IsEmpty() {
		doc.TraceGroup = span.Name()
		doc.TraceGroupFields = &otelV1TraceGroup{
			EndTime:         endTime,
			DurationInNanos: durationInNanos,
			StatusCode:      statusCode,
		}
	}

	// Events
	if span.Events().Len() > 0 {
		doc.Events = make([]otelV1SpanEvent, span.Events().Len())
		for i := 0; i < span.Events().Len(); i++ {
			e := span.Events().At(i)
			doc.Events[i] = otelV1SpanEvent{
				Name:                   e.Name(),
				Attributes:             e.Attributes().AsRaw(),
				DroppedAttributesCount: e.DroppedAttributesCount(),
				Time:                   e.Timestamp().AsTime(),
			}
		}
	}

	// Links
	if span.Links().Len() > 0 {
		doc.Links = make([]otelV1SpanLink, span.Links().Len())
		for i := 0; i < span.Links().Len(); i++ {
			l := span.Links().At(i)
			doc.Links[i] = otelV1SpanLink{
				TraceID:                l.TraceID().String(),
				SpanID:                 l.SpanID().String(),
				TraceState:             l.TraceState().AsRaw(),
				Attributes:             l.Attributes().AsRaw(),
				DroppedAttributesCount: l.DroppedAttributesCount(),
			}
		}
	}

	return json.Marshal(doc)
}

func epochMilliTimestamp(record plog.LogRecord) int64 {
	return record.Timestamp().AsTime().UnixMilli()
}
