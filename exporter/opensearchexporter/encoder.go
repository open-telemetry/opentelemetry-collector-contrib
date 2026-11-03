// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/pool"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/serializer"
)

var errInvalidTypeForBodyMapMode = errors.New("invalid log record body type for 'bodymap' mapping mode")

// resolveAttributeKeyConflicts rewrites attribute keys that would otherwise
// cause an OpenSearch mapping conflict.
//
// OpenSearch expands dots in JSON field names into nested objects during
// dynamic mapping. A flat OTel attribute map that contains both a concrete key
// (e.g. "code.function") and a longer key that uses it as an object prefix
// (e.g. "code.function.name") makes OpenSearch try to map
// "attributes.code.function" as both a concrete value and an object, which it
// rejects with a mapper_parsing_exception. This is common while migrating
// between semantic-convention versions (code.function -> code.function.name).
//
// To keep the document indexable, the concrete value is moved under a ".value"
// sub-key ("code.function" -> "code.function.value"), mirroring the behavior
// of the ECS mapping mode's objmodel.Dedup step. The rewrite only triggers when
// a conflicting sibling is present in the same map, i.e. only for documents
// OpenSearch would otherwise reject, so well-formed documents are unchanged.
// It recurses into nested maps and arrays of maps so conflicts within map-typed
// attribute values are handled too.
func resolveAttributeKeyConflicts(m map[string]any) {
	if len(m) == 0 {
		return
	}

	// Handle nested maps and arrays of maps first.
	for _, v := range m {
		switch vv := v.(type) {
		case map[string]any:
			resolveAttributeKeyConflicts(vv)
		case []any:
			for _, e := range vv {
				if em, ok := e.(map[string]any); ok {
					resolveAttributeKeyConflicts(em)
				}
			}
		}
	}

	// Repeatedly rename the shortest conflicting key until the map is stable.
	// Renaming can, in pathological cases, create a new adjacency, so re-scan
	// with a fresh, sorted key set after each rename.
	for {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		renamed := false
		for i := 0; i < len(keys)-1; i++ {
			key, next := keys[i], keys[i+1]
			// next must use key as a strict, dot-delimited object prefix.
			if len(key) >= len(next) || !strings.HasPrefix(next, key) || next[len(key)] != '.' {
				continue
			}
			// Only a concrete (non-object) value conflicts with the prefix use.
			if _, isObj := m[key].(map[string]any); isObj {
				continue
			}
			target := key + ".value"
			if _, exists := m[target]; !exists {
				m[target] = m[key]
			}
			// If target already exists it is an object being built from other
			// keys; drop the concrete value rather than clobber it.
			delete(m, key)
			renamed = true
			break
		}
		if !renamed {
			return
		}
	}
}

type mappingModel interface {
	encodeLog(resource pcommon.Resource,
		scope pcommon.InstrumentationScope,
		schemaURL string,
		record plog.LogRecord) ([]byte, error)
	encodeTrace(resource pcommon.Resource,
		scope pcommon.InstrumentationScope,
		schemaURL string,
		record ptrace.Span) ([]byte, error)
	encodeMetric(resource pcommon.Resource,
		scope pcommon.InstrumentationScope,
		schemaURL string,
		metric pmetric.Metric,
		dp metricDataPoint) ([]byte, error)
}

// metricDataPoint is the interface shared by all pmetric data point types
// (NumberDataPoint, HistogramDataPoint, ExponentialHistogramDataPoint and
// SummaryDataPoint).
type metricDataPoint interface {
	Attributes() pcommon.Map
	StartTimestamp() pcommon.Timestamp
	Timestamp() pcommon.Timestamp
	Flags() pmetric.DataPointFlags
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

func (*bodyMapMappingModel) encodeMetric(
	_ pcommon.Resource,
	_ pcommon.InstrumentationScope,
	_ string,
	_ pmetric.Metric,
	_ metricDataPoint,
) ([]byte, error) {
	return nil, fmt.Errorf("mapping mode '%s' does not support encoding metrics", MappingBodyMap.String())
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
	resolveAttributeKeyConflicts(sso.Attributes)
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
	resolveAttributeKeyConflicts(sso.InstrumentationScope.Attributes)

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
	resolveAttributeKeyConflicts(sso.Attributes)
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
			resolveAttributeKeyConflicts(ssoEvent.Attributes)
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
	resolveAttributeKeyConflicts(sso.InstrumentationScope.Attributes)

	if span.Links().Len() > 0 {
		sso.Links = make([]ssoSpanLinks, span.Links().Len())
		for i := 0; i < span.Links().Len(); i++ {
			link := span.Links().At(i)
			ssoLink := &sso.Links[i]
			ssoLink.Attributes = link.Attributes().AsRaw()
			resolveAttributeKeyConflicts(ssoLink.Attributes)
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
	resolveAttributeKeyConflicts(doc.Attributes)
	resolveAttributeKeyConflicts(doc.InstrumentationScope.Attributes)
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
	resolveAttributeKeyConflicts(doc.Attributes)
	resolveAttributeKeyConflicts(doc.InstrumentationScope.Attributes)

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
			resolveAttributeKeyConflicts(doc.Events[i].Attributes)
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
			resolveAttributeKeyConflicts(doc.Links[i].Attributes)
		}
	}

	return json.Marshal(doc)
}

func epochMilliTimestamp(record plog.LogRecord) int64 {
	return record.Timestamp().AsTime().UnixMilli()
}
