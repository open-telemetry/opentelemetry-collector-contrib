// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

var (
	// resourceAttrsConversionMap contains conversions for resource-level attributes
	// from their Semantic Conventions (SemConv) names to equivalent Elastic Common
	// Schema (ECS) names.
	// If the ECS field name is specified as an empty string (""), the converter will
	// neither convert the SemConv key to the equivalent ECS name nor pass-through the
	// SemConv key as-is to become the ECS name.
	resourceAttrsConversionMap = map[string]string{
		semconv.AttributeServiceInstanceID:      "service.node.name",
		semconv.AttributeDeploymentEnvironment:  "service.environment",
		semconv.AttributeTelemetrySDKName:       "",
		semconv.AttributeTelemetrySDKLanguage:   "",
		semconv.AttributeTelemetrySDKVersion:    "",
		semconv.AttributeTelemetryDistroName:    "",
		semconv.AttributeTelemetryDistroVersion: "",
		semconv.AttributeCloudPlatform:          "cloud.service.name",
		semconv.AttributeContainerImageTags:     "container.image.tag",
		semconv.AttributeHostName:               "host.hostname",
		semconv.AttributeHostArch:               "host.architecture",
		semconv.AttributeProcessExecutablePath:  "process.executable",
		semconv.AttributeProcessRuntimeName:     "service.runtime.name",
		semconv.AttributeProcessRuntimeVersion:  "service.runtime.version",
		semconv.AttributeOSName:                 "host.os.name",
		semconv.AttributeOSType:                 "host.os.platform",
		semconv.AttributeOSDescription:          "host.os.full",
		semconv.AttributeOSVersion:              "host.os.version",
		"k8s.namespace.name":                    "kubernetes.namespace",
		"k8s.node.name":                         "kubernetes.node.name",
		"k8s.pod.name":                          "kubernetes.pod.name",
		"k8s.pod.uid":                           "kubernetes.pod.uid",
	}

	// resourceAttrsConversionMap contains conversions for log record attributes
	// from their Semantic Conventions (SemConv) names to equivalent Elastic Common
	// Schema (ECS) names.
	recordAttrsConversionMap = map[string]string{
		"event.name":                         "event.action",
		semconv.AttributeExceptionMessage:    "error.message",
		semconv.AttributeExceptionStacktrace: "error.stacktrace",
		semconv.AttributeExceptionType:       "error.type",
		semconv.AttributeExceptionEscaped:    "event.error.exception.handled",
	}

	resourceAttrsConversionMapRemapper = keyRemapperFromMap(resourceAttrsConversionMap)
	recordAttrsConversionMapRemapper   = keyRemapperFromMap(recordAttrsConversionMap)
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
		m.encodeLogECSMode(&document, resource, record, scope)
	default:
		m.encodeLogDefaultMode(&document, resource, record, scope)
	}

	var buf bytes.Buffer
	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}

func (m *encodeModel) encodeLogDefaultMode(document *objmodel.Document, resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) {
	document.AddMultiple(
		// We use @timestamp in order to ensure that we can index if the
		// default data stream logs template is used.
		objmodel.NewKV("@timestamp", getTimestampForECS(record)),
		objmodel.NewKV("TraceId", objmodel.NonZeroStringValue(record.TraceID().String())),
		objmodel.NewKV("SpanId", objmodel.NonZeroStringValue(record.SpanID().String())),
		objmodel.NewKV("TraceFlags", objmodel.IntValue(int64(record.Flags()))),
		objmodel.NewKV("SeverityText", objmodel.NonZeroStringValue(record.SeverityText())),
		objmodel.NewKV("SeverityNumber", objmodel.IntValue(int64(record.SeverityNumber()))),
		objmodel.NewKV("Body", objmodel.NewPValueProcessorValue(record.Body())),
		// Add scope name and version as additional scope attributes.
		// Empty values are also allowed to be added.
		objmodel.NewKV("Scope.name", objmodel.StringValue(scope.Name())),
		objmodel.NewKV("Scope.version", objmodel.StringValue(scope.Version())),
		objmodel.NewKV("Resource", objmodel.NewMapProcessorValue(resource.Attributes(), nil)),
		objmodel.NewKV("Scope", objmodel.NewMapProcessorValue(scope.Attributes(), nil)),
		objmodel.NewKV(m.emptyIfRaw("Attributes"), objmodel.NewMapProcessorValue(record.Attributes(), nil)),
	)
}

func (m *encodeModel) encodeLogECSMode(document *objmodel.Document, resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) {
	document.AddMultiple(
		objmodel.NewKV("@timestamp", getTimestampForECS(record)),
		objmodel.NewKV("agent.name", getAgentNameForECS(resource)),
		objmodel.NewKV("agent.version", getAgentVersionForECS(resource)),
		objmodel.NewKV("host.os.type", getHostOsTypeForECS(resource)),
		objmodel.NewKV("trace.id", objmodel.NonZeroStringValue(record.TraceID().String())),
		objmodel.NewKV("span.id", objmodel.NonZeroStringValue(record.SpanID().String())),
		objmodel.NewKV("log.level", objmodel.NonZeroStringValue(record.SeverityText())),
		objmodel.NewKV("event.severity", objmodel.NonZeroIntValue(int64(record.SeverityNumber()))),
		objmodel.NewKV("message", objmodel.NewPValueProcessorValue(record.Body())),
		objmodel.NewKV("", objmodel.NewMapProcessorValue(resource.Attributes(), resourceAttrsConversionMapRemapper)),
		objmodel.NewKV("", objmodel.NewMapProcessorValue(scope.Attributes(), nil)),
		objmodel.NewKV("", objmodel.NewMapProcessorValue(record.Attributes(), recordAttrsConversionMapRemapper)),
	)
}

func (m *encodeModel) encodeSpan(resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope) ([]byte, error) {
	var document objmodel.Document
	document.AddMultiple(
		objmodel.NewKV("@timestamp", objmodel.TimestampValue(span.StartTimestamp())),
		objmodel.NewKV("EndTimestamp", objmodel.TimestampValue(span.EndTimestamp())),
		objmodel.NewKV("TraceId", objmodel.NonZeroStringValue(span.TraceID().String())),
		objmodel.NewKV("SpanId", objmodel.NonZeroStringValue(span.SpanID().String())),
		objmodel.NewKV("ParentSpanId", objmodel.NonZeroStringValue(span.ParentSpanID().String())),
		objmodel.NewKV("Name", objmodel.NonZeroStringValue(span.Name())),
		objmodel.NewKV("Kind", objmodel.NonZeroStringValue(traceutil.SpanKindStr(span.Kind()))),
		objmodel.NewKV("TraceStatus", objmodel.IntValue(int64(span.Status().Code()))),
		objmodel.NewKV("TraceStatusDescription", objmodel.NonZeroStringValue(span.Status().Message())),
		objmodel.NewKV("Link", objmodel.NonZeroStringValue(spanLinksToString(span.Links()))),
		objmodel.NewKV("Duration", objmodel.IntValue(durationAsMicroseconds(span.StartTimestamp(), span.EndTimestamp()))),
		// Add scope name and version as additional scope attributes
		// Empty values are also allowed to be added.
		objmodel.NewKV("Scope.name", objmodel.StringValue(scope.Name())),
		objmodel.NewKV("Scope.version", objmodel.StringValue(scope.Version())),
		objmodel.NewKV("Resource", objmodel.NewMapProcessorValue(resource.Attributes(), nil)),
		objmodel.NewKV("Scope", objmodel.NewMapProcessorValue(scope.Attributes(), nil)),
		objmodel.NewKV(m.emptyIfRaw("Attributes"), objmodel.NewMapProcessorValue(span.Attributes(), nil)),
		objmodel.NewKV(m.emptyIfRaw("Events"), objmodel.NewSpansProcessorValue(span.Events())),
	)

	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}

// emptyIfRaw is a utility function for defining attribute root keys for raw
// mapping mode where they are supposed to be empty.
func (m *encodeModel) emptyIfRaw(k string) string {
	if m.mode == MappingRaw {
		return ""
	}
	return k
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

func getAgentNameForECS(resource pcommon.Resource) objmodel.Value {
	// Parse out telemetry SDK name, language, and distro name from resource
	// attributes, setting defaults as needed.
	telemetrySdkName := "otlp"
	var telemetrySdkLanguage, telemetryDistroName string

	attrs := resource.Attributes()
	if v, exists := attrs.Get(semconv.AttributeTelemetrySDKName); exists {
		telemetrySdkName = v.Str()
	}
	if v, exists := attrs.Get(semconv.AttributeTelemetrySDKLanguage); exists {
		telemetrySdkLanguage = v.Str()
	}
	if v, exists := attrs.Get(semconv.AttributeTelemetryDistroName); exists {
		telemetryDistroName = v.Str()
		if telemetrySdkLanguage == "" {
			telemetrySdkLanguage = "unknown"
		}
	}

	// Construct agent name from telemetry SDK name, language, and distro name.
	agentName := telemetrySdkName
	if telemetryDistroName != "" {
		agentName = fmt.Sprintf("%s/%s/%s", agentName, telemetrySdkLanguage, telemetryDistroName)
	} else if telemetrySdkLanguage != "" {
		agentName = fmt.Sprintf("%s/%s", agentName, telemetrySdkLanguage)
	}

	return objmodel.NonZeroStringValue(agentName)
}

func getAgentVersionForECS(resource pcommon.Resource) objmodel.Value {
	attrs := resource.Attributes()

	if telemetryDistroVersion, exists := attrs.Get(semconv.AttributeTelemetryDistroVersion); exists {
		return objmodel.NonZeroStringValue(telemetryDistroVersion.Str())
	}

	if telemetrySdkVersion, exists := attrs.Get(semconv.AttributeTelemetrySDKVersion); exists {
		return objmodel.NonZeroStringValue(telemetrySdkVersion.Str())
	}
	return objmodel.NilValue
}

func getHostOsTypeForECS(resource pcommon.Resource) objmodel.Value {
	// https://www.elastic.co/guide/en/ecs/current/ecs-os.html#field-os-type:
	//
	// "One of these following values should be used (lowercase): linux, macos, unix, windows.
	// If the OS you’re dealing with is not in the list, the field should not be populated."

	var ecsHostOsType string
	if semConvOsType, exists := resource.Attributes().Get(semconv.AttributeOSType); exists {
		switch semConvOsType.Str() {
		case "windows", "linux":
			ecsHostOsType = semConvOsType.Str()
		case "darwin":
			ecsHostOsType = "macos"
		case "aix", "hpux", "solaris":
			ecsHostOsType = "unix"
		}
	}

	if semConvOsName, exists := resource.Attributes().Get(semconv.AttributeOSName); exists {
		switch semConvOsName.Str() {
		case "Android":
			ecsHostOsType = "android"
		case "iOS":
			ecsHostOsType = "ios"
		}
	}

	if ecsHostOsType == "" {
		return objmodel.NilValue
	}
	return objmodel.NonZeroStringValue(ecsHostOsType)
}

func getTimestampForECS(record plog.LogRecord) objmodel.Value {
	if record.Timestamp() != 0 {
		return objmodel.TimestampValue(record.Timestamp())
	}

	return objmodel.TimestampValue(record.ObservedTimestamp())
}

// durationAsMicroseconds calculate span duration through end - start
// nanoseconds and converts time.Time to microseconds, which is the format
// the Duration field is stored in the Span.
func durationAsMicroseconds(start, end pcommon.Timestamp) int64 {
	return (end.AsTime().UnixNano() - start.AsTime().UnixNano()) / 1000
}

func keyRemapperFromMap(m map[string]string) func(string) string {
	return func(orig string) string {
		if remappedKey, ok := m[orig]; ok {
			return remappedKey
		}
		return orig
	}
}
