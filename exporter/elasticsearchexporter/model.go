// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash"
	"hash/fnv"
	"math"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/encoder"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/encoder/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/encoder/log"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/encoder/otel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/exphistogram"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type mappingModel interface {
	log.Encoder

	encodeSpan(pcommon.Resource, string, ptrace.Span, pcommon.InstrumentationScope, string) ([]byte, error)
	encodeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent, scope pcommon.InstrumentationScope, scopeSchemaURL string) *objmodel.Document
	upsertMetricDataPointValue(map[uint32]objmodel.Document, pcommon.Resource, string, pcommon.InstrumentationScope, string, pmetric.Metric, dataPoint) error
	encodeDocument(objmodel.Document) ([]byte, error)
}

// encodeModel tries to keep the event as close to the original open telemetry semantics as is.
// No fields will be mapped by default.
//
// Field deduplication and dedotting of attributes is supported by the encodeModel.
//
// See: https://github.com/open-telemetry/oteps/blob/master/text/logs/0097-log-data-model.md
type encodeModel struct {
	dedot bool
	mode  mapping.Mode

	log.Encoder
}

func newEncodeModel(dedot bool, mode mapping.Mode) *encodeModel {
	return &encodeModel{
		dedot: dedot,
		mode:  mode,

		Encoder: log.New(dedot, mode),
	}
}

type dataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Attributes() pcommon.Map
	Value() (pcommon.Value, error)
	DynamicTemplate(pmetric.Metric) string
	DocCount() uint64
	HasMappingHint(mappingHint) bool
}

const (
	traceIDField   = "traceID"
	spanIDField    = "spanID"
	attributeField = "attribute"
)

func (m *encodeModel) encodeDocument(document objmodel.Document) ([]byte, error) {
	// For OTel mode, prefix conflicts are not a problem as otel-data has subobjects: false
	document.Dedup(m.mode != mapping.ModeOTel)

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot, m.mode == mapping.ModeOTel)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// upsertMetricDataPointValue upserts a datapoint value to documents which is already hashed by resource and index
func (m *encodeModel) upsertMetricDataPointValue(documents map[uint32]objmodel.Document, resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, metric pmetric.Metric, dp dataPoint) error {
	switch m.mode {
	case mapping.ModeOTel:
		return m.upsertMetricDataPointValueOTelMode(documents, resource, resourceSchemaURL, scope, scopeSchemaURL, metric, dp)
	case mapping.ModeECS:
		return m.upsertMetricDataPointValueECSMode(documents, resource, resourceSchemaURL, scope, scopeSchemaURL, metric, dp)
	default:
		// Defaults to ECS for backward compatibility
		return m.upsertMetricDataPointValueECSMode(documents, resource, resourceSchemaURL, scope, scopeSchemaURL, metric, dp)
	}
}

func (m *encodeModel) upsertMetricDataPointValueECSMode(documents map[uint32]objmodel.Document, resource pcommon.Resource, _ string, _ pcommon.InstrumentationScope, _ string, metric pmetric.Metric, dp dataPoint) error {
	value, err := dp.Value()
	if err != nil {
		return err
	}

	hash := metricECSHash(dp.Timestamp(), dp.Attributes())
	var (
		document objmodel.Document
		ok       bool
	)
	if document, ok = documents[hash]; !ok {
		ecs.EncodeAttributes(&document, resource.Attributes(), nil)
		document.AddTimestamp("@timestamp", dp.Timestamp())
		document.AddAttributes("", dp.Attributes())
	}

	document.AddAttribute(metric.Name(), value)

	documents[hash] = document
	return nil
}

func (m *encodeModel) upsertMetricDataPointValueOTelMode(documents map[uint32]objmodel.Document, resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, metric pmetric.Metric, dp dataPoint) error {
	value, err := dp.Value()
	if err != nil {
		return err
	}

	// documents is per-resource. Therefore, there is no need to hash resource attributes
	hash := metricOTelHash(dp, scope.Attributes(), metric.Unit())
	var (
		document objmodel.Document
		ok       bool
	)
	if document, ok = documents[hash]; !ok {
		document.AddTimestamp("@timestamp", dp.Timestamp())
		if dp.StartTimestamp() != 0 {
			document.AddTimestamp("start_timestamp", dp.StartTimestamp())
		}
		document.AddString("unit", metric.Unit())

		otel.EncodeAttributes(&document, dp.Attributes())
		otel.EncodeResource(&document, resource, resourceSchemaURL)
		otel.EncodeScope(&document, scope, scopeSchemaURL)
	}

	if dp.HasMappingHint(hintDocCount) {
		docCount := dp.DocCount()
		document.AddUInt("_doc_count", docCount)
	}

	switch value.Type() {
	case pcommon.ValueTypeMap:
		m := pcommon.NewMap()
		value.Map().CopyTo(m)
		document.Add("metrics."+metric.Name(), objmodel.UnflattenableObjectValue(m))
	default:
		document.Add("metrics."+metric.Name(), objmodel.ValueFromAttribute(value))
	}
	// TODO: support quantiles
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34561

	// DynamicTemplate returns the name of dynamic template that applies to the metric and data point,
	// so that the field is indexed into Elasticsearch with the correct mapping. The name should correspond to a
	// dynamic template that is defined in ES mapping, e.g.
	// https://github.com/elastic/elasticsearch/blob/8.15/x-pack/plugin/core/template-resources/src/main/resources/metrics%40mappings.json
	document.AddDynamicTemplate("metrics."+metric.Name(), dp.DynamicTemplate(metric))
	documents[hash] = document
	return nil
}

type summaryDataPoint struct {
	pmetric.SummaryDataPoint
	mappingHintGetter
}

func newSummaryDataPoint(dp pmetric.SummaryDataPoint) summaryDataPoint {
	return summaryDataPoint{SummaryDataPoint: dp, mappingHintGetter: newMappingHintGetter(dp.Attributes())}
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

type exponentialHistogramDataPoint struct {
	pmetric.ExponentialHistogramDataPoint
	mappingHintGetter
}

func newExponentialHistogramDataPoint(dp pmetric.ExponentialHistogramDataPoint) exponentialHistogramDataPoint {
	return exponentialHistogramDataPoint{ExponentialHistogramDataPoint: dp, mappingHintGetter: newMappingHintGetter(dp.Attributes())}
}

func (dp exponentialHistogramDataPoint) Value() (pcommon.Value, error) {
	if dp.HasMappingHint(hintAggregateMetricDouble) {
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
	if dp.HasMappingHint(hintAggregateMetricDouble) {
		return "summary"
	}
	return "histogram"
}

func (dp exponentialHistogramDataPoint) DocCount() uint64 {
	return dp.Count()
}

type histogramDataPoint struct {
	pmetric.HistogramDataPoint
	mappingHintGetter
}

func newHistogramDataPoint(dp pmetric.HistogramDataPoint) histogramDataPoint {
	return histogramDataPoint{HistogramDataPoint: dp, mappingHintGetter: newMappingHintGetter(dp.Attributes())}
}

func (dp histogramDataPoint) Value() (pcommon.Value, error) {
	if dp.HasMappingHint(hintAggregateMetricDouble) {
		vm := pcommon.NewValueMap()
		m := vm.Map()
		m.PutDouble("sum", dp.Sum())
		m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
		return vm, nil
	}
	return histogramToValue(dp.HistogramDataPoint)
}

func (dp histogramDataPoint) DynamicTemplate(_ pmetric.Metric) string {
	if dp.HasMappingHint(hintAggregateMetricDouble) {
		return "summary"
	}
	return "histogram"
}

func (dp histogramDataPoint) DocCount() uint64 {
	return dp.HistogramDataPoint.Count()
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
	mappingHintGetter
}

func newNumberDataPoint(dp pmetric.NumberDataPoint) numberDataPoint {
	return numberDataPoint{NumberDataPoint: dp, mappingHintGetter: newMappingHintGetter(dp.Attributes())}
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

var errInvalidNumberDataPoint = errors.New("invalid number data point")

func (m *encodeModel) encodeSpan(resource pcommon.Resource, resourceSchemaURL string, span ptrace.Span, scope pcommon.InstrumentationScope, scopeSchemaURL string) ([]byte, error) {
	var document objmodel.Document
	switch m.mode {
	case mapping.ModeOTel:
		document = m.encodeSpanOTelMode(resource, resourceSchemaURL, span, scope, scopeSchemaURL)
	default:
		document = m.encodeSpanDefaultMode(resource, span, scope)
	}
	// For OTel mode, prefix conflicts are not a problem as otel-data has subobjects: false
	document.Dedup(m.mode != mapping.ModeOTel)
	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot, m.mode == mapping.ModeOTel)
	return buf.Bytes(), err
}

func (m *encodeModel) encodeSpanOTelMode(resource pcommon.Resource, resourceSchemaURL string, span ptrace.Span, scope pcommon.InstrumentationScope, scopeSchemaURL string) objmodel.Document {
	var document objmodel.Document
	document.AddTimestamp("@timestamp", span.StartTimestamp())
	document.AddTraceID("trace_id", span.TraceID())
	document.AddSpanID("span_id", span.SpanID())
	document.AddString("trace_state", span.TraceState().AsRaw())
	document.AddSpanID("parent_span_id", span.ParentSpanID())
	document.AddString("name", span.Name())
	document.AddString("kind", span.Kind().String())
	document.AddUInt("duration", uint64(span.EndTimestamp()-span.StartTimestamp()))

	otel.EncodeAttributes(&document, span.Attributes())

	document.AddInt("dropped_attributes_count", int64(span.DroppedAttributesCount()))
	document.AddInt("dropped_events_count", int64(span.DroppedEventsCount()))

	links := pcommon.NewValueSlice()
	linkSlice := links.SetEmptySlice()
	spanLinks := span.Links()
	for i := 0; i < spanLinks.Len(); i++ {
		linkMap := linkSlice.AppendEmpty().SetEmptyMap()
		spanLink := spanLinks.At(i)
		linkMap.PutStr("trace_id", spanLink.TraceID().String())
		linkMap.PutStr("span_id", spanLink.SpanID().String())
		linkMap.PutStr("trace_state", spanLink.TraceState().AsRaw())
		mAttr := linkMap.PutEmptyMap("attributes")
		spanLink.Attributes().CopyTo(mAttr)
		linkMap.PutInt("dropped_attributes_count", int64(spanLink.DroppedAttributesCount()))
	}
	document.AddAttribute("links", links)

	document.AddInt("dropped_links_count", int64(span.DroppedLinksCount()))
	document.AddString("status.message", span.Status().Message())
	document.AddString("status.code", span.Status().Code().String())

	otel.EncodeResource(&document, resource, resourceSchemaURL)
	otel.EncodeScope(&document, scope, scopeSchemaURL)

	return document
}

func (m *encodeModel) encodeSpanDefaultMode(resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope) objmodel.Document {
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
	encoder.EncodeAttributes(m.mode, &document, span.Attributes())
	document.AddAttributes("Resource", resource.Attributes())
	m.encodeEvents(&document, span.Events())
	document.AddInt("Duration", durationAsMicroseconds(span.StartTimestamp().AsTime(), span.EndTimestamp().AsTime())) // unit is microseconds
	document.AddAttributes("Scope", encoder.ScopeToAttributes(scope))
	return document
}

func (m *encodeModel) encodeSpanEvent(resource pcommon.Resource, resourceSchemaURL string, span ptrace.Span, spanEvent ptrace.SpanEvent, scope pcommon.InstrumentationScope, scopeSchemaURL string) *objmodel.Document {
	if m.mode != mapping.ModeOTel {
		// Currently span events are stored separately only in OTel mapping mode.
		// In other modes, they are stored within the span document.
		return nil
	}
	var document objmodel.Document
	document.AddTimestamp("@timestamp", spanEvent.Timestamp())
	document.AddString("attributes.event.name", spanEvent.Name())
	document.AddSpanID("span_id", span.SpanID())
	document.AddTraceID("trace_id", span.TraceID())
	document.AddInt("dropped_attributes_count", int64(spanEvent.DroppedAttributesCount()))

	otel.EncodeAttributes(&document, spanEvent.Attributes())
	otel.EncodeResource(&document, resource, resourceSchemaURL)
	otel.EncodeScope(&document, scope, scopeSchemaURL)

	return &document
}

func (m *encodeModel) encodeEvents(document *objmodel.Document, events ptrace.SpanEventSlice) {
	key := "Events"
	if m.mode == mapping.ModeRaw {
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

// TODO use https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/internal/exp/metrics/identity
func metricECSHash(timestamp pcommon.Timestamp, attributes pcommon.Map) uint32 {
	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(timestamp))
	hasher.Write(timestampBuf)

	mapHashExcludeReservedAttrs(hasher, attributes)

	return hasher.Sum32()
}

func metricOTelHash(dp dataPoint, scopeAttrs pcommon.Map, unit string) uint32 {
	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.StartTimestamp()))
	hasher.Write(timestampBuf)

	hasher.Write([]byte(unit))

	mapHashExcludeReservedAttrs(hasher, scopeAttrs)
	mapHashExcludeReservedAttrs(hasher, dp.Attributes(), mappingHintsAttrKey)

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
		binary.LittleEndian.PutUint64(buf, uint64(v.Int())) // nolint:gosec // Overflow assumed. We prefer having high integers over zero.
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
	} else {
		return int64(v) // nolint:goset // overflow checked
	}
}
