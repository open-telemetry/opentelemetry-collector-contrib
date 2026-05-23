// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package validation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter/internal/validation"

import (
	"regexp"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const (
	// InvalidSpanSampleLimit is the maximum number of invalid span samples to collect
	// This is just to not pollute the log output a lot
	InvalidSpanSampleLimit = 5

	// Backend validation constants matching traces-gateway validation
	maxFutureNanos = uint64(1 * time.Hour / time.Nanosecond)
	maxPastNanos   = uint64(24 * time.Hour / time.Nanosecond)
)

// Regex patterns to match backend error messages
var (
	// "Invalid trace identifier: {hex}. A valid trace identifier is a 16-byte array with at least one non-zero byte."
	invalidTraceIDPattern = regexp.MustCompile(`Invalid trace identifier`)

	// "Invalid span identifier: {hex}. A valid span identifier is an 8-byte array with at least one non-zero byte."
	invalidSpanIDPattern = regexp.MustCompile(`Invalid span identifier`)

	// "Invalid span start time: {timestamp}. A span timestamp should be at most 24 hours back and 1 front."
	invalidStartTimePattern = regexp.MustCompile(`Invalid span start time`)

	// "Invalid span duration: start time {start_time_unix_nano} is greater than end time {end_time_unix_nano}."
	invalidDurationPattern = regexp.MustCompile(`Invalid span duration`)

	// "Application name is not available. A "cx.application.name" attribute must be specified for a resource."
	missingAppNamePattern = regexp.MustCompile(`Application name is not available`)

	// "Subsystem name is not available. A "cx.subsystem.name" attribute must be specified for a resource."
	missingSubsystemNamePattern = regexp.MustCompile(`Subsystem name is not available`)

	// "Span {span_id} is too big" - extracts the span ID from error message
	spanTooBigPattern = regexp.MustCompile(`Span ([0-9a-f]+) is too big`)
)

// Resource attribute keys to include in invalid span details for debugging
var resourceAttributeKeys = []string{
	"service.name",
	"service.namespace",
	"service.instance.id",
	"service.version",
	"service.group",
	"k8s.namespace.name",
	"k8s.pod.name",
	"k8s.deployment.name",
	"k8s.statefulset.name",
	"k8s.daemonset.name",
	"aws.ecs.cluster.name",
	"cx.application.name",
	"cx.subsystem.name",
}

// PartialSuccessErrorType represents types of partial success errors from the backend
type PartialSuccessErrorType string

// Partial success error types that can be detected in error messages
const (
	ErrorTypeInvalidDuration  PartialSuccessErrorType = "invalid_duration"
	ErrorTypeInvalidTraceID   PartialSuccessErrorType = "invalid_trace_id"
	ErrorTypeInvalidSpanID    PartialSuccessErrorType = "invalid_span_id"
	ErrorTypeInvalidStartTime PartialSuccessErrorType = "invalid_start_time"
	ErrorTypeNoAppName        PartialSuccessErrorType = "missing_application_name"
	ErrorTypeNoSubsystemName  PartialSuccessErrorType = "missing_subsystem_name"
	ErrorTypeSpanTooBig       PartialSuccessErrorType = "span_too_big"
)

// BaseSpanDetail contains common fields for all span validation details
// NOTE: JSON tags are required for proper serialization by zap logger
type BaseSpanDetail struct {
	TraceID                     string            `json:"trace_id"`
	SpanID                      string            `json:"span_id"`
	SpanName                    string            `json:"span_name"`
	ResourceAttributes          map[string]string `json:"resource_attributes,omitempty"`
	InstrumentationScopeName    string            `json:"instrumentation_scope_name,omitempty"`
	InstrumentationScopeVersion string            `json:"instrumentation_scope_version,omitempty"`
}

// InvalidSpanDetail contains debugging information for spans with invalid durations
type InvalidSpanDetail struct {
	SpanDetails       BaseSpanDetail `json:"span_details"`
	StartTimeUnixNano uint64         `json:"start_time_unix_nano"`
	EndTimeUnixNano   uint64         `json:"end_time_unix_nano"`
	DurationNano      int64          `json:"duration_nano,omitempty"`
}

// InvalidTraceIDDetail contains debugging information for invalid trace IDs
type InvalidTraceIDDetail struct {
	SpanDetails BaseSpanDetail `json:"span_details"`
}

// InvalidSpanIDDetail contains debugging information for invalid span IDs
type InvalidSpanIDDetail struct {
	SpanDetails BaseSpanDetail `json:"span_details"`
}

// InvalidStartTimeDetail contains debugging information for invalid start times
type InvalidStartTimeDetail struct {
	SpanDetails       BaseSpanDetail `json:"span_details"`
	StartTimeUnixNano uint64         `json:"start_time_unix_nano"`
}

// MissingAttributeDetail contains debugging information for missing required attributes
type MissingAttributeDetail struct {
	SpanDetails      BaseSpanDetail `json:"span_details"`
	MissingAttribute string         `json:"missing_attribute"`
}

// TooBigSpanDetail contains debugging information for spans that exceed size limits
type TooBigSpanDetail struct {
	SpanDetails         BaseSpanDetail `json:"span_details"`
	SerializedSizeBytes int            `json:"serialized_size_bytes"`
}

// BuildPartialSuccessLogFieldsForTraces creates log fields for partial success responses
// This is the main entry point for exporters to get validation details as log fields
// It returns ready-to-use zap fields that can be passed directly to the logger
func BuildPartialSuccessLogFieldsForTraces(errorMessage string, td ptrace.Traces, appNameAttr, subsystemNameAttr string) []zap.Field {
	fields := []zap.Field{}

	if errorMessage == "" {
		return fields
	}

	var samples any
	var errorType PartialSuccessErrorType

	switch {
	case spanTooBigPattern.MatchString(errorMessage):
		errorType = ErrorTypeSpanTooBig
		if matches := spanTooBigPattern.FindStringSubmatch(errorMessage); len(matches) > 1 {
			spanID := matches[1]
			samples = collectSpansByID(td, spanID)
		}
	case invalidDurationPattern.MatchString(errorMessage):
		errorType = ErrorTypeInvalidDuration
		samples = collectInvalidDurationSpans(td)
	case invalidTraceIDPattern.MatchString(errorMessage):
		errorType = ErrorTypeInvalidTraceID
		samples = collectInvalidTraceIDs(td)
	case invalidSpanIDPattern.MatchString(errorMessage):
		errorType = ErrorTypeInvalidSpanID
		samples = collectInvalidSpanIDs(td)
	case invalidStartTimePattern.MatchString(errorMessage):
		errorType = ErrorTypeInvalidStartTime
		samples = collectInvalidStartTimes(td, time.Now())
	// Missing app or subsysten name should not happen because we are adding them in the
	// exporter but checking just in case there is any bug
	case missingAppNamePattern.MatchString(errorMessage):
		errorType = ErrorTypeNoAppName
		if appNameAttr != "" {
			samples = collectMissingAttributes(td, appNameAttr)
		}
	case missingSubsystemNamePattern.MatchString(errorMessage):
		errorType = ErrorTypeNoSubsystemName
		if subsystemNameAttr != "" {
			samples = collectMissingAttributes(td, subsystemNameAttr)
		}
	default:
		return fields
	}

	if samples != nil {
		return []zap.Field{
			zap.String("partial_success_type", string(errorType)),
			zap.Any("samples", samples),
		}
	}

	return fields
}

// collectInvalidDurationSpans scans traces for spans with start time > end time
func collectInvalidDurationSpans(td ptrace.Traces) []InvalidSpanDetail {
	return collectInvalidSpans(td, func(span ptrace.Span, scope pcommon.InstrumentationScope, resourceAttrs map[string]string) (InvalidSpanDetail, bool) {
		// Skip spans with zero timestamps - backend treats these as incomplete/in-progress
		if span.StartTimestamp() == 0 || span.EndTimestamp() == 0 {
			return InvalidSpanDetail{}, false
		}
		if span.StartTimestamp() <= span.EndTimestamp() {
			return InvalidSpanDetail{}, false
		}

		base := createBaseSpanDetail(span, scope, resourceAttrs)
		return InvalidSpanDetail{
			SpanDetails:       base,
			StartTimeUnixNano: uint64(span.StartTimestamp()),
			EndTimeUnixNano:   uint64(span.EndTimestamp()),
			DurationNano:      int64(span.EndTimestamp()) - int64(span.StartTimestamp()),
		}, true
	})
}

// collectInvalidTraceIDs scans traces for invalid trace IDs (all zeros)
func collectInvalidTraceIDs(td ptrace.Traces) []InvalidTraceIDDetail {
	return collectInvalidSpans(td, func(span ptrace.Span, scope pcommon.InstrumentationScope, resourceAttrs map[string]string) (InvalidTraceIDDetail, bool) {
		if !span.TraceID().IsEmpty() {
			return InvalidTraceIDDetail{}, false
		}
		base := createBaseSpanDetail(span, scope, resourceAttrs)
		return InvalidTraceIDDetail{
			SpanDetails: base,
		}, true
	})
}

// collectInvalidSpanIDs scans traces for invalid span IDs (all zeros)
func collectInvalidSpanIDs(td ptrace.Traces) []InvalidSpanIDDetail {
	return collectInvalidSpans(td, func(span ptrace.Span, scope pcommon.InstrumentationScope, resourceAttrs map[string]string) (InvalidSpanIDDetail, bool) {
		if !span.SpanID().IsEmpty() {
			return InvalidSpanIDDetail{}, false
		}
		base := createBaseSpanDetail(span, scope, resourceAttrs)
		return InvalidSpanIDDetail{
			SpanDetails: base,
		}, true
	})
}

// collectInvalidStartTimes scans traces for invalid start times (too far in past/future)
func collectInvalidStartTimes(td ptrace.Traces, now time.Time) []InvalidStartTimeDetail {
	nowUnixNano := uint64(now.UnixNano())
	return collectInvalidSpans(td, func(span ptrace.Span, scope pcommon.InstrumentationScope, resourceAttrs map[string]string) (InvalidStartTimeDetail, bool) {
		startTime := uint64(span.StartTimestamp())
		if startTime == 0 {
			return InvalidStartTimeDetail{}, false
		}

		if startTime <= nowUnixNano+maxFutureNanos && startTime >= nowUnixNano-maxPastNanos {
			return InvalidStartTimeDetail{}, false
		}

		base := createBaseSpanDetail(span, scope, resourceAttrs)
		return InvalidStartTimeDetail{
			SpanDetails:       base,
			StartTimeUnixNano: startTime,
		}, true
	})
}

// collectMissingAttributes scans traces for missing required attributes
func collectMissingAttributes(td ptrace.Traces, attributeKey string) []MissingAttributeDetail {
	samples := make([]MissingAttributeDetail, 0, InvalidSpanSampleLimit)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		attrs := rs.Resource().Attributes()

		if _, ok := attrs.Get(attributeKey); !ok {
			resourceSummary := summarizeResourceAttributes(rs.Resource(), resourceAttributeKeys)

			var baseDetail BaseSpanDetail
			scopeSpans := rs.ScopeSpans()
			for j := 0; j < scopeSpans.Len(); j++ {
				scopeSpan := scopeSpans.At(j)
				scope := scopeSpan.Scope()
				spans := scopeSpan.Spans()

				if spans.Len() > 0 {
					baseDetail = createBaseSpanDetail(spans.At(0), scope, resourceSummary)
					break
				}
			}

			if len(samples) < InvalidSpanSampleLimit {
				samples = append(samples, MissingAttributeDetail{
					SpanDetails:      baseDetail,
					MissingAttribute: attributeKey,
				})
			}
		}
	}
	return samples
}

// collectSpansByID finds specific spans by their span ID (from backend error message)
// Backend error format: "Span {span_id} is too big"
func collectSpansByID(td ptrace.Traces, spanIDHex string) []TooBigSpanDetail {
	marshaler := &ptrace.ProtoMarshaler{}
	samples := make([]TooBigSpanDetail, 0, InvalidSpanSampleLimit)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resourceSummary := summarizeResourceAttributes(rs.Resource(), resourceAttributeKeys)

		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			scope := scopeSpan.Scope()
			spans := scopeSpan.Spans()

			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				if span.SpanID().String() == spanIDHex {
					size := marshaler.SpanSize(span)
					base := createBaseSpanDetail(span, scope, resourceSummary)
					detail := TooBigSpanDetail{
						SpanDetails:         base,
						SerializedSizeBytes: size,
					}
					samples = append(samples, detail)

					if len(samples) >= InvalidSpanSampleLimit {
						return samples
					}
				}
			}
		}
	}

	return samples
}

// collectInvalidSpans is a generic helper that iterates through spans and collects invalid ones
type spanCollectorFunc[T any] func(span ptrace.Span, scope pcommon.InstrumentationScope, resourceAttrs map[string]string) (T, bool)

func collectInvalidSpans[T any](td ptrace.Traces, collector spanCollectorFunc[T]) []T {
	samples := make([]T, 0, InvalidSpanSampleLimit)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resourceSummary := summarizeResourceAttributes(rs.Resource(), resourceAttributeKeys)
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			scope := scopeSpan.Scope()
			spans := scopeSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				detail, isInvalid := collector(span, scope, resourceSummary)
				if !isInvalid {
					continue
				}

				if len(samples) < InvalidSpanSampleLimit {
					samples = append(samples, detail)
				}
			}
		}
	}
	return samples
}

// createBaseSpanDetail creates the base span detail from a span
func createBaseSpanDetail(span ptrace.Span, scope pcommon.InstrumentationScope, resourceAttrs map[string]string) BaseSpanDetail {
	return BaseSpanDetail{
		TraceID:                     span.TraceID().String(),
		SpanID:                      span.SpanID().String(),
		SpanName:                    span.Name(),
		ResourceAttributes:          resourceAttrs,
		InstrumentationScopeName:    scope.Name(),
		InstrumentationScopeVersion: scope.Version(),
	}
}

// summarizeResourceAttributes extracts specific resource attributes for debugging
func summarizeResourceAttributes(res pcommon.Resource, keys []string) map[string]string {
	if len(keys) == 0 {
		return nil
	}
	attrs := res.Attributes()
	summary := make(map[string]string, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		attr, ok := attrs.Get(key)
		if !ok {
			continue
		}
		summary[key] = attr.AsString()
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}
