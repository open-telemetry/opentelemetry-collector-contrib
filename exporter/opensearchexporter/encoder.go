// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"encoding/json"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/objmodel"
)

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

// encodeModel supports multiple encoding OpenTelemetry signals to multiple schemas.
type encodeModel struct {
	dedup             bool
	dedot             bool
	sso               bool
	flattenAttributes bool
	timestampField    string
	unixTime          bool

	dataset   string
	namespace string
}

func (m *encodeModel) encodeLog(resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	record plog.LogRecord) ([]byte, error) {
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

func epochMilliTimestamp(record plog.LogRecord) int64 {
	return record.Timestamp().AsTime().UnixMilli()
}
