// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventionsv126 "go.opentelemetry.io/otel/semconv/v1.26.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type conversionEntry struct {
	to               string
	preserveOriginal bool
	skip             bool
	skipIfExists     bool
}

// resourceAttrsConversionMap contains conversions for resource-level attributes
// from their Semantic Conventions (SemConv) names to equivalent Elastic Common
// Schema (ECS) names.
// If the ECS field name is specified as an empty string (""), the converter will
// neither convert the SemConv key to the equivalent ECS name nor pass-through the
// SemConv key as-is to become the ECS name.
var resourceAttrsConversionMap = map[string]conversionEntry{
	string(conventions.ServiceInstanceIDKey):         {to: "service.node.name"},
	string(conventionsv126.DeploymentEnvironmentKey): {to: "service.environment"},
	string(conventions.DeploymentEnvironmentNameKey): {to: "service.environment"},
	string(conventions.TelemetrySDKNameKey):          {skip: true},
	string(conventions.TelemetrySDKLanguageKey):      {skip: true},
	string(conventions.TelemetrySDKVersionKey):       {skip: true},
	string(conventions.TelemetryDistroNameKey):       {skip: true},
	string(conventions.TelemetryDistroVersionKey):    {skip: true},
	string(conventions.CloudPlatformKey):             {to: "cloud.service.name"},
	string(conventions.ContainerImageTagsKey):        {to: "container.image.tag"},
	string(conventions.HostNameKey):                  {to: "host.hostname", preserveOriginal: true, skipIfExists: true},
	string(conventions.HostArchKey):                  {to: "host.architecture"},
	string(conventions.ProcessParentPIDKey):          {to: "process.parent.pid"},
	string(conventions.ProcessExecutableNameKey):     {to: "process.title"},
	string(conventions.ProcessExecutablePathKey):     {to: "process.executable"},
	string(conventions.ProcessCommandLineKey):        {to: "process.args"},
	string(conventions.ProcessRuntimeNameKey):        {to: "service.runtime.name"},
	string(conventions.ProcessRuntimeVersionKey):     {to: "service.runtime.version"},
	string(conventions.OSNameKey):                    {to: "host.os.name"},
	string(conventions.OSTypeKey):                    {to: "host.os.platform"},
	string(conventions.OSDescriptionKey):             {to: "host.os.full"},
	string(conventions.OSVersionKey):                 {to: "host.os.version"},
	string(conventions.ClientAddressKey):             {to: "client.ip"},
	string(conventions.SourceAddressKey):             {to: "source.ip"},
	string(conventions.K8SDeploymentNameKey):         {to: "kubernetes.deployment.name"},
	string(conventions.K8SNamespaceNameKey):          {to: "kubernetes.namespace"},
	string(conventions.K8SNodeNameKey):               {to: "kubernetes.node.name"},
	string(conventions.K8SPodNameKey):                {to: "kubernetes.pod.name"},
	string(conventions.K8SPodUIDKey):                 {to: "kubernetes.pod.uid"},
	string(conventions.K8SJobNameKey):                {to: "kubernetes.job.name"},
	string(conventions.K8SCronJobNameKey):            {to: "kubernetes.cronjob.name"},
	string(conventions.K8SStatefulSetNameKey):        {to: "kubernetes.statefulset.name"},
	string(conventions.K8SReplicaSetNameKey):         {to: "kubernetes.replicaset.name"},
	string(conventions.K8SDaemonSetNameKey):          {to: "kubernetes.daemonset.name"},
	string(conventions.K8SContainerNameKey):          {to: "kubernetes.container.name"},
	string(conventions.K8SClusterNameKey):            {to: "orchestrator.cluster.name"},
	string(conventions.FaaSInstanceKey):              {to: "faas.id"},
	string(conventions.FaaSTriggerKey):               {to: "faas.trigger.type"},
}

var ErrInvalidTypeForBodyMapMode = errors.New("invalid log record body type for 'bodymap' mapping mode")

// documentEncoder is an interface for encoding signals to Elasticsearch documents.
type documentEncoder interface {
	encodeLog(encodingContext, plog.LogRecord, elasticsearch.Index, *bytes.Buffer) error
	encodeSpan(encodingContext, ptrace.Span, elasticsearch.Index, *bytes.Buffer) error
	encodeSpanEvent(encodingContext, ptrace.Span, ptrace.SpanEvent, elasticsearch.Index, *bytes.Buffer) error
	encodeMetrics(_ encodingContext, _ []datapoints.DataPoint, validationErrors *[]error, _ elasticsearch.Index, _ *bytes.Buffer) (map[string]string, error)
	encodeProfile(_ encodingContext, _ pprofile.ProfilesDictionary, _ pprofile.Profile, _ func(*bytes.Buffer, string, string) error) error
}

type encodingContext struct {
	resource          pcommon.Resource
	resourceSchemaURL string
	scope             pcommon.InstrumentationScope
	scopeSchemaURL    string
}

func newEncoder(mode MappingMode) (documentEncoder, error) {
	switch mode {
	case MappingNone:
		return legacyModeEncoder{
			metricsUnsupportedEncoder:  metricsUnsupportedEncoder{mode: mode},
			profilesUnsupportedEncoder: profilesUnsupportedEncoder{mode: mode},
			nonOTelSpanEncoder: nonOTelSpanEncoder{
				attributesPrefix: "Attributes",
				eventsPrefix:     "Events",
			},
			attributesPrefix: "Attributes",
		}, nil
	case MappingRaw:
		return legacyModeEncoder{
			metricsUnsupportedEncoder:  metricsUnsupportedEncoder{mode: mode},
			profilesUnsupportedEncoder: profilesUnsupportedEncoder{mode: mode},
			nonOTelSpanEncoder: nonOTelSpanEncoder{
				attributesPrefix: "",
				eventsPrefix:     "",
			},
			attributesPrefix: "",
		}, nil
	case MappingECS:
		return ecsModeEncoder{
			profilesUnsupportedEncoder: profilesUnsupportedEncoder{mode: mode},
		}, nil
	case MappingBodyMap:
		return bodymapModeEncoder{
			metricsUnsupportedEncoder:  metricsUnsupportedEncoder{mode: mode},
			profilesUnsupportedEncoder: profilesUnsupportedEncoder{mode: mode},
		}, nil
	case MappingOTel:
		ser, err := otelserializer.New()
		if err != nil {
			return nil, err
		}
		return otelModeEncoder{serializer: ser}, nil
	}
	return nil, fmt.Errorf("unknown mapping mode %q (%d)", mode, int(mode))
}

type legacyModeEncoder struct {
	nonOTelSpanEncoder
	nopSpanEventEncoder
	metricsUnsupportedEncoder
	profilesUnsupportedEncoder
	attributesPrefix string
}

type ecsModeEncoder struct {
	ecsDataPointsEncoder
	nopSpanEventEncoder
	profilesUnsupportedEncoder
}

type bodymapModeEncoder struct {
	metricsUnsupportedEncoder
	profilesUnsupportedEncoder
}

type otelModeEncoder struct {
	serializer *otelserializer.Serializer
}

const (
	traceIDField   = "traceID"
	spanIDField    = "spanID"
	attributeField = "attribute"
)

func (e legacyModeEncoder) encodeLog(ec encodingContext, record plog.LogRecord, idx elasticsearch.Index, buf *bytes.Buffer) error {
	var document objmodel.Document

	docTimeStamp := record.Timestamp()
	if docTimeStamp.AsTime().UnixNano() == 0 {
		docTimeStamp = record.ObservedTimestamp()
	}
	// We use @timestamp in order to ensure that we can index if the default data stream logs template is used.
	document.AddTimestamp("@timestamp", docTimeStamp)
	document.AddTraceID("TraceId", record.TraceID())
	document.AddSpanID("SpanId", record.SpanID())
	document.AddInt("TraceFlags", int64(record.Flags()))
	document.AddString("SeverityText", record.SeverityText())
	document.AddInt("SeverityNumber", int64(record.SeverityNumber()))
	document.AddAttribute("Body", record.Body())
	document.AddAttributes("Resource", ec.resource.Attributes())
	document.AddAttributes("Scope", scopeToAttributes(ec.scope))
	encodeAttributes(e.attributesPrefix, &document, record.Attributes(), idx)

	return document.Serialize(buf, false)
}

func (ecsModeEncoder) encodeLog(
	ec encodingContext,
	record plog.LogRecord,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) error {
	var document objmodel.Document

	// First, try to map resource-level attributes to ECS fields.
	encodeAttributesECSMode(&document, ec.resource.Attributes(), resourceAttrsConversionMap)

	// Then, try to map scope-level attributes to ECS fields.
	scopeAttrsConversionMap := map[string]conversionEntry{
		// None at the moment
	}
	encodeAttributesECSMode(&document, ec.scope.Attributes(), scopeAttrsConversionMap)

	// Finally, try to map record-level attributes to ECS fields.
	recordAttrsConversionMap := map[string]conversionEntry{
		"event.name":                                {to: "event.action"},
		string(conventions.ExceptionMessageKey):     {to: "error.message"},
		string(conventions.ExceptionStacktraceKey):  {to: "error.stacktrace"},
		string(conventions.ExceptionTypeKey):        {to: "error.type"},
		string(conventionsv126.ExceptionEscapedKey): {to: "event.error.exception.handled"},
		string(conventions.HTTPResponseBodySizeKey): {to: "http.response.encoded_body_size"},
	}
	encodeAttributesECSMode(&document, record.Attributes(), recordAttrsConversionMap)
	addDataStreamAttributes(&document, "", idx)

	// Handle special cases.
	encodeLogAgentNameECSMode(&document, ec.resource)
	encodeLogAgentVersionECSMode(&document, ec.resource)
	encodeHostOsTypeECSMode(&document, ec.resource)
	encodeLogTimestampECSMode(&document, record)
	document.AddTraceID("trace.id", record.TraceID())
	document.AddSpanID("span.id", record.SpanID())
	if n := record.SeverityNumber(); n != plog.SeverityNumberUnspecified {
		document.AddInt("event.severity", int64(record.SeverityNumber()))
	}

	document.AddString("log.level", record.SeverityText())

	if record.Body().Type() == pcommon.ValueTypeStr {
		document.AddAttribute("message", record.Body())
	}

	return document.Serialize(buf, true)
}

func (ecsModeEncoder) encodeSpan(
	ec encodingContext,
	span ptrace.Span,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) error {
	var document objmodel.Document

	// First, try to map resource-level attributes to ECS fields.
	encodeAttributesECSMode(&document, ec.resource.Attributes(), resourceAttrsConversionMap)

	// Then, try to map scope-level attributes to ECS fields.
	scopeAttrsConversionMap := map[string]conversionEntry{
		// None at the moment
	}
	encodeAttributesECSMode(&document, ec.scope.Attributes(), scopeAttrsConversionMap)

	// Finally, try to map record-level attributes to ECS fields.

	// determine the correct message queue name based on the trace type (Elastic span or transaction)
	messageQueueName := "span.message.queue.name"
	processor, _ := span.Attributes().Get("processor.event")
	if processor.Str() == "transaction" {
		messageQueueName = "transaction.message.queue.name"
	}

	spanAttrsConversionMap := map[string]conversionEntry{
		string(conventions.MessagingDestinationNameKey): {to: messageQueueName},
		string(conventions.MessagingOperationNameKey):   {to: "span.action"},
		string(conventionsv126.DBSystemKey):             {to: "span.db.type"},
		string(conventions.DBNamespaceKey):              {to: "span.db.instance"},
		string(conventions.DBQueryTextKey):              {to: "span.db.statement"},
		string(conventions.HTTPResponseBodySizeKey):     {to: "http.response.encoded_body_size"},
	}

	// Handle special cases.
	encodeAttributesECSMode(&document, span.Attributes(), spanAttrsConversionMap)
	encodeHostOsTypeECSMode(&document, ec.resource)
	addDataStreamAttributes(&document, "", idx)

	document.AddTimestamp("@timestamp", span.StartTimestamp())
	document.AddTraceID("trace.id", span.TraceID())
	document.AddSpanID("span.id", span.SpanID())
	document.AddString("span.name", span.Name())
	document.AddSpanID("parent.id", span.ParentSpanID())
	if span.Status().Code() == ptrace.StatusCodeOk {
		document.AddString("event.outcome", "success")
	} else if span.Status().Code() == ptrace.StatusCodeError {
		document.AddString("event.outcome", "failure")
	}
	document.AddLinks("span.links", span.Links())
	if spanKind := spanKindToECSStr(span.Kind()); spanKind != "" {
		document.AddString("span.kind", spanKind)
	}

	return document.Serialize(buf, true)
}

// spanKindToECSStr converts an OTel SpanKind to its ECS equivalent string representation defined here:
// https://github.com/elastic/apm-data/blob/main/input/elasticapm/internal/modeldecoder/v2/decoder.go#L1665-L1669
func spanKindToECSStr(sk ptrace.SpanKind) string {
	switch sk {
	case ptrace.SpanKindInternal:
		return "INTERNAL"
	case ptrace.SpanKindServer:
		return "SERVER"
	case ptrace.SpanKindClient:
		return "CLIENT"
	case ptrace.SpanKindProducer:
		return "PRODUCER"
	case ptrace.SpanKindConsumer:
		return "CONSUMER"
	}
	return ""
}

func (e otelModeEncoder) encodeLog(
	ec encodingContext,
	record plog.LogRecord,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) error {
	return e.serializer.SerializeLog(
		ec.resource, ec.resourceSchemaURL,
		ec.scope, ec.scopeSchemaURL,
		record, idx, buf,
	)
}

func (e otelModeEncoder) encodeSpan(
	ec encodingContext,
	span ptrace.Span,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) error {
	return e.serializer.SerializeSpan(
		ec.resource, ec.resourceSchemaURL,
		ec.scope, ec.scopeSchemaURL,
		span, idx, buf,
	)
}

func (e otelModeEncoder) encodeSpanEvent(
	ec encodingContext,
	span ptrace.Span,
	spanEvent ptrace.SpanEvent,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) error {
	e.serializer.SerializeSpanEvent(
		ec.resource, ec.resourceSchemaURL,
		ec.scope, ec.scopeSchemaURL,
		span, spanEvent, idx, buf,
	)
	return nil
}

func (e otelModeEncoder) encodeMetrics(
	ec encodingContext,
	dataPoints []datapoints.DataPoint,
	validationErrors *[]error,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) (map[string]string, error) {
	return e.serializer.SerializeMetrics(
		ec.resource, ec.resourceSchemaURL,
		ec.scope, ec.scopeSchemaURL,
		dataPoints, validationErrors, idx, buf,
	)
}

func (e otelModeEncoder) encodeProfile(
	ec encodingContext,
	dic pprofile.ProfilesDictionary,
	profile pprofile.Profile,
	pushData func(*bytes.Buffer, string, string) error,
) error {
	return e.serializer.SerializeProfile(dic, ec.resource, ec.scope, profile, pushData)
}

func (bodymapModeEncoder) encodeLog(
	_ encodingContext,
	record plog.LogRecord,
	_ elasticsearch.Index,
	buf *bytes.Buffer,
) error {
	body := record.Body()
	if body.Type() != pcommon.ValueTypeMap {
		return fmt.Errorf("%w: %q", ErrInvalidTypeForBodyMapMode, body.Type())
	}
	serializer.Map(body.Map(), buf)
	return nil
}

func (bodymapModeEncoder) encodeSpan(encodingContext, ptrace.Span, elasticsearch.Index, *bytes.Buffer) error {
	return errors.New("bodymap mode does not support encoding spans")
}

func (bodymapModeEncoder) encodeSpanEvent(encodingContext, ptrace.Span, ptrace.SpanEvent, elasticsearch.Index, *bytes.Buffer) error {
	return errors.New("bodymap mode does not support encoding span events")
}

type metricsUnsupportedEncoder struct {
	mode MappingMode
}

//nolint:unparam // result 0 is expected to always be nil
func (e metricsUnsupportedEncoder) encodeMetrics(
	_ encodingContext,
	_ []datapoints.DataPoint,
	_ *[]error,
	_ elasticsearch.Index,
	_ *bytes.Buffer,
) (map[string]string, error) {
	return nil, fmt.Errorf("mapping mode %q (%d) does not support metrics", e.mode, int(e.mode))
}

type profilesUnsupportedEncoder struct {
	mode MappingMode
}

func (e profilesUnsupportedEncoder) encodeProfile(
	_ encodingContext, _ pprofile.ProfilesDictionary, _ pprofile.Profile, _ func(*bytes.Buffer, string, string) error,
) error {
	return fmt.Errorf("mapping mode %q (%d) does not support profiles", e.mode, int(e.mode))
}

type nonOTelSpanEncoder struct {
	attributesPrefix string
	eventsPrefix     string
	dedot            bool
}

func (e nonOTelSpanEncoder) encodeSpan(
	ec encodingContext,
	span ptrace.Span,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) error {
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
	document.AddAttributes("Resource", ec.resource.Attributes())
	document.AddInt("Duration", durationAsMicroseconds(span.StartTimestamp().AsTime(), span.EndTimestamp().AsTime())) // unit is microseconds
	document.AddAttributes("Scope", scopeToAttributes(ec.scope))
	encodeAttributes(e.attributesPrefix, &document, span.Attributes(), idx)
	document.AddEvents(e.eventsPrefix, span.Events())
	return document.Serialize(buf, e.dedot)
}

type ecsDataPointsEncoder struct{}

func (ecsDataPointsEncoder) encodeMetrics(
	ec encodingContext,
	dataPoints []datapoints.DataPoint,
	validationErrors *[]error,
	idx elasticsearch.Index,
	buf *bytes.Buffer,
) (map[string]string, error) {
	dp0 := dataPoints[0]
	var document objmodel.Document
	encodeAttributesECSMode(&document, ec.resource.Attributes(), resourceAttrsConversionMap)
	document.AddTimestamp("@timestamp", dp0.Timestamp())
	document.AddAttributes("", dp0.Attributes())
	addDataStreamAttributes(&document, "", idx)

	for _, dp := range dataPoints {
		value, err := dp.Value()
		if err != nil {
			*validationErrors = append(*validationErrors, err)
			continue
		}
		document.AddAttribute(dp.Metric().Name(), value)
	}
	err := document.Serialize(buf, true)

	return document.DynamicTemplates(), err
}

func addDataStreamAttributes(document *objmodel.Document, key string, idx elasticsearch.Index) {
	if idx.IsDataStream() {
		if key != "" {
			key += "."
		}
		document.AddString(key+elasticsearch.DataStreamType, idx.Type)
		document.AddString(key+elasticsearch.DataStreamDataset, idx.Dataset)
		document.AddString(key+elasticsearch.DataStreamNamespace, idx.Namespace)
	}
}

// nopSpanEventEncoder is embedded in all non-OTel encoders,
// since only OTel mapping mode currently encodes span events
// as separate documents. In all others they are stored within
// the span document.
type nopSpanEventEncoder struct{}

func (nopSpanEventEncoder) encodeSpanEvent(encodingContext, ptrace.Span, ptrace.SpanEvent, elasticsearch.Index, *bytes.Buffer) error {
	return nil
}

func encodeAttributes(prefix string, document *objmodel.Document, attributes pcommon.Map, idx elasticsearch.Index) {
	document.AddAttributes(prefix, attributes)
	addDataStreamAttributes(document, prefix, idx)
}

func spanLinksToString(spanLinkSlice ptrace.SpanLinkSlice) string {
	linkArray := make([]map[string]any, 0, spanLinkSlice.Len())
	for _, spanLink := range spanLinkSlice.All() {
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

	scope.Attributes().CopyTo(attrs)

	attrs.PutStr("name", scope.Name())
	attrs.PutStr("version", scope.Version())

	return attrs
}

func encodeAttributesECSMode(document *objmodel.Document, attrs pcommon.Map, conversionMap map[string]conversionEntry) {
	if len(conversionMap) == 0 {
		// No conversions to be done; add all attributes at top level of
		// document.
		document.AddAttributes("", attrs)
		return
	}

	for k, v := range attrs.All() {
		// If ECS key is found for current k in conversion map, use it.
		if c, exists := conversionMap[k]; exists {
			if c.skip {
				// Skip the conversion for this k.
				continue
			}
			if !c.skipIfExists {
				document.AddAttribute(c.to, v)
			} else if _, exists := attrs.Get(c.to); !exists {
				document.AddAttribute(c.to, v)
			}

			if c.preserveOriginal {
				document.AddAttribute(k, v)
			}
			continue
		}

		// Otherwise, add key at top level with attribute name as-is.
		document.AddAttribute(k, v)
	}
}

func encodeLogAgentNameECSMode(document *objmodel.Document, resource pcommon.Resource) {
	// Parse out telemetry SDK name, language, and distro name from resource
	// attributes, setting defaults as needed.
	telemetrySdkName := "otlp"
	var telemetrySdkLanguage, telemetryDistroName string

	attrs := resource.Attributes()
	if v, exists := attrs.Get(string(conventions.TelemetrySDKNameKey)); exists {
		telemetrySdkName = v.Str()
	}
	if v, exists := attrs.Get(string(conventions.TelemetrySDKLanguageKey)); exists {
		telemetrySdkLanguage = v.Str()
	}
	if v, exists := attrs.Get(string(conventions.TelemetryDistroNameKey)); exists {
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

	// Set agent name in document.
	document.AddString("agent.name", agentName)
}

func encodeLogAgentVersionECSMode(document *objmodel.Document, resource pcommon.Resource) {
	attrs := resource.Attributes()

	if telemetryDistroVersion, exists := attrs.Get(string(conventions.TelemetryDistroVersionKey)); exists {
		document.AddString("agent.version", telemetryDistroVersion.Str())
		return
	}

	if telemetrySdkVersion, exists := attrs.Get(string(conventions.TelemetrySDKVersionKey)); exists {
		document.AddString("agent.version", telemetrySdkVersion.Str())
		return
	}
}

func encodeHostOsTypeECSMode(document *objmodel.Document, resource pcommon.Resource) {
	// https://www.elastic.co/guide/en/ecs/current/ecs-os.html#field-os-type:
	//
	// "One of these following values should be used (lowercase): linux, macos, unix, windows.
	// If the OS you’re dealing with is not in the list, the field should not be populated."

	var ecsHostOsType string
	if semConvOsType, exists := resource.Attributes().Get(string(conventions.OSTypeKey)); exists {
		switch semConvOsType.Str() {
		case "windows", "linux":
			ecsHostOsType = semConvOsType.Str()
		case "darwin":
			ecsHostOsType = "macos"
		case "aix", "hpux", "solaris":
			ecsHostOsType = "unix"
		}
	}

	if semConvOsName, exists := resource.Attributes().Get(string(conventions.OSNameKey)); exists {
		switch semConvOsName.Str() {
		case "Android":
			ecsHostOsType = "android"
		case "iOS":
			ecsHostOsType = "ios"
		}
	}

	if ecsHostOsType == "" {
		return
	}
	document.AddString("host.os.type", ecsHostOsType)
}

func encodeLogTimestampECSMode(document *objmodel.Document, record plog.LogRecord) {
	if record.Timestamp() != 0 {
		document.AddTimestamp("@timestamp", record.Timestamp())
		return
	}

	document.AddTimestamp("@timestamp", record.ObservedTimestamp())
}
