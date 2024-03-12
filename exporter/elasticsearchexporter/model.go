// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/json"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type mappingModel interface {
	encodeLog(pcommon.Resource, plog.LogRecord, pcommon.InstrumentationScope) ([]byte, error)
	encodeSpan(pcommon.Resource, ptrace.Span, pcommon.InstrumentationScope) ([]byte, error)
}

// encodeModel tries to keep the event as close to the original open telemetry semantics as is.
// No fields will be mapped by default.
//
// Field deduplication and dedotting of attributes is supported by the encodeModel.
//
// See: https://github.com/open-telemetry/oteps/blob/master/text/logs/0097-log-data-model.md
type encodeModel struct {
	dedup bool
	dedot bool
	mode  MappingMode
}

const (
	traceIDField   = "traceID"
	spanIDField    = "spanID"
	attributeField = "attribute"
)

func (m *encodeModel) encodeLog(resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) ([]byte, error) {
	var document objmodel.Document
	switch m.mode {
	case MappingECS:
		document = m.encodeLogECSMode(resource, record, scope, time.Now())
	default:
		document = m.encodeLogDefaultMode(resource, record, scope)
	}

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}

func (m *encodeModel) encodeLogDefaultMode(resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) objmodel.Document {
	var document objmodel.Document

	switch m.mode {
	case MappingECS:
		if record.Timestamp() != 0 {
			document.AddTimestamp("@timestamp", record.Timestamp())
		} else {
			document.AddTimestamp("@timestamp", record.ObservedTimestamp())
		}

		document.AddTraceID("trace.id", record.TraceID())
		document.AddSpanID("span.id", record.SpanID())

		if n := record.SeverityNumber(); n != plog.SeverityNumberUnspecified {
			document.AddInt("event.severity", int64(record.SeverityNumber()))
		}

		document.AddString("log.level", record.SeverityText())

		if record.Body().Type() == pcommon.ValueTypeStr {
			document.AddAttribute("message", record.Body())
		}

		fieldMapper := func(k string) string {
			switch k {
			case "exception.type":
				return "error.type"
			case "exception.message":
				return "error.message"
			case "exception.stacktrace":
				return "error.stack_trace"
			default:
				return k
			}
		}

		resource.Attributes().Range(func(k string, v pcommon.Value) bool {
			k = fieldMapper(k)
			document.AddAttribute(k, v)
			return true
		})
		scope.Attributes().Range(func(k string, v pcommon.Value) bool {
			k = fieldMapper(k)
			document.AddAttribute(k, v)
			return true
		})
		record.Attributes().Range(func(k string, v pcommon.Value) bool {
			k = fieldMapper(k)
			document.AddAttribute(k, v)
			return true
		})
	default:
		docTimeStamp := record.Timestamp()
		if docTimeStamp.AsTime().UnixNano() == 0 {
			docTimeStamp = record.ObservedTimestamp()
		}
		document.AddTimestamp("@timestamp", docTimeStamp) // We use @timestamp in order to ensure that we can index if the default data stream logs template is used.
		document.AddTraceID("TraceId", record.TraceID())
		document.AddSpanID("SpanId", record.SpanID())
		document.AddInt("TraceFlags", int64(record.Flags()))
		document.AddString("SeverityText", record.SeverityText())
		document.AddInt("SeverityNumber", int64(record.SeverityNumber()))
		document.AddAttribute("Body", record.Body())
		m.encodeAttributes(&document, record.Attributes())
		document.AddAttributes("Resource", resource.Attributes())
		document.AddAttributes("Scope", scopeToAttributes(scope))
	}

	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}

	return document

}

func (m *encodeModel) encodeLogECSMode(resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope, now time.Time) objmodel.Document {
	var document objmodel.Document

	// First, try to map resource-level attributes to ECS fields.
	resourceAttrsConversionMap := map[string]string{
		semconv.AttributeServiceInstanceID:     "service.node.name",
		semconv.AttributeDeploymentEnvironment: "service.environment",
		semconv.AttributeTelemetrySDKName:      "agent.name",
		semconv.AttributeTelemetrySDKVersion:   "agent.version",
		semconv.AttributeTelemetrySDKLanguage:  "service.language.name",
		semconv.AttributeCloudPlatform:         "cloud.service.name",
		semconv.AttributeContainerImageTags:    "container.image.tag",
		semconv.AttributeHostName:              "host.hostname",
		semconv.AttributeHostArch:              "host.architecture",
		semconv.AttributeProcessExecutablePath: "process.executable",
		semconv.AttributeOSType:                "os.platform",
		semconv.AttributeOSDescription:         "os.full",
	}
	mapLogAttributesToECS(&document, resource.Attributes(), resourceAttrsConversionMap)

	// Then, try to map scope-level attributes to ECS fields.
	scopeAttrsConversionMap := map[string]string{
		// None at the moment
	}
	mapLogAttributesToECS(&document, scope.Attributes(), scopeAttrsConversionMap)

	// Finally, try to map record-level attributes to ECS fields.
	recordAttrsConversionMap := map[string]string{
		// None at the moment
	}
	mapLogAttributesToECS(&document, record.Attributes(), recordAttrsConversionMap)

	// Handle special cases.
	document.Add("event.received", objmodel.TimestampValue(now))
	return document
}

func (m *encodeModel) encodeSpan(resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope) ([]byte, error) {
	var document objmodel.Document
	document.AddTimestamp("@timestamp", span.StartTimestamp()) // We use @timestamp in order to ensure that we can index if the default data stream logs template is used.
	document.AddTimestamp("EndTimestamp", span.EndTimestamp())
	document.AddTraceID("TraceId", span.TraceID())
	document.AddSpanID("SpanId", span.SpanID())
	document.AddSpanID("ParentSpanId", span.ParentSpanID())
	document.AddString("Name", span.Name())
	document.AddString("Kind", traceutil.SpanKindStr(span.Kind()))
	document.AddInt("TraceStatus", int64(span.Status().Code()))
	document.AddString("TraceStatusDescription", span.Status().Message())
	document.AddString("Link", spanLinksToString(span.Links()))
	m.encodeAttributes(&document, span.Attributes())
	document.AddAttributes("Resource", resource.Attributes())
	m.encodeEvents(&document, span.Events())
	document.AddInt("Duration", durationAsMicroseconds(span.StartTimestamp().AsTime(), span.EndTimestamp().AsTime())) // unit is microseconds
	document.AddAttributes("Scope", scopeToAttributes(scope))

	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}

func (m *encodeModel) encodeAttributes(document *objmodel.Document, attributes pcommon.Map) {
	key := "Attributes"
	if m.mode == MappingRaw {
		key = ""
	}
	document.AddAttributes(key, attributes)
}

func (m *encodeModel) encodeEvents(document *objmodel.Document, events ptrace.SpanEventSlice) {
	key := "Events"
	if m.mode == MappingRaw {
		key = ""
	}
	document.AddEvents(key, events)
}

func spanLinksToString(spanLinkSlice ptrace.SpanLinkSlice) string {
	linkArray := make([]map[string]any, 0, spanLinkSlice.Len())
	for i := 0; i < spanLinkSlice.Len(); i++ {
		spanLink := spanLinkSlice.At(i)
		link := map[string]any{}
		link[spanIDField] = traceutil.SpanIDToHexOrEmptyString(spanLink.SpanID())
		link[traceIDField] = traceutil.TraceIDToHexOrEmptyString(spanLink.TraceID())
		link[attributeField] = spanLink.Attributes().AsRaw()
		linkArray = append(linkArray, link)
	}
	linkArrayBytes, _ := json.Marshal(&linkArray)
	return string(linkArrayBytes)
}

// durationAsMicroseconds calculate span duration through end - start nanoseconds and converts time.Time to microseconds,
// which is the format the Duration field is stored in the Span.
func durationAsMicroseconds(start, end time.Time) int64 {
	return (end.UnixNano() - start.UnixNano()) / 1000
}

func scopeToAttributes(scope pcommon.InstrumentationScope) pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("name", scope.Name())
	attrs.PutStr("version", scope.Version())
	for k, v := range scope.Attributes().AsRaw() {
		attrs.PutStr(k, v.(string))
	}
	return attrs
}

func mapLogAttributesToECS(document *objmodel.Document, attrs pcommon.Map, conversionMap map[string]string) {
	if len(conversionMap) == 0 {
		// No conversions to be done; add all attributes at top level of
		// document.
		document.AddAttributes("", attrs)
		return
	}

	attrs.Range(func(k string, v pcommon.Value) bool {
		// If mapping to ECS key is found, use it.
		if ecsKey, exists := conversionMap[k]; exists {
			document.AddAttribute(ecsKey, v)
			return true
		}

		// Otherwise, add key at top level with attribute name as-is.
		document.AddAttribute(k, v)
		return true
	})
}
