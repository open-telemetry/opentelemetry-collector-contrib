// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package semconvtest validates Collector component telemetry against
// OpenTelemetry semantic conventions using Weaver's live-check.
//
// This file reimplements Weaver's live-check report types in Go so the
// JSON report returned by Weaver can be parsed and inspected.
package semconvtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"

import (
	"encoding/json"
	"fmt"
)

// LiveCheckReport represents the top-level output from Weaver's live-check command.
type LiveCheckReport struct {
	Samples    []Sample            `json:"samples"`
	Statistics LiveCheckStatistics `json:"statistics"`
}

// Sample represents a live-check sample entity.
type Sample struct {
	Attribute *SampleAttribute `json:"attribute,omitempty"`
	Span      *SampleSpan      `json:"span,omitempty"`
	SpanEvent *SampleSpanEvent `json:"span_event,omitempty"`
	SpanLink  *SampleSpanLink  `json:"span_link,omitempty"`
	Resource  *SampleResource  `json:"resource,omitempty"`
	Metric    *SampleMetric    `json:"metric,omitempty"`
	Log       *SampleLog       `json:"log,omitempty"`
}

// LiveCheckResult contains the advice findings for a sample entity.
type LiveCheckResult struct {
	AllAdvice          []PolicyFinding `json:"all_advice"`
	HighestAdviceLevel *FindingLevel   `json:"highest_advice_level,omitempty"`
}

// PolicyFinding represents a single finding from the live-check.
type PolicyFinding struct {
	ID         string          `json:"id"`
	Context    json.RawMessage `json:"context"`
	Message    string          `json:"message"`
	Level      FindingLevel    `json:"level"`
	SignalType *string         `json:"signal_type,omitempty"`
	SignalName *string         `json:"signal_name,omitempty"`
}

// FindingLevel represents the severity level of a finding.
type FindingLevel string

const (
	FindingLevelInformation FindingLevel = "information"
	FindingLevelImprovement FindingLevel = "improvement"
	FindingLevelViolation   FindingLevel = "violation"
)

// LiveCheckStatistics contains statistics about the live-check run.
type LiveCheckStatistics struct {
	TotalEntities             int                  `json:"total_entities,omitempty"`
	TotalEntitiesByType       map[string]int       `json:"total_entities_by_type,omitempty"`
	TotalAdvisories           int                  `json:"total_advisories,omitempty"`
	AdviceLevelCounts         map[FindingLevel]int `json:"advice_level_counts,omitempty"`
	HighestAdviceLevelCounts  map[FindingLevel]int `json:"highest_advice_level_counts,omitempty"`
	NoAdviceCount             int                  `json:"no_advice_count,omitempty"`
	AdviceTypeCounts          map[string]int       `json:"advice_type_counts,omitempty"`
	AdviceMessageCounts       map[string]int       `json:"advice_message_counts,omitempty"`
	SeenRegistryAttributes    map[string]int       `json:"seen_registry_attributes,omitempty"`
	SeenNonRegistryAttributes map[string]int       `json:"seen_non_registry_attributes,omitempty"`
	SeenRegistryMetrics       map[string]int       `json:"seen_registry_metrics,omitempty"`
	SeenNonRegistryMetrics    map[string]int       `json:"seen_non_registry_metrics,omitempty"`
	SeenRegistryEvents        map[string]int       `json:"seen_registry_events,omitempty"`
	SeenNonRegistryEvents     map[string]int       `json:"seen_non_registry_events,omitempty"`
	RegistryCoverage          float32              `json:"registry_coverage,omitempty"`
}

// SampleAttribute represents a sample telemetry attribute.
type SampleAttribute struct {
	Name            string           `json:"name"`
	Value           json.RawMessage  `json:"value,omitempty"`
	Type            *string          `json:"type,omitempty"`
	LiveCheckResult *LiveCheckResult `json:"live_check_result,omitempty"`
}

// SampleResource represents a sample resource.
type SampleResource struct {
	Attributes      []SampleAttribute `json:"attributes,omitempty"`
	LiveCheckResult *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// SampleLog represents a sample telemetry log (event).
type SampleLog struct {
	EventName       string            `json:"event_name"`
	SeverityNumber  *int32            `json:"severity_number,omitempty"`
	SeverityText    *string           `json:"severity_text,omitempty"`
	Body            *string           `json:"body,omitempty"`
	Attributes      []SampleAttribute `json:"attributes,omitempty"`
	TraceID         *string           `json:"trace_id,omitempty"`
	SpanID          *string           `json:"span_id,omitempty"`
	LiveCheckResult *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// SampleSpan represents a sample telemetry span.
type SampleSpan struct {
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	Status          *SpanStatus       `json:"status,omitempty"`
	Attributes      []SampleAttribute `json:"attributes,omitempty"`
	SpanEvents      []SampleSpanEvent `json:"span_events,omitempty"`
	SpanLinks       []SampleSpanLink  `json:"span_links,omitempty"`
	LiveCheckResult *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// SpanStatus represents the status of a span.
type SpanStatus struct {
	Code    StatusCode `json:"code"`
	Message string     `json:"message"`
}

// StatusCode represents the status code of a span.
type StatusCode string

const (
	StatusCodeUnset StatusCode = "unset"
	StatusCodeOK    StatusCode = "ok"
	StatusCodeError StatusCode = "error"
)

// SampleSpanEvent represents a span event.
type SampleSpanEvent struct {
	Name            string            `json:"name"`
	Attributes      []SampleAttribute `json:"attributes,omitempty"`
	LiveCheckResult *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// SampleSpanLink represents a span link.
type SampleSpanLink struct {
	Attributes      []SampleAttribute `json:"attributes,omitempty"`
	LiveCheckResult *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// SampleMetric represents a sample telemetry metric.
type SampleMetric struct {
	Name            string           `json:"name"`
	Instrument      string           `json:"instrument"`
	Unit            string           `json:"unit"`
	DataPoints      *DataPoints      `json:"data_points,omitempty"`
	LiveCheckResult *LiveCheckResult `json:"live_check_result,omitempty"`
}

// DataPoints contains metric data points.
// Weaver uses an untagged enum, so we need custom unmarshaling.
type DataPoints struct {
	Number               []SampleNumberDataPoint               `json:"-"`
	Histogram            []SampleHistogramDataPoint            `json:"-"`
	ExponentialHistogram []SampleExponentialHistogramDataPoint `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling for DataPoints.
// Weaver serializes this as an untagged enum, so we try each variant.
func (dp *DataPoints) UnmarshalJSON(data []byte) error {
	var numberPoints []SampleNumberDataPoint
	if err := json.Unmarshal(data, &numberPoints); err == nil && len(numberPoints) > 0 {
		dp.Number = numberPoints
		return nil
	}

	var histogramPoints []SampleHistogramDataPoint
	if err := json.Unmarshal(data, &histogramPoints); err == nil && len(histogramPoints) > 0 {
		dp.Histogram = histogramPoints
		return nil
	}

	var expHistogramPoints []SampleExponentialHistogramDataPoint
	if err := json.Unmarshal(data, &expHistogramPoints); err == nil && len(expHistogramPoints) > 0 {
		dp.ExponentialHistogram = expHistogramPoints
		return nil
	}

	// No variant matched. Return nil rather than an error so that
	// unrecognized data point types don't prevent parsing the rest
	// of the report.
	return nil
}

// SampleNumberDataPoint represents a number data point (gauge, counter, etc.).
type SampleNumberDataPoint struct {
	Attributes      []SampleAttribute `json:"attributes,omitempty"`
	Value           json.RawMessage   `json:"value,omitempty"`
	Flags           uint32            `json:"flags,omitempty"`
	Exemplars       []SampleExemplar  `json:"exemplars,omitempty"`
	LiveCheckResult *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// SampleHistogramDataPoint represents a histogram data point.
type SampleHistogramDataPoint struct {
	Attributes      []SampleAttribute `json:"attributes,omitempty"`
	Count           uint64            `json:"count,omitempty"`
	Sum             *float64          `json:"sum,omitempty"`
	BucketCounts    []uint64          `json:"bucket_counts,omitempty"`
	ExplicitBounds  []float64         `json:"explicit_bounds,omitempty"`
	Min             *float64          `json:"min,omitempty"`
	Max             *float64          `json:"max,omitempty"`
	Flags           uint32            `json:"flags,omitempty"`
	Exemplars       []SampleExemplar  `json:"exemplars,omitempty"`
	LiveCheckResult *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// SampleExponentialHistogramDataPoint represents an exponential histogram data point.
type SampleExponentialHistogramDataPoint struct {
	Attributes      []SampleAttribute                  `json:"attributes,omitempty"`
	Count           uint64                             `json:"count,omitempty"`
	Sum             *float64                           `json:"sum,omitempty"`
	Scale           int32                              `json:"scale,omitempty"`
	ZeroCount       uint64                             `json:"zero_count,omitempty"`
	Positive        *SampleExponentialHistogramBuckets `json:"positive,omitempty"`
	Negative        *SampleExponentialHistogramBuckets `json:"negative,omitempty"`
	Flags           uint32                             `json:"flags,omitempty"`
	Min             *float64                           `json:"min,omitempty"`
	Max             *float64                           `json:"max,omitempty"`
	ZeroThreshold   float64                            `json:"zero_threshold,omitempty"`
	Exemplars       []SampleExemplar                   `json:"exemplars,omitempty"`
	LiveCheckResult *LiveCheckResult                   `json:"live_check_result,omitempty"`
}

// SampleExponentialHistogramBuckets represents buckets in an exponential histogram.
type SampleExponentialHistogramBuckets struct {
	Offset       int32    `json:"offset,omitempty"`
	BucketCounts []uint64 `json:"bucket_counts,omitempty"`
}

// SampleExemplar represents an exemplar measurement.
type SampleExemplar struct {
	FilteredAttributes []SampleAttribute `json:"filtered_attributes,omitempty"`
	Value              json.RawMessage   `json:"value,omitempty"`
	Timestamp          string            `json:"timestamp,omitempty"`
	SpanID             string            `json:"span_id,omitempty"`
	TraceID            string            `json:"trace_id,omitempty"`
	LiveCheckResult    *LiveCheckResult  `json:"live_check_result,omitempty"`
}

// ParseLiveCheckReport parses a JSON byte slice into a LiveCheckReport.
func ParseLiveCheckReport(data []byte) (*LiveCheckReport, error) {
	var report LiveCheckReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse live check report: %w", err)
	}
	return &report, nil
}

// HasViolations returns true if the report contains any violation-level findings.
func (r *LiveCheckReport) HasViolations() bool {
	count, ok := r.Statistics.HighestAdviceLevelCounts[FindingLevelViolation]
	return ok && count > 0
}

// GetViolations returns all findings with violation level from the report.
func (r *LiveCheckReport) GetViolations() []PolicyFinding {
	var violations []PolicyFinding
	for _, sample := range r.Samples {
		violations = append(violations, sample.collectViolations()...)
	}
	return violations
}

func (s *Sample) collectViolations() []PolicyFinding {
	var violations []PolicyFinding

	collectFromResult := func(result *LiveCheckResult) {
		if result == nil {
			return
		}
		for _, finding := range result.AllAdvice {
			if finding.Level == FindingLevelViolation {
				violations = append(violations, finding)
			}
		}
	}

	collectFromAttributes := func(attrs []SampleAttribute) {
		for _, attr := range attrs {
			collectFromResult(attr.LiveCheckResult)
		}
	}

	if s.Attribute != nil {
		collectFromResult(s.Attribute.LiveCheckResult)
	}
	if s.Span != nil {
		collectFromResult(s.Span.LiveCheckResult)
		collectFromAttributes(s.Span.Attributes)
		for _, event := range s.Span.SpanEvents {
			collectFromResult(event.LiveCheckResult)
			collectFromAttributes(event.Attributes)
		}
		for _, link := range s.Span.SpanLinks {
			collectFromResult(link.LiveCheckResult)
			collectFromAttributes(link.Attributes)
		}
	}
	if s.SpanEvent != nil {
		collectFromResult(s.SpanEvent.LiveCheckResult)
		collectFromAttributes(s.SpanEvent.Attributes)
	}
	if s.SpanLink != nil {
		collectFromResult(s.SpanLink.LiveCheckResult)
		collectFromAttributes(s.SpanLink.Attributes)
	}
	if s.Resource != nil {
		collectFromResult(s.Resource.LiveCheckResult)
		collectFromAttributes(s.Resource.Attributes)
	}
	if s.Metric != nil {
		collectFromResult(s.Metric.LiveCheckResult)
		if s.Metric.DataPoints != nil {
			for i := range s.Metric.DataPoints.Number {
				dp := &s.Metric.DataPoints.Number[i]
				collectFromResult(dp.LiveCheckResult)
				collectFromAttributes(dp.Attributes)
				for j := range dp.Exemplars {
					collectFromResult(dp.Exemplars[j].LiveCheckResult)
					collectFromAttributes(dp.Exemplars[j].FilteredAttributes)
				}
			}
			for i := range s.Metric.DataPoints.Histogram {
				dp := &s.Metric.DataPoints.Histogram[i]
				collectFromResult(dp.LiveCheckResult)
				collectFromAttributes(dp.Attributes)
				for j := range dp.Exemplars {
					collectFromResult(dp.Exemplars[j].LiveCheckResult)
					collectFromAttributes(dp.Exemplars[j].FilteredAttributes)
				}
			}
			for i := range s.Metric.DataPoints.ExponentialHistogram {
				dp := &s.Metric.DataPoints.ExponentialHistogram[i]
				collectFromResult(dp.LiveCheckResult)
				collectFromAttributes(dp.Attributes)
				for j := range dp.Exemplars {
					collectFromResult(dp.Exemplars[j].LiveCheckResult)
					collectFromAttributes(dp.Exemplars[j].FilteredAttributes)
				}
			}
		}
	}
	if s.Log != nil {
		collectFromResult(s.Log.LiveCheckResult)
		collectFromAttributes(s.Log.Attributes)
	}

	return violations
}
