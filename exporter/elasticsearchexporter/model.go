// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"math"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/exphistogram"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

// resourceAttrsConversionMap contains conversions for resource-level attributes
// from their Semantic Conventions (SemConv) names to equivalent Elastic Common
// Schema (ECS) names.
// If the ECS field name is specified as an empty string (""), the converter will
// neither convert the SemConv key to the equivalent ECS name nor pass-through the
// SemConv key as-is to become the ECS name.
var resourceAttrsConversionMap = map[string]string{
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
	semconv.AttributeK8SDeploymentName:      "kubernetes.deployment.name",
	semconv.AttributeK8SNamespaceName:       "kubernetes.namespace",
	semconv.AttributeK8SNodeName:            "kubernetes.node.name",
	semconv.AttributeK8SPodName:             "kubernetes.pod.name",
	semconv.AttributeK8SPodUID:              "kubernetes.pod.uid",
	semconv.AttributeK8SJobName:             "kubernetes.job.name",
	semconv.AttributeK8SCronJobName:         "kubernetes.cronjob.name",
	semconv.AttributeK8SStatefulSetName:     "kubernetes.statefulset.name",
	semconv.AttributeK8SReplicaSetName:      "kubernetes.replicaset.name",
	semconv.AttributeK8SDaemonSetName:       "kubernetes.daemonset.name",
	semconv.AttributeK8SContainerName:       "kubernetes.container.name",
	semconv.AttributeK8SClusterName:         "orchestrator.cluster.name",
}

// resourceAttrsToPreserve contains conventions that should be preserved in ECS mode.
// This can happen when an attribute needs to be mapped to an ECS equivalent but
// at the same time be preserved to its original form.
var resourceAttrsToPreserve = map[string]bool{
	semconv.AttributeHostName: true,
}

var ErrInvalidTypeForBodyMapMode = errors.New("invalid log record body type for 'bodymap' mapping mode")

type mappingModel interface {
	encodeLog(pcommon.Resource, string, plog.LogRecord, pcommon.InstrumentationScope, string, esIndex, *bytes.Buffer) error
	encodeSpan(pcommon.Resource, string, ptrace.Span, pcommon.InstrumentationScope, string, esIndex, *bytes.Buffer) error
	encodeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent, scope pcommon.InstrumentationScope, scopeSchemaURL string, idx esIndex, buf *bytes.Buffer)
	hashDataPoint(dataPoint) uint32
	encodeDocument(objmodel.Document, *bytes.Buffer) error
	encodeMetrics(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, dataPoints []dataPoint, validationErrors *[]error, idx esIndex, buf *bytes.Buffer) (map[string]string, error)
}

// encodeModel tries to keep the event as close to the original open telemetry semantics as is.
// No fields will be mapped by default.
//
// Field deduplication and dedotting of attributes is supported by the encodeModel.
//
// See: https://github.com/open-telemetry/oteps/blob/master/text/logs/0097-log-data-model.md
type encodeModel struct {
	dedot bool
	mode  MappingMode
}

type dataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Attributes() pcommon.Map
	Value() (pcommon.Value, error)
	DynamicTemplate(pmetric.Metric) string
	DocCount() uint64
	HasMappingHint(elasticsearch.MappingHint) bool
	Metric() pmetric.Metric
}

const (
	traceIDField   = "traceID"
	spanIDField    = "spanID"
	attributeField = "attribute"
)

func (m *encodeModel) encodeLog(resource pcommon.Resource, resourceSchemaURL string, record plog.LogRecord, scope pcommon.InstrumentationScope, scopeSchemaURL string, idx esIndex, buf *bytes.Buffer) error {
	var document objmodel.Document
	switch m.mode {
	case MappingECS:
		document = m.encodeLogECSMode(resource, record, scope, idx)
	case MappingOTel:
		return serializeLog(resource, resourceSchemaURL, scope, scopeSchemaURL, record, idx, buf)
	case MappingBodyMap:
		return m.encodeLogBodyMapMode(record, buf)
	default:
		document = m.encodeLogDefaultMode(resource, record, scope, idx)
	}
	document.Dedup()

	return document.Serialize(buf, m.dedot)
}

func (m *encodeModel) encodeLogDefaultMode(resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope, idx esIndex) objmodel.Document {
	var document objmodel.Document

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
	m.encodeAttributes(&document, record.Attributes(), idx)
	document.AddAttributes("Resource", resource.Attributes())
	document.AddAttributes("Scope", scopeToAttributes(scope))

	return document
}

func (m *encodeModel) encodeLogBodyMapMode(record plog.LogRecord, buf *bytes.Buffer) error {
	body := record.Body()
	if body.Type() != pcommon.ValueTypeMap {
		return fmt.Errorf("%w: %q", ErrInvalidTypeForBodyMapMode, body.Type())
	}

	serializeMap(body.Map(), buf)
	return nil
}

func (m *encodeModel) encodeLogECSMode(resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope, idx esIndex) objmodel.Document {
	var document objmodel.Document

	// First, try to map resource-level attributes to ECS fields.
	encodeAttributesECSMode(&document, resource.Attributes(), resourceAttrsConversionMap, resourceAttrsToPreserve)

	// Then, try to map scope-level attributes to ECS fields.
	scopeAttrsConversionMap := map[string]string{
		// None at the moment
	}
	encodeAttributesECSMode(&document, scope.Attributes(), scopeAttrsConversionMap, resourceAttrsToPreserve)

	// Finally, try to map record-level attributes to ECS fields.
	recordAttrsConversionMap := map[string]string{
		"event.name":                         "event.action",
		semconv.AttributeExceptionMessage:    "error.message",
		semconv.AttributeExceptionStacktrace: "error.stacktrace",
		semconv.AttributeExceptionType:       "error.type",
		semconv.AttributeExceptionEscaped:    "event.error.exception.handled",
	}
	encodeAttributesECSMode(&document, record.Attributes(), recordAttrsConversionMap, resourceAttrsToPreserve)
	addDataStreamAttributes(&document, "", idx)

	// Handle special cases.
	encodeLogAgentNameECSMode(&document, resource)
	encodeLogAgentVersionECSMode(&document, resource)
	encodeLogHostOsTypeECSMode(&document, resource)
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

	return document
}

func (m *encodeModel) encodeDocument(document objmodel.Document, buf *bytes.Buffer) error {
	document.Dedup()

	err := document.Serialize(buf, m.dedot)
	if err != nil {
		return err
	}
	return nil
}

// upsertMetricDataPointValue upserts a datapoint value to documents which is already hashed by resource and index
func (m *encodeModel) hashDataPoint(dp dataPoint) uint32 {
	switch m.mode {
	case MappingOTel:
		return metricOTelHash(dp, dp.Metric().Unit())
	default:
		// Defaults to ECS for backward compatibility
		return metricECSHash(dp.Timestamp(), dp.Attributes())
	}
}

func (m *encodeModel) encodeDataPointsECSMode(resource pcommon.Resource, dataPoints []dataPoint, validationErrors *[]error, idx esIndex, buf *bytes.Buffer) (map[string]string, error) {
	dp0 := dataPoints[0]
	var document objmodel.Document
	encodeAttributesECSMode(&document, resource.Attributes(), resourceAttrsConversionMap, resourceAttrsToPreserve)
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
	err := m.encodeDocument(document, buf)

	return document.DynamicTemplates(), err
}

func addDataStreamAttributes(document *objmodel.Document, key string, idx esIndex) {
	if idx.isDataStream() {
		document.AddString(key+"data_stream.type", idx.Type)
		document.AddString(key+"data_stream.dataset", idx.Dataset)
		document.AddString(key+"data_stream.namespace", idx.Namespace)
	}
}

func (m *encodeModel) encodeMetrics(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, dataPoints []dataPoint, validationErrors *[]error, idx esIndex, buf *bytes.Buffer) (map[string]string, error) {
	switch m.mode {
	case MappingOTel:
		return serializeMetrics(resource, resourceSchemaURL, scope, scopeSchemaURL, dataPoints, validationErrors, idx, buf)
	default:
		return m.encodeDataPointsECSMode(resource, dataPoints, validationErrors, idx, buf)
	}
}

type summaryDataPoint struct {
	pmetric.SummaryDataPoint
	elasticsearch.MappingHintGetter
	metric pmetric.Metric
}

func newSummaryDataPoint(metric pmetric.Metric, dp pmetric.SummaryDataPoint) summaryDataPoint {
	return summaryDataPoint{SummaryDataPoint: dp, MappingHintGetter: elasticsearch.NewMappingHintGetter(dp.Attributes()), metric: metric}
}

func (dp summaryDataPoint) Value() (pcommon.Value, error) {
	// TODO: Add support for quantiles
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34561
	vm := pcommon.NewValueMap()
	m := vm.Map()
	m.PutDouble("sum", dp.Sum())
	m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
	return vm, nil
}

func (dp summaryDataPoint) DynamicTemplate(_ pmetric.Metric) string {
	return "summary"
}

func (dp summaryDataPoint) DocCount() uint64 {
	return dp.Count()
}

func (dp summaryDataPoint) Metric() pmetric.Metric {
	return dp.metric
}

type exponentialHistogramDataPoint struct {
	pmetric.ExponentialHistogramDataPoint
	elasticsearch.MappingHintGetter
	metric pmetric.Metric
}

func newExponentialHistogramDataPoint(metric pmetric.Metric, dp pmetric.ExponentialHistogramDataPoint) exponentialHistogramDataPoint {
	return exponentialHistogramDataPoint{ExponentialHistogramDataPoint: dp, MappingHintGetter: elasticsearch.NewMappingHintGetter(dp.Attributes()), metric: metric}
}

func (dp exponentialHistogramDataPoint) Value() (pcommon.Value, error) {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		vm := pcommon.NewValueMap()
		m := vm.Map()
		m.PutDouble("sum", dp.Sum())
		m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
		return vm, nil
	}

	counts, values := exphistogram.ToTDigest(dp.ExponentialHistogramDataPoint)

	vm := pcommon.NewValueMap()
	m := vm.Map()
	vmCounts := m.PutEmptySlice("counts")
	vmCounts.EnsureCapacity(len(counts))
	for _, c := range counts {
		vmCounts.AppendEmpty().SetInt(c)
	}
	vmValues := m.PutEmptySlice("values")
	vmValues.EnsureCapacity(len(values))
	for _, v := range values {
		vmValues.AppendEmpty().SetDouble(v)
	}

	return vm, nil
}

func (dp exponentialHistogramDataPoint) DynamicTemplate(_ pmetric.Metric) string {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		return "summary"
	}
	return "histogram"
}

func (dp exponentialHistogramDataPoint) DocCount() uint64 {
	return dp.Count()
}

func (dp exponentialHistogramDataPoint) Metric() pmetric.Metric {
	return dp.metric
}

type histogramDataPoint struct {
	pmetric.HistogramDataPoint
	elasticsearch.MappingHintGetter
	metric pmetric.Metric
}

func newHistogramDataPoint(metric pmetric.Metric, dp pmetric.HistogramDataPoint) histogramDataPoint {
	return histogramDataPoint{HistogramDataPoint: dp, MappingHintGetter: elasticsearch.NewMappingHintGetter(dp.Attributes()), metric: metric}
}

func (dp histogramDataPoint) Value() (pcommon.Value, error) {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		vm := pcommon.NewValueMap()
		m := vm.Map()
		m.PutDouble("sum", dp.Sum())
		m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
		return vm, nil
	}
	return histogramToValue(dp.HistogramDataPoint)
}

func (dp histogramDataPoint) DynamicTemplate(_ pmetric.Metric) string {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		return "summary"
	}
	return "histogram"
}

func (dp histogramDataPoint) DocCount() uint64 {
	return dp.HistogramDataPoint.Count()
}

func (dp histogramDataPoint) Metric() pmetric.Metric {
	return dp.metric
}

func histogramToValue(dp pmetric.HistogramDataPoint) (pcommon.Value, error) {
	// Histogram conversion function is from
	// https://github.com/elastic/apm-data/blob/3b28495c3cbdc0902983134276eb114231730249/input/otlp/metrics.go#L277
	bucketCounts := dp.BucketCounts()
	explicitBounds := dp.ExplicitBounds()
	if bucketCounts.Len() != explicitBounds.Len()+1 || explicitBounds.Len() == 0 {
		return pcommon.Value{}, errors.New("invalid histogram data point")
	}

	vm := pcommon.NewValueMap()
	m := vm.Map()
	counts := m.PutEmptySlice("counts")
	values := m.PutEmptySlice("values")

	values.EnsureCapacity(bucketCounts.Len())
	counts.EnsureCapacity(bucketCounts.Len())
	for i := 0; i < bucketCounts.Len(); i++ {
		count := bucketCounts.At(i)
		if count == 0 {
			continue
		}

		var value float64
		switch i {
		// (-infinity, explicit_bounds[i]]
		case 0:
			value = explicitBounds.At(i)
			if value > 0 {
				value /= 2
			}

		// (explicit_bounds[i], +infinity)
		case bucketCounts.Len() - 1:
			value = explicitBounds.At(i - 1)

		// [explicit_bounds[i-1], explicit_bounds[i])
		default:
			// Use the midpoint between the boundaries.
			value = explicitBounds.At(i-1) + (explicitBounds.At(i)-explicitBounds.At(i-1))/2.0
		}

		counts.AppendEmpty().SetInt(safeUint64ToInt64(count))
		values.AppendEmpty().SetDouble(value)
	}

	return vm, nil
}

type numberDataPoint struct {
	pmetric.NumberDataPoint
	elasticsearch.MappingHintGetter
	metric pmetric.Metric
}

func newNumberDataPoint(metric pmetric.Metric, dp pmetric.NumberDataPoint) numberDataPoint {
	return numberDataPoint{NumberDataPoint: dp, MappingHintGetter: elasticsearch.NewMappingHintGetter(dp.Attributes()), metric: metric}
}

func (dp numberDataPoint) Value() (pcommon.Value, error) {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		value := dp.DoubleValue()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return pcommon.Value{}, errInvalidNumberDataPoint
		}
		return pcommon.NewValueDouble(value), nil
	case pmetric.NumberDataPointValueTypeInt:
		return pcommon.NewValueInt(dp.IntValue()), nil
	}
	return pcommon.Value{}, errInvalidNumberDataPoint
}

func (dp numberDataPoint) DynamicTemplate(metric pmetric.Metric) string {
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		switch dp.NumberDataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			if metric.Sum().IsMonotonic() {
				return "counter_double"
			}
			return "gauge_double"
		case pmetric.NumberDataPointValueTypeInt:
			if metric.Sum().IsMonotonic() {
				return "counter_long"
			}
			return "gauge_long"
		default:
			return "" // NumberDataPointValueTypeEmpty should already be discarded in numberToValue
		}
	case pmetric.MetricTypeGauge:
		switch dp.NumberDataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			return "gauge_double"
		case pmetric.NumberDataPointValueTypeInt:
			return "gauge_long"
		default:
			return "" // NumberDataPointValueTypeEmpty should already be discarded in numberToValue
		}
	}
	return ""
}

func (dp numberDataPoint) DocCount() uint64 {
	return 1
}

func (dp numberDataPoint) Metric() pmetric.Metric {
	return dp.metric
}

var errInvalidNumberDataPoint = errors.New("invalid number data point")

func (m *encodeModel) encodeSpan(resource pcommon.Resource, resourceSchemaURL string, span ptrace.Span, scope pcommon.InstrumentationScope, scopeSchemaURL string, idx esIndex, buf *bytes.Buffer) error {
	var document objmodel.Document
	switch m.mode {
	case MappingOTel:
		return serializeSpan(resource, resourceSchemaURL, scope, scopeSchemaURL, span, idx, buf)
	default:
		document = m.encodeSpanDefaultMode(resource, span, scope, idx)
	}
	document.Dedup()
	err := document.Serialize(buf, m.dedot)
	return err
}

func (m *encodeModel) encodeSpanDefaultMode(resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope, idx esIndex) objmodel.Document {
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
	m.encodeAttributes(&document, span.Attributes(), idx)
	document.AddAttributes("Resource", resource.Attributes())
	m.encodeEvents(&document, span.Events())
	document.AddInt("Duration", durationAsMicroseconds(span.StartTimestamp().AsTime(), span.EndTimestamp().AsTime())) // unit is microseconds
	document.AddAttributes("Scope", scopeToAttributes(scope))
	return document
}

func (m *encodeModel) encodeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent, scope pcommon.InstrumentationScope, scopeSchemaURL string, idx esIndex, buf *bytes.Buffer) {
	if m.mode != MappingOTel {
		// Currently span events are stored separately only in OTel mapping mode.
		// In other modes, they are stored within the span document.
		return
	}
	serializeSpanEvent(resource, resourceSchemaURL, scope, scopeSchemaURL, span, spanEvent, idx, buf)
}

func (m *encodeModel) encodeAttributes(document *objmodel.Document, attributes pcommon.Map, idx esIndex) {
	key := "Attributes"
	if m.mode == MappingRaw {
		key = ""
	}
	document.AddAttributes(key, attributes)
	addDataStreamAttributes(document, key, idx)
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

func encodeAttributesECSMode(document *objmodel.Document, attrs pcommon.Map, conversionMap map[string]string, preserveMap map[string]bool) {
	if len(conversionMap) == 0 {
		// No conversions to be done; add all attributes at top level of
		// document.
		document.AddAttributes("", attrs)
		return
	}

	attrs.Range(func(k string, v pcommon.Value) bool {
		// If ECS key is found for current k in conversion map, use it.
		if ecsKey, exists := conversionMap[k]; exists {
			if ecsKey == "" {
				// Skip the conversion for this k.
				return true
			}

			document.AddAttribute(ecsKey, v)
			if preserve := preserveMap[k]; preserve {
				document.AddAttribute(k, v)
			}
			return true
		}

		// Otherwise, add key at top level with attribute name as-is.
		document.AddAttribute(k, v)
		return true
	})
}

func encodeLogAgentNameECSMode(document *objmodel.Document, resource pcommon.Resource) {
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

	// Set agent name in document.
	document.AddString("agent.name", agentName)
}

func encodeLogAgentVersionECSMode(document *objmodel.Document, resource pcommon.Resource) {
	attrs := resource.Attributes()

	if telemetryDistroVersion, exists := attrs.Get(semconv.AttributeTelemetryDistroVersion); exists {
		document.AddString("agent.version", telemetryDistroVersion.Str())
		return
	}

	if telemetrySdkVersion, exists := attrs.Get(semconv.AttributeTelemetrySDKVersion); exists {
		document.AddString("agent.version", telemetrySdkVersion.Str())
		return
	}
}

func encodeLogHostOsTypeECSMode(document *objmodel.Document, resource pcommon.Resource) {
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

// TODO use https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/internal/exp/metrics/identity
func metricECSHash(timestamp pcommon.Timestamp, attributes pcommon.Map) uint32 {
	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(timestamp))
	hasher.Write(timestampBuf)

	mapHashExcludeReservedAttrs(hasher, attributes)

	return hasher.Sum32()
}

func metricOTelHash(dp dataPoint, unit string) uint32 {
	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.StartTimestamp()))
	hasher.Write(timestampBuf)

	hasher.Write([]byte(unit))

	mapHashExcludeReservedAttrs(hasher, dp.Attributes(), elasticsearch.MappingHintsAttrKey)

	return hasher.Sum32()
}

// mapHashExcludeReservedAttrs is mapHash but ignoring some reserved attributes.
// e.g. index is already considered during routing and DS attributes do not need to be considered in hashing
func mapHashExcludeReservedAttrs(hasher hash.Hash, m pcommon.Map, extra ...string) {
	m.Range(func(k string, v pcommon.Value) bool {
		switch k {
		case dataStreamType, dataStreamDataset, dataStreamNamespace:
			return true
		}
		if slices.Contains(extra, k) {
			return true
		}
		hasher.Write([]byte(k))
		valueHash(hasher, v)

		return true
	})
}

func mapHash(hasher hash.Hash, m pcommon.Map) {
	m.Range(func(k string, v pcommon.Value) bool {
		hasher.Write([]byte(k))
		valueHash(hasher, v)

		return true
	})
}

func valueHash(h hash.Hash, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		h.Write([]byte{0})
	case pcommon.ValueTypeStr:
		h.Write([]byte(v.Str()))
	case pcommon.ValueTypeBool:
		if v.Bool() {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	case pcommon.ValueTypeDouble:
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(v.Double()))
		h.Write(buf)
	case pcommon.ValueTypeInt:
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, uint64(v.Int()))
		h.Write(buf)
	case pcommon.ValueTypeBytes:
		h.Write(v.Bytes().AsRaw())
	case pcommon.ValueTypeMap:
		mapHash(h, v.Map())
	case pcommon.ValueTypeSlice:
		sliceHash(h, v.Slice())
	}
}

func sliceHash(h hash.Hash, s pcommon.Slice) {
	for i := 0; i < s.Len(); i++ {
		valueHash(h, s.At(i))
	}
}

func safeUint64ToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v) //nolint:goset // overflow checked
}
