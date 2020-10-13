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
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/newrelic/newrelic-telemetry-sdk-go/cumulative"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/component/componenterror"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	unitAttrKey         = "unit"
	descriptionAttrKey  = "description"
	collectorNameKey    = "collector.name"
	collectorVersionKey = "collector.version"
	serviceNameKey      = "service.name"
)

type transformer struct {
	DeltaCalculator *cumulative.DeltaCalculator
	ServiceName     string
	Resource        *resourcepb.Resource
}

var (
	emptySpan   telemetry.Span
	emptySpanID trace.SpanID
)

func (t *transformer) Span(span *tracepb.Span) (telemetry.Span, error) {
	if span == nil {
		return emptySpan, errors.New("empty span")
	}

	startTime := t.Timestamp(span.StartTime)

	var (
		traceID          trace.TraceID
		spanID, parentID trace.SpanID
	)

	copy(traceID[:], span.GetTraceId())
	copy(spanID[:], span.GetSpanId())
	copy(parentID[:], span.GetParentSpanId())
	sp := telemetry.Span{
		ID:          spanID.String(),
		TraceID:     traceID.String(),
		Name:        t.TruncatableString(span.GetName()),
		Timestamp:   startTime,
		Duration:    t.Timestamp(span.EndTime).Sub(startTime),
		ServiceName: t.ServiceName,
		Attributes:  t.SpanAttributes(span),
	}

	if parentID != emptySpanID {
		sp.ParentID = parentID.String()
	}

	return sp, nil
}

func (t *transformer) TruncatableString(ts *tracepb.TruncatableString) string {
	if ts == nil {
		return ""
	}
	return ts.Value
}

func (t *transformer) SpanAttributes(span *tracepb.Span) map[string]interface{} {

	length := 2

	isErr := span.Status != nil && span.Status.Code != 0
	if isErr {
		length++
	}

	if t.Resource != nil {
		length += len(t.Resource.Labels)
	}

	if span.Attributes != nil {
		length += len(span.Attributes.AttributeMap)
	}

	attrs := make(map[string]interface{}, length)

	// Any existing error attribute will override this.
	if isErr {
		attrs["error"] = true
	}

	// Add span kind if it is set
	if span.Kind != tracepb.Span_SPAN_KIND_UNSPECIFIED {
		attrs["span.kind"] = strings.ToLower(span.Kind.String())
	}

	if t.Resource != nil {
		for k, v := range t.Resource.Labels {
			attrs[k] = v
		}
	}

	if span.Attributes != nil {
		for key, attr := range span.Attributes.AttributeMap {
			if attr == nil || attr.Value == nil {
				continue
			}
			// Default to skipping if unknown type.
			switch v := attr.Value.(type) {
			case *tracepb.AttributeValue_BoolValue:
				attrs[key] = v.BoolValue
			case *tracepb.AttributeValue_IntValue:
				attrs[key] = v.IntValue
			case *tracepb.AttributeValue_DoubleValue:
				attrs[key] = v.DoubleValue
			case *tracepb.AttributeValue_StringValue:
				attrs[key] = t.TruncatableString(v.StringValue)
			}
		}
	}

	// Default attributes to tell New Relic about this collector.
	// (overrides any existing)
	attrs[collectorNameKey] = name
	attrs[collectorVersionKey] = version

	return attrs
}

func (t *transformer) Timestamp(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return time.Unix(ts.Seconds, int64(ts.Nanos))
}

func (t *transformer) Metric(metric *metricspb.Metric) ([]telemetry.Metric, error) {
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

func (t *transformer) MetricAttributes(metric *metricspb.Metric) map[string]interface{} {
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

func (t *transformer) MergeAttributes(base map[string]interface{}, lk []*metricspb.LabelKey, lv []*metricspb.LabelValue) (map[string]interface{}, error) {
	if len(lk) != len(lv) {
		return nil, fmt.Errorf("number of label keys (%d) different than label values (%d)", len(lk), len(lv))
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

func (t *transformer) Gauge(name string, attrs map[string]interface{}, point *metricspb.Point) telemetry.Metric {
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

func (t *transformer) DeltaSummary(name string, attrs map[string]interface{}, start time.Time, point *metricspb.Point) telemetry.Metric {
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

func (t *transformer) CumulativeCount(name string, attrs map[string]interface{}, start time.Time, point *metricspb.Point) telemetry.Metric {
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

func (t *transformer) CumulativeSummary(name string, attrs map[string]interface{}, start time.Time, point *metricspb.Point) telemetry.Metric {
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
