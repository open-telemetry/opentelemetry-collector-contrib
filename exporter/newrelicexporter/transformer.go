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

package newrelicexporter

import (
	"errors"
	"fmt"
	"strings"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/newrelic/newrelic-telemetry-sdk-go/cumulative"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	unitAttrKey               = "unit"
	descriptionAttrKey        = "description"
	collectorNameKey          = "collector.name"
	collectorVersionKey       = "collector.version"
	instrumentationNameKey    = "instrumentation.name"
	instrumentationVersionKey = "instrumentation.version"
	statusCodeKey             = "otel.status_code"
	statusDescriptionKey      = "otel.status_description"
	spanKindKey               = "span.kind"
	serviceNameKey            = "service.name"
)

// TODO (MrAlias): unify this with the traceTransformer when the metric data
// export moves to using pdata and away from the OC proto.
type metricTransformer struct {
	DeltaCalculator *cumulative.DeltaCalculator
	ServiceName     string
	Resource        *resourcepb.Resource
}

type traceTransformer struct {
	ResourceAttributes map[string]interface{}
}

func newTraceTransformer(resource pdata.Resource, lib pdata.InstrumentationLibrary) *traceTransformer {
	t := &traceTransformer{
		ResourceAttributes: tracetranslator.AttributeMapToMap(
			resource.Attributes(),
		),
	}

	if n := lib.Name(); n != "" {
		t.ResourceAttributes[instrumentationNameKey] = n
		if v := lib.Version(); v != "" {
			t.ResourceAttributes[instrumentationVersionKey] = v
		}
	}
	return t
}

var (
	errInvalidSpanID  = errors.New("SpanID is invalid")
	errInvalidTraceID = errors.New("TraceID is invalid")
)

func (t *traceTransformer) Span(span pdata.Span) (telemetry.Span, error) {
	startTime := pdata.UnixNanoToTime(span.StartTime())
	sp := telemetry.Span{
		// HexString validates the IDs, it will be an empty string if invalid.
		ID:         span.SpanID().HexString(),
		TraceID:    span.TraceID().HexString(),
		ParentID:   span.ParentSpanID().HexString(),
		Name:       span.Name(),
		Timestamp:  startTime,
		Duration:   pdata.UnixNanoToTime(span.EndTime()).Sub(startTime),
		Attributes: t.SpanAttributes(span),
		Events:     t.SpanEvents(span),
	}

	if sp.ID == "" {
		return sp, errInvalidSpanID
	}
	if sp.TraceID == "" {
		return sp, errInvalidTraceID
	}

	return sp, nil
}

func (t *traceTransformer) SpanAttributes(span pdata.Span) map[string]interface{} {

	length := 2 + len(t.ResourceAttributes) + span.Attributes().Len()

	var hasStatusCode, hasStatusDesc bool
	s := span.Status()
	if s.Code() != pdata.StatusCodeUnset {
		hasStatusCode = true
		length++
		if s.Message() != "" {
			hasStatusDesc = true
			length++
		}
	}

	validSpanKind := span.Kind() != pdata.SpanKindUNSPECIFIED
	if validSpanKind {
		length++
	}

	attrs := make(map[string]interface{}, length)

	if hasStatusCode {
		code := strings.TrimPrefix(span.Status().Code().String(), "STATUS_CODE_")
		attrs[statusCodeKey] = code
	}
	if hasStatusDesc {
		attrs[statusDescriptionKey] = span.Status().Message()
	}

	// Add span kind if it is set
	if validSpanKind {
		kind := strings.TrimPrefix(span.Kind().String(), "SPAN_KIND_")
		attrs[spanKindKey] = strings.ToLower(kind)
	}

	for k, v := range t.ResourceAttributes {
		attrs[k] = v
	}

	for k, v := range tracetranslator.AttributeMapToMap(span.Attributes()) {
		attrs[k] = v
	}

	// Default attributes to tell New Relic about this collector.
	// (overrides any existing)
	attrs[collectorNameKey] = name
	attrs[collectorVersionKey] = version

	return attrs
}

// SpanEvents transforms the recorded events of span into New Relic tracing events.
func (t *traceTransformer) SpanEvents(span pdata.Span) []telemetry.Event {
	length := span.Events().Len()
	if length == 0 {
		return nil
	}

	events := make([]telemetry.Event, length)

	for i := 0; i < length; i++ {
		event := span.Events().At(i)
		events[i] = telemetry.Event{
			EventType:  event.Name(),
			Timestamp:  pdata.UnixNanoToTime(event.Timestamp()),
			Attributes: tracetranslator.AttributeMapToMap(event.Attributes()),
		}
	}
	return events
}

func (t *metricTransformer) Timestamp(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return time.Unix(ts.Seconds, int64(ts.Nanos))
}

func (t *metricTransformer) Metric(metric *metricspb.Metric) ([]telemetry.Metric, error) {
	if metric == nil || metric.MetricDescriptor == nil {
		return nil, errors.New("empty metric")
	}

	var errs []error
	md := metric.MetricDescriptor
	baseAttrs := t.MetricAttributes(metric)

	var metrics []telemetry.Metric
	for _, ts := range metric.Timeseries {
		startTime := t.Timestamp(ts.StartTimestamp)

		attr, err := t.MergeAttributes(baseAttrs, md.LabelKeys, ts.LabelValues)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, point := range ts.Points {
			switch md.Type {
			case
				metricspb.MetricDescriptor_GAUGE_INT64,
				metricspb.MetricDescriptor_GAUGE_DOUBLE:
				metrics = append(metrics, t.Gauge(md.Name, attr, point))
			case
				metricspb.MetricDescriptor_GAUGE_DISTRIBUTION,
				metricspb.MetricDescriptor_SUMMARY:
				metrics = append(metrics, t.DeltaSummary(md.Name, attr, startTime, point))
			case
				metricspb.MetricDescriptor_CUMULATIVE_INT64,
				metricspb.MetricDescriptor_CUMULATIVE_DOUBLE:
				metrics = append(metrics, t.CumulativeCount(md.Name, attr, startTime, point))
			case
				metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION:
				metrics = append(metrics, t.CumulativeSummary(md.Name, attr, startTime, point))
			default:
				errs = append(errs, fmt.Errorf("unsupported metric type: %s", md.Type.String()))
			}
		}
	}
	return metrics, componenterror.CombineErrors(errs)
}

func (t *metricTransformer) MetricAttributes(metric *metricspb.Metric) map[string]interface{} {
	length := 3

	if t.Resource != nil {
		length += len(t.Resource.Labels)
	}

	if metric.MetricDescriptor.Unit != "" {
		length++
	}

	if metric.MetricDescriptor.Description != "" {
		length++
	}

	attrs := make(map[string]interface{}, length)

	if t.Resource != nil {
		for k, v := range t.Resource.Labels {
			attrs[k] = v
		}
	}

	if metric.MetricDescriptor.Unit != "" {
		attrs[unitAttrKey] = metric.MetricDescriptor.Unit
	}

	if metric.MetricDescriptor.Description != "" {
		attrs[descriptionAttrKey] = metric.MetricDescriptor.Description
	}

	attrs[collectorNameKey] = name
	attrs[collectorVersionKey] = version
	if t.ServiceName != "" {
		attrs[serviceNameKey] = t.ServiceName
	}

	return attrs
}

var errIncompatibleLabels = errors.New("label keys and values do not match")

func (t *metricTransformer) MergeAttributes(base map[string]interface{}, lk []*metricspb.LabelKey, lv []*metricspb.LabelValue) (map[string]interface{}, error) {
	if len(lk) != len(lv) {
		return nil, fmt.Errorf("%w: number of label keys (%d) different than label values (%d)", errIncompatibleLabels, len(lk), len(lv))
	}

	attrs := make(map[string]interface{}, len(base)+len(lk))

	for k, v := range base {
		attrs[k] = v
	}
	for i, k := range lk {
		v := lv[i]
		if v.Value != "" || v.HasValue {
			attrs[k.Key] = v.Value
		}
	}
	return attrs, nil
}

func (t *metricTransformer) Gauge(name string, attrs map[string]interface{}, point *metricspb.Point) telemetry.Metric {
	now := time.Now()
	if point.Timestamp != nil {
		now = t.Timestamp(point.Timestamp)
	}

	m := telemetry.Gauge{
		Name:       name,
		Attributes: attrs,
		Timestamp:  now,
	}

	switch t := point.Value.(type) {
	case *metricspb.Point_Int64Value:
		m.Value = float64(t.Int64Value)
	case *metricspb.Point_DoubleValue:
		m.Value = t.DoubleValue
	}

	return m
}

func (t *metricTransformer) DeltaSummary(name string, attrs map[string]interface{}, start time.Time, point *metricspb.Point) telemetry.Metric {
	now := time.Now()
	if point.Timestamp != nil {
		now = t.Timestamp(point.Timestamp)
	}

	m := telemetry.Summary{
		Name:       name,
		Attributes: attrs,
		Timestamp:  start,
		Interval:   now.Sub(start),
	}

	switch t := point.Value.(type) {
	case *metricspb.Point_DistributionValue:
		m.Count = float64(t.DistributionValue.Count)
		m.Sum = t.DistributionValue.Sum
	case *metricspb.Point_SummaryValue:
		m.Count = float64(t.SummaryValue.Count.Value)
		m.Sum = t.SummaryValue.Sum.Value
	}

	return m
}

func (t *metricTransformer) CumulativeCount(name string, attrs map[string]interface{}, start time.Time, point *metricspb.Point) telemetry.Metric {
	var value float64
	switch t := point.Value.(type) {
	case *metricspb.Point_Int64Value:
		value = float64(t.Int64Value)
	case *metricspb.Point_DoubleValue:
		value = t.DoubleValue
	}

	now := time.Now()
	if point.Timestamp != nil {
		now = t.Timestamp(point.Timestamp)
	}

	count, valid := t.DeltaCalculator.CountMetric(name, attrs, value, now)

	// This is the first measurement or a reset happened.
	if !valid {
		count = telemetry.Count{
			Name:       name,
			Attributes: attrs,
			Value:      value,
			Timestamp:  start,
			Interval:   now.Sub(start),
		}
	}

	return count
}

func (t *metricTransformer) CumulativeSummary(name string, attrs map[string]interface{}, start time.Time, point *metricspb.Point) telemetry.Metric {
	var sum, count float64
	switch t := point.Value.(type) {
	case *metricspb.Point_DistributionValue:
		sum = t.DistributionValue.Sum
		count = float64(t.DistributionValue.Count)
	}

	now := time.Now()
	if point.Timestamp != nil {
		now = t.Timestamp(point.Timestamp)
	}

	cCount, cValid := t.DeltaCalculator.CountMetric(name+".count", attrs, count, now)
	sCount, sValid := t.DeltaCalculator.CountMetric(name+".sum", attrs, sum, now)

	summary := telemetry.Summary{
		Name:       name,
		Attributes: attrs,
		Count:      cCount.Value,
		Sum:        sCount.Value,
		Timestamp:  sCount.Timestamp,
		Interval:   sCount.Interval,
	}

	// This is the first measurement or a reset happened.
	if !cValid || !sValid {
		summary.Count = count
		summary.Sum = sum
		summary.Timestamp = start
		summary.Interval = now.Sub(start)
	}

	return summary
}
