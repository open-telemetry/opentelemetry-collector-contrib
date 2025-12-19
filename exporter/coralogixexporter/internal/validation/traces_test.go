// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func getTraceID(s string) [16]byte {
	var id [16]byte
	copy(id[:], s)
	return id
}

func TestCollectInvalidDurationSpans(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "broken-service")
	rs.Resource().Attributes().PutStr("k8s.pod.name", "pod-1")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("scope")
	ss.Scope().SetVersion("1.0.0")

	// Invalid span 1
	firstSpan := ss.Spans().AppendEmpty()
	firstSpan.SetName("invalid-one")
	firstSpan.SetTraceID(getTraceID("trace-invalid-1"))
	firstSpan.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	firstSpan.SetStartTimestamp(200)
	firstSpan.SetEndTimestamp(100)

	// Invalid span 2
	secondSpan := ss.Spans().AppendEmpty()
	secondSpan.SetName("invalid-two")
	secondSpan.SetTraceID(getTraceID("trace-invalid-2"))
	secondSpan.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
	secondSpan.SetStartTimestamp(400)
	secondSpan.SetEndTimestamp(100)

	// Valid span
	validSpan := ss.Spans().AppendEmpty()
	validSpan.SetName("valid")
	validSpan.SetTraceID(getTraceID("trace-valid-----"))
	validSpan.SetSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9})
	validSpan.SetStartTimestamp(100)
	validSpan.SetEndTimestamp(200)

	samples := collectInvalidDurationSpans(traces)
	require.Len(t, samples, 2) // Both invalid spans collected
	sample := samples[0]
	assert.Equal(t, "invalid-one", sample.SpanDetails.SpanName)
	assert.Equal(t, "scope", sample.SpanDetails.InstrumentationScopeName)
	assert.Equal(t, "1.0.0", sample.SpanDetails.InstrumentationScopeVersion)
	assert.Equal(t, uint64(200), sample.StartTimeUnixNano)
	assert.Equal(t, uint64(100), sample.EndTimeUnixNano)
	assert.Equal(t, int64(-100), sample.DurationNano)
	assert.Equal(t, "broken-service", sample.SpanDetails.ResourceAttributes["service.name"])
	assert.Equal(t, "pod-1", sample.SpanDetails.ResourceAttributes["k8s.pod.name"])

	traceID := getTraceID("trace-invalid-1")
	expectedTraceID := hex.EncodeToString(traceID[:])
	assert.Equal(t, expectedTraceID, sample.SpanDetails.TraceID)
	expectedSpanID := hex.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	assert.Equal(t, expectedSpanID, sample.SpanDetails.SpanID)
}

func TestCollectInvalidDurationSpans_SkipsZeroTimestamps(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	// Span with zero start
	span1 := ss.Spans().AppendEmpty()
	span1.SetStartTimestamp(0)
	span1.SetEndTimestamp(100)

	// Span with zero end
	span2 := ss.Spans().AppendEmpty()
	span2.SetStartTimestamp(100)
	span2.SetEndTimestamp(0)

	samples := collectInvalidDurationSpans(traces)
	assert.Empty(t, samples)
}

func TestCollectInvalidTraceIDs(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()

	// Invalid trace ID (all zeros) - note: pdata creates empty IDs by default
	span1 := ss.Spans().AppendEmpty()
	span1.SetName("span-with-invalid-trace-id")
	// Don't set trace ID, leaving it empty (all zeros)
	span1.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	// Valid trace ID
	span2 := ss.Spans().AppendEmpty()
	span2.SetName("span-with-valid-trace-id")
	span2.SetTraceID(getTraceID("valid-trace-id--"))
	span2.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})

	samples := collectInvalidTraceIDs(traces)
	require.Len(t, samples, 1)
	sample := samples[0]
	assert.Equal(t, "span-with-invalid-trace-id", sample.SpanDetails.SpanName)
	assert.Empty(t, sample.SpanDetails.TraceID) // Empty trace IDs return empty string
	expectedSpanID := hex.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	assert.Equal(t, expectedSpanID, sample.SpanDetails.SpanID)
	assert.Equal(t, "test-service", sample.SpanDetails.ResourceAttributes["service.name"])
}

func TestCollectInvalidSpanIDs(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()

	// Invalid span ID (all zeros) - note: pdata creates empty IDs by default
	span1 := ss.Spans().AppendEmpty()
	span1.SetName("span-with-invalid-span-id")
	span1.SetTraceID(getTraceID("valid-trace-id--"))
	// Don't set span ID, leaving it empty (all zeros)

	// Valid span ID
	span2 := ss.Spans().AppendEmpty()
	span2.SetName("span-with-valid-span-id")
	span2.SetTraceID(getTraceID("valid-trace-id--"))
	span2.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	samples := collectInvalidSpanIDs(traces)
	require.Len(t, samples, 1)
	sample := samples[0]
	assert.Equal(t, "span-with-invalid-span-id", sample.SpanDetails.SpanName)
	assert.Empty(t, sample.SpanDetails.SpanID) // Empty span IDs return empty string
	assert.Equal(t, "test-service", sample.SpanDetails.ResourceAttributes["service.name"])
}

func TestCollectInvalidStartTimes(t *testing.T) {
	now := time.Unix(1700000000, 0) // Fixed time for testing
	nowUnixNano := uint64(now.UnixNano())

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test-scope")

	// Span too far in the future (> 1 hour)
	span1 := ss.Spans().AppendEmpty()
	span1.SetName("span-too-far-future")
	span1.SetTraceID(getTraceID("trace-1---------"))
	span1.SetSpanID([8]byte{1, 1, 1, 1, 1, 1, 1, 1})
	span1.SetStartTimestamp(pcommon.Timestamp(nowUnixNano + maxFutureNanos + 1000))

	// Span too far in the past (> 24 hours)
	span2 := ss.Spans().AppendEmpty()
	span2.SetName("span-too-far-past")
	span2.SetTraceID(getTraceID("trace-2---------"))
	span2.SetSpanID([8]byte{2, 2, 2, 2, 2, 2, 2, 2})
	span2.SetStartTimestamp(pcommon.Timestamp(nowUnixNano - maxPastNanos - 1000))

	// Valid span
	span3 := ss.Spans().AppendEmpty()
	span3.SetName("valid-span")
	span3.SetTraceID(getTraceID("trace-3---------"))
	span3.SetSpanID([8]byte{3, 3, 3, 3, 3, 3, 3, 3})
	span3.SetStartTimestamp(pcommon.Timestamp(nowUnixNano))

	samples := collectInvalidStartTimes(traces, now)
	require.Len(t, samples, 2)

	// Check first invalid span
	assert.Equal(t, "span-too-far-future", samples[0].SpanDetails.SpanName)
	assert.Equal(t, "test-scope", samples[0].SpanDetails.InstrumentationScopeName)

	// Check second invalid span
	assert.Equal(t, "span-too-far-past", samples[1].SpanDetails.SpanName)
}

func TestCollectMissingAttributes(t *testing.T) {
	traces := ptrace.NewTraces()

	// ResourceSpan 1: missing cx.application.name
	rs1 := traces.ResourceSpans().AppendEmpty()
	rs1.Resource().Attributes().PutStr("service.name", "service-1")
	rs1.Resource().Attributes().PutStr("cx.subsystem.name", "subsystem-1")
	ss1 := rs1.ScopeSpans().AppendEmpty()
	for i := range 3 {
		span := ss1.Spans().AppendEmpty()
		span.SetName("span-" + string(rune('a'+i)))
		span.SetTraceID(getTraceID("trace-1---------"))
		span.SetSpanID([8]byte{byte(i), 0, 0, 0, 0, 0, 0, 0})
	}

	// ResourceSpan 2: has cx.application.name
	rs2 := traces.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr("service.name", "service-2")
	rs2.Resource().Attributes().PutStr("cx.application.name", "app-2")
	rs2.Resource().Attributes().PutStr("cx.subsystem.name", "subsystem-2")
	ss2 := rs2.ScopeSpans().AppendEmpty()
	span := ss2.Spans().AppendEmpty()
	span.SetName("valid-span")
	span.SetTraceID(getTraceID("trace-2---------"))

	samples := collectMissingAttributes(traces, "cx.application.name")
	require.Len(t, samples, 1)
	sample := samples[0]
	assert.Equal(t, "cx.application.name", sample.MissingAttribute)
	assert.Equal(t, "service-1", sample.SpanDetails.ResourceAttributes["service.name"])
	assert.Equal(t, "span-a", sample.SpanDetails.SpanName)
}

func TestCollectInvalidDurationSpans_Limit(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	// Create 10 invalid spans
	for i := range 10 {
		span := ss.Spans().AppendEmpty()
		span.SetName("invalid-span")
		span.SetTraceID(getTraceID("trace----------"))
		span.SetSpanID([8]byte{byte(i), 0, 0, 0, 0, 0, 0, 0})
		span.SetStartTimestamp(200)
		span.SetEndTimestamp(100)
	}

	// Test that it collects up to InvalidSpanSampleLimit
	samples := collectInvalidDurationSpans(traces)
	assert.Len(t, samples, InvalidSpanSampleLimit)
}

func TestCollectInvalidStartTimes_ZeroTimestamp(t *testing.T) {
	now := time.Unix(1700000000, 0)
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	// Span with zero start time
	span := ss.Spans().AppendEmpty()
	span.SetName("span-with-zero-timestamp")
	span.SetStartTimestamp(0)

	samples := collectInvalidStartTimes(traces, now)
	assert.Empty(t, samples)
}

func TestSummarizeResourceAttributes_EmptyKeys(t *testing.T) {
	res := pcommon.NewResource()
	res.Attributes().PutStr("service.name", "test")

	// Empty keys slice
	summary := summarizeResourceAttributes(res, []string{})
	assert.Nil(t, summary)

	// Keys with empty string
	summary = summarizeResourceAttributes(res, []string{""})
	assert.Nil(t, summary)

	// Keys that don't exist
	summary = summarizeResourceAttributes(res, []string{"nonexistent.key"})
	assert.Nil(t, summary)
}

func TestBuildPartialSuccessLogFieldsForTraces_AllErrorTypes(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		setupTraces func() ptrace.Traces
		expectType  PartialSuccessErrorType
		expectCount int
	}{
		{
			name:     "InvalidTraceID",
			errorMsg: "Invalid trace identifier: 00000000000000000000000000000000. A valid trace identifier is a 16-byte array with at least one non-zero byte.",
			setupTraces: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				ss := rs.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				span.SetName("span-no-trace-id")
				// Don't set trace ID - will be all zeros
				return traces
			},
			expectType:  ErrorTypeInvalidTraceID,
			expectCount: 1,
		},
		{
			name:     "InvalidSpanID",
			errorMsg: "Invalid span identifier: 0000000000000000. A valid span identifier is an 8-byte array with at least one non-zero byte.",
			setupTraces: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				ss := rs.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				span.SetName("span-no-span-id")
				span.SetTraceID(getTraceID("valid-trace-id--"))
				// Don't set span ID - will be all zeros
				return traces
			},
			expectType:  ErrorTypeInvalidSpanID,
			expectCount: 1,
		},
		{
			name:     "InvalidStartTime",
			errorMsg: "Invalid span start time: 9999999999999999999. A span timestamp should be at most 24 hours back and 1 front.",
			setupTraces: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				ss := rs.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				span.SetName("span-far-future")
				span.SetTraceID(getTraceID("trace----------"))
				span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
				// Set start time far in the future
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()) + pcommon.Timestamp(maxFutureNanos) + 1000000)
				return traces
			},
			expectType:  ErrorTypeInvalidStartTime,
			expectCount: 1,
		},
		{
			name:     "NoAppName",
			errorMsg: "Application name is not available. A \"cx.application.name\" attribute must be specified for a resource.",
			setupTraces: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				// No cx.application.name attribute
				ss := rs.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				span.SetName("span-no-app")
				return traces
			},
			expectType:  ErrorTypeNoAppName,
			expectCount: 1,
		},
		{
			name:     "NoSubsystemName",
			errorMsg: "Subsystem name is not available. A \"cx.subsystem.name\" attribute must be specified for a resource.",
			setupTraces: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				// No cx.subsystem.name attribute
				ss := rs.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				span.SetName("span-no-subsystem")
				return traces
			},
			expectType:  ErrorTypeNoSubsystemName,
			expectCount: 1,
		},
		{
			name:     "SpanTooBig",
			errorMsg: "Span 0102030405060708 is too big",
			setupTraces: func() ptrace.Traces {
				traces := ptrace.NewTraces()
				rs := traces.ResourceSpans().AppendEmpty()
				ss := rs.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				span.SetName("large-span")
				span.SetTraceID(getTraceID("trace----------"))
				return traces
			},
			expectType:  ErrorTypeSpanTooBig,
			expectCount: 1,
		},
		{
			name:        "UnknownError",
			errorMsg:    "some unknown error occurred",
			setupTraces: ptrace.NewTraces,
			expectType:  "",
			expectCount: 0,
		},
		{
			name:        "EmptyMessage",
			errorMsg:    "",
			setupTraces: ptrace.NewTraces,
			expectType:  "",
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces := tt.setupTraces()
			fields := BuildPartialSuccessLogFieldsForTraces(tt.errorMsg, traces, "cx.application.name", "cx.subsystem.name")

			if tt.expectCount == 0 {
				assert.Empty(t, fields)
				return
			}

			require.Len(t, fields, 2)
			// Check that we have partial_success_type and samples fields
			var foundType, foundSamples bool
			for _, field := range fields {
				if field.Key == "partial_success_type" {
					foundType = true
					assert.Equal(t, string(tt.expectType), field.String)
				}
				if field.Key == "samples" {
					foundSamples = true
				}
			}
			assert.True(t, foundType, "Should have partial_success_type field")
			assert.True(t, foundSamples, "Should have samples field")
		})
	}
}

func TestCollectMissingAttributes_ResourceWithNoSpans(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	// Resource has no spans
	rs.ScopeSpans().AppendEmpty()

	samples := collectMissingAttributes(traces, "cx.application.name")
	require.Len(t, samples, 1)
	// baseDetail should be empty since there are no spans
	assert.Empty(t, samples[0].SpanDetails.SpanName)
}

func TestCollectSpansByID(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test-scope")

	// Create span with specific ID
	targetSpan := ss.Spans().AppendEmpty()
	targetSpan.SetName("target-span")
	targetSpan.SetTraceID(getTraceID("trace----------"))
	targetSpan.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	// Add attributes to make it larger
	for i := range 50 {
		targetSpan.Attributes().PutStr("attr-"+string(rune('a'+i%26)), "value-data")
	}

	// Create another span
	otherSpan := ss.Spans().AppendEmpty()
	otherSpan.SetName("other-span")
	otherSpan.SetTraceID(getTraceID("trace----------"))
	otherSpan.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})

	// Search for the target span by ID
	spanIDHex := "0102030405060708"
	samples := collectSpansByID(traces, spanIDHex)

	require.Len(t, samples, 1)
	assert.Equal(t, "target-span", samples[0].SpanDetails.SpanName)
	assert.Equal(t, spanIDHex, samples[0].SpanDetails.SpanID)
	assert.Positive(t, samples[0].SerializedSizeBytes)
}

func TestBuildPartialSuccessLogFieldsForTraces_SpanTooBigWithID(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("large-span")
	span.SetTraceID(getTraceID("trace----------"))
	span.SetSpanID([8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22})

	// Backend error message with span ID: "Span aabbccddeeff1122 is too big"
	errorMsg := "Span aabbccddeeff1122 is too big"
	fields := BuildPartialSuccessLogFieldsForTraces(errorMsg, traces, "", "")

	require.Len(t, fields, 2)
	assert.Equal(t, "partial_success_type", fields[0].Key)
	assert.Equal(t, string(ErrorTypeSpanTooBig), fields[0].String)
	assert.Equal(t, "samples", fields[1].Key)
}
