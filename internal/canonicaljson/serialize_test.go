// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package canonicaljson

import (
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/probityai/agent-evidence-vectors/jcsvectors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSerializeLogRecord_BasicFields(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567890, 123456789)))
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(1234567891, 987654321)))
	lr.Body().SetStr("test message")

	opts := SerializeLogRecordOptions{
		ExcludePrefixes: []string{"audit.integrity."},
	}

	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	// Parse back to verify structure
	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	assert.Equal(t, "1234567890123456789", data["timestamp"])
	assert.Equal(t, "1234567891987654321", data["observed_timestamp"])
	assert.Equal(t, map[string]any{"stringValue": "test message"}, data["body"])
}

func TestSerializeLogRecord_EventName(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.SetEventName("user.login")
	lr.Body().SetStr("User logged in")

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	assert.Equal(t, "user.login", data["event_name"])
}

func TestSerializeLogRecord_TraceContext(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("test")

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	lr.SetTraceID(traceID)
	lr.SetSpanID(spanID)

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", data["trace_id"])
	assert.Equal(t, "0102030405060708", data["span_id"])
}

func TestSerializeLogRecord_Attributes(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("test")
	lr.Attributes().PutStr("http.method", "GET")
	lr.Attributes().PutInt("http.status_code", 200)
	lr.Attributes().PutBool("success", true)

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	attrs := data["attributes"].(map[string]any)
	assert.Equal(t, map[string]any{"stringValue": "GET"}, attrs["http.method"])
	assert.Equal(t, map[string]any{"intValue": "200"}, attrs["http.status_code"])
	assert.Equal(t, map[string]any{"boolValue": true}, attrs["success"])
}

func TestSerializeLogRecord_ExcludePrefixes(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("test")
	lr.Attributes().PutStr("normal.attr", "value1")
	lr.Attributes().PutStr("audit.integrity.value", "signature")
	lr.Attributes().PutStr("audit.integrity.algorithm", "RS256")
	lr.Attributes().PutStr("audit.verification.status", "verified")
	lr.Attributes().PutStr("other.attr", "value2")

	opts := SerializeLogRecordOptions{
		ExcludePrefixes: []string{"audit.integrity.", "audit.verification."},
	}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	attrs := data["attributes"].(map[string]any)
	assert.Contains(t, attrs, "normal.attr")
	assert.Contains(t, attrs, "other.attr")
	assert.NotContains(t, attrs, "audit.integrity.value")
	assert.NotContains(t, attrs, "audit.integrity.algorithm")
	assert.NotContains(t, attrs, "audit.verification.status")
}

func TestSerializeLogRecord_EmptyBody(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Attributes().PutStr("key", "value")

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	assert.NotContains(t, data, "body")
	assert.Contains(t, data, "attributes")
}

func TestSerializeLogRecord_NoAttributes(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("test message")

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	assert.Contains(t, data, "body")
	assert.NotContains(t, data, "attributes")
}

func TestSerializeLogRecord_ComplexBody(t *testing.T) {
	lr := plog.NewLogRecord()

	bodyMap := lr.Body().SetEmptyMap()
	bodyMap.PutStr("message", "error occurred")
	bodyMap.PutInt("code", 500)
	details := bodyMap.PutEmptyMap("details")
	details.PutStr("component", "database")

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	body := data["body"].(map[string]any)
	assert.Equal(t, map[string]any{"stringValue": "error occurred"}, body["message"])
	assert.Equal(t, map[string]any{"intValue": "500"}, body["code"])

	detailsData := body["details"].(map[string]any)
	assert.Equal(t, map[string]any{"stringValue": "database"}, detailsData["component"])
}

func TestSerializeLogRecord_Defaults(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("test")
	lr.SetTimestamp(pcommon.Timestamp(1234567890000000000))

	// Create nested structure close to default limit (128) to verify defaults are actually applied
	attrs := lr.Attributes()
	current := attrs.PutEmptyMap("nested")
	for i := 0; i < 100; i++ { // Well under default MaxDepth of 128
		current = current.PutEmptyMap("level")
	}
	current.PutStr("deep", "value")

	// Test with empty options - should use defaults (MaxDepth: 128, MaxInputBytes: 2 MiB)
	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Verify it's actually using defaults
	assert.Less(t, len(result), DefaultMaxInputBytes, "result should be well under 2 MiB default")
	assert.Greater(t, len(result), 100, "result should contain the nested structure")

	// Verify the deeply nested structure was serialized
	assert.Contains(t, string(result), "deep")
	assert.Contains(t, string(result), "value")
}

func TestSerializeLogRecord_ExceedsDepthLimit(t *testing.T) {
	lr := plog.NewLogRecord()

	// Create deeply nested structure
	bodyMap := lr.Body().SetEmptyMap()
	current := bodyMap
	for i := 0; i < 5; i++ {
		current = current.PutEmptyMap("level")
	}
	current.PutStr("deep", "value")

	opts := SerializeLogRecordOptions{
		MaxDepth: 3,
	}
	_, err := SerializeLogRecord(lr, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth limit")
}

func TestSerializeLogRecord_ExceedsSizeLimit(t *testing.T) {
	lr := plog.NewLogRecord()

	// Create a large body
	largeString := strings.Repeat("a", 1000)
	lr.Body().SetStr(largeString)

	opts := SerializeLogRecordOptions{
		MaxInputBytes: 100, // Very small limit
	}
	_, err := SerializeLogRecord(lr, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds size limit")
}

func TestSerializeLogRecord_InvalidUTF8InEventName(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.SetEventName("invalid\xffutf8")
	lr.Body().SetStr("test")

	opts := SerializeLogRecordOptions{}
	_, err := SerializeLogRecord(lr, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UTF-8")
}

func TestSerializeLogRecord_CanonicalJSON(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("test")

	// Add attributes in non-alphabetical order
	lr.Attributes().PutStr("zebra", "z")
	lr.Attributes().PutStr("alpha", "a")
	lr.Attributes().PutStr("beta", "b")

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	// Canonical JSON should have keys in lexicographic order
	// The result should be deterministic
	result2, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	assert.Equal(t, result, result2, "Serialization should be deterministic")

	// Check that keys are in order in the JSON
	jsonStr := string(result)
	alphaPos := strings.Index(jsonStr, `"alpha"`)
	betaPos := strings.Index(jsonStr, `"beta"`)
	zebraPos := strings.Index(jsonStr, `"zebra"`)

	assert.True(t, alphaPos < betaPos && betaPos < zebraPos,
		"Attributes should be in alphabetical order in canonical JSON")
}

// TestSerializeLogRecord_AttributeInsertionOrderDeterministic verifies that two records
// with the same data but attributes inserted in different order produce identical
// canonical bytes. This tests the JCS key sorting requirement.
func TestSerializeLogRecord_AttributeInsertionOrderDeterministic(t *testing.T) {
	makeRecord := func(order []string) plog.LogRecord {
		lr := plog.NewLogRecord()
		lr.SetTimestamp(pcommon.Timestamp(1000000000))
		for _, k := range order {
			lr.Attributes().PutStr(k, "v")
		}
		return lr
	}

	r1 := makeRecord([]string{"z.attr", "a.attr", "m.attr"})
	r2 := makeRecord([]string{"a.attr", "m.attr", "z.attr"})

	opts := SerializeLogRecordOptions{}
	b1, err := SerializeLogRecord(r1, opts)
	require.NoError(t, err)
	b2, err := SerializeLogRecord(r2, opts)
	require.NoError(t, err)

	require.Equal(t, b1, b2, "attribute insertion order must not affect serialization (JCS key sorting)")
}

func TestMarshalJCS_Empty(t *testing.T) {
	result, err := marshalJCS(map[string]any{}, DefaultMaxInputBytes)
	require.NoError(t, err)
	assert.Equal(t, "{}", string(result))
}

func TestMarshalJCS_ExceedsSize(t *testing.T) {
	data := map[string]any{
		"key": strings.Repeat("a", 1000),
	}
	_, err := marshalJCS(data, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds size limit")
}

func TestSerializeLogRecordUint64Timestamps(t *testing.T) {
	for _, field := range []struct {
		name string
		set  func(plog.LogRecord, pcommon.Timestamp)
	}{
		{"timestamp", plog.LogRecord.SetTimestamp},
		{"observed_timestamp", plog.LogRecord.SetObservedTimestamp},
	} {
		for _, tc := range []struct {
			name      string
			timestamp pcommon.Timestamp
			decimal   string
		}{
			{"zero", 0, ""},
			{"one", 1, "1"},
			{"MaxInt64", math.MaxInt64, "9223372036854775807"},
			{"MaxInt64 plus one", math.MaxInt64 + 1, "9223372036854775808"},
			{"MaxUint64", math.MaxUint64, "18446744073709551615"},
		} {
			t.Run(field.name+"/"+tc.name, func(t *testing.T) {
				lr := plog.NewLogRecord()
				field.set(lr, tc.timestamp)
				got, err := SerializeLogRecord(lr, SerializeLogRecordOptions{})
				require.NoError(t, err)
				if tc.timestamp == 0 {
					assert.Equal(t, "{}", string(got))
				} else {
					assert.Equal(t, `{"`+field.name+`":"`+tc.decimal+`"}`, string(got))
				}
			})
		}
	}
}

func TestSerializeLogRecord_Int64Timestamps(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("test")

	// Use specific timestamp values that fit in int64
	lr.SetTimestamp(pcommon.Timestamp(1234567890123456789))
	lr.SetObservedTimestamp(pcommon.Timestamp(1234567891987654321))

	opts := SerializeLogRecordOptions{}
	result, err := SerializeLogRecord(lr, opts)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(result, &data)
	require.NoError(t, err)

	// Timestamps should be encoded as decimal strings
	assert.Equal(t, "1234567890123456789", data["timestamp"])
	assert.Equal(t, "1234567891987654321", data["observed_timestamp"])
}

// TestSerializeLogRecord_BodyTypes verifies that the log body is serialized
// correctly for every pcommon.Value type. This ensures that records with
// different body types produce distinct canonical bytes.
func TestSerializeLogRecord_BodyTypes(t *testing.T) {
	// Serialize record with no body for comparison
	emptyLR := plog.NewLogRecord()
	emptyLR.SetEventName("e")
	emptyResult, err := SerializeLogRecord(emptyLR, SerializeLogRecordOptions{})
	require.NoError(t, err)

	tests := []struct {
		name        string
		setBody     func(plog.LogRecord)
		wantContain string
	}{
		{
			name:        "string body",
			setBody:     func(lr plog.LogRecord) { lr.Body().SetStr("hello") },
			wantContain: `"body":{"stringValue":"hello"}`,
		},
		{
			name:        "int body",
			setBody:     func(lr plog.LogRecord) { lr.Body().SetInt(42) },
			wantContain: `"body":{"intValue":"42"}`,
		},
		{
			name:        "double body",
			setBody:     func(lr plog.LogRecord) { lr.Body().SetDouble(1.5) },
			wantContain: `"body":{"doubleValue":1.5}`,
		},
		{
			name:        "bool body",
			setBody:     func(lr plog.LogRecord) { lr.Body().SetBool(true) },
			wantContain: `"body":{"boolValue":true}`,
		},
		{
			name:        "bytes body",
			setBody:     func(lr plog.LogRecord) { lr.Body().SetEmptyBytes().Append(0xDE, 0xAD) },
			wantContain: `"body":{"bytesValue":"3q0="}`,
		},
		{
			name:        "slice body",
			setBody:     func(lr plog.LogRecord) { lr.Body().SetEmptySlice().AppendEmpty().SetStr("x") },
			wantContain: `"body":[{"stringValue":"x"}]`,
		},
		{
			name:        "map body",
			setBody:     func(lr plog.LogRecord) { lr.Body().SetEmptyMap().PutStr("action", "delete-all") },
			wantContain: `"body":{"action":{"stringValue":"delete-all"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			lr.SetEventName("e")
			tc.setBody(lr)

			result, err := SerializeLogRecord(lr, SerializeLogRecordOptions{})
			require.NoError(t, err)

			// Must differ from empty body
			assert.NotEqual(t, string(emptyResult), string(result),
				"body should be present in canonical payload")

			// Must contain expected body representation
			assert.Contains(t, string(result), tc.wantContain,
				"payload does not carry the expected body representation")
		})
	}
}

// TestSerializeLogRecord_BodyDistinguishesRecords verifies that two records
// differing only in their body produce distinct canonical bytes.
func TestSerializeLogRecord_BodyDistinguishesRecords(t *testing.T) {
	lr1 := plog.NewLogRecord()
	lr1.SetEventName("e")
	lr1.Body().SetEmptyMap().PutStr("action", "read")

	lr2 := plog.NewLogRecord()
	lr2.SetEventName("e")
	lr2.Body().SetEmptyMap().PutStr("action", "delete-all")

	result1, err := SerializeLogRecord(lr1, SerializeLogRecordOptions{})
	require.NoError(t, err)

	result2, err := SerializeLogRecord(lr2, SerializeLogRecordOptions{})
	require.NoError(t, err)

	assert.NotEqual(t, result1, result2,
		"records with different map bodies must produce distinct canonical bytes")
}

// TestSerializeLogRecord_EmptyBodyOmitted confirms that an unset body is
// omitted from the payload rather than being encoded as null.
func TestSerializeLogRecord_EmptyBodyOmitted(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.SetEventName("e")

	// Verify body is actually empty
	require.Equal(t, pcommon.ValueTypeEmpty, lr.Body().Type(),
		"expected unset body to be ValueTypeEmpty")

	result, err := SerializeLogRecord(lr, SerializeLogRecordOptions{})
	require.NoError(t, err)

	// Body key should not appear in output
	assert.NotContains(t, string(result), `"body"`,
		"unset body should be omitted from canonical payload")
}

// TestSerializeLogRecord_BodyInvalidUTF8Rejected verifies that invalid UTF-8
// in the body is rejected during serialization.
func TestSerializeLogRecord_BodyInvalidUTF8Rejected(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr(string([]byte{0xff, 0xfe}))

	_, err := SerializeLogRecord(lr, SerializeLogRecordOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UTF-8",
		"expected UTF-8 validation error for invalid body")
}

// TestSerializeLogRecord_BodyNestingDepthCapped verifies that deeply nested
// body structures respect the MaxDepth limit and don't overflow the stack.
func TestSerializeLogRecord_BodyNestingDepthCapped(t *testing.T) {
	lr := plog.NewLogRecord()

	// Create body nested beyond default limit
	m := lr.Body().SetEmptyMap()
	for i := 0; i < DefaultMaxDepth+1; i++ {
		m = m.PutEmptyMap("n")
	}

	_, err := SerializeLogRecord(lr, SerializeLogRecordOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nesting depth limit",
		"expected depth limit error for deeply nested body")
}

// TestSerializeLogRecord_TimestampPrecision verifies that timestamps differing
// by 1 nanosecond produce distinct canonical bytes. This ensures int64 timestamps
// are serialized as strings, not as float64 (which would lose precision for
// values in the ~1.7e18 range).
func TestSerializeLogRecord_TimestampPrecision(t *testing.T) {
	base := pcommon.Timestamp(1714041600000000000)

	lr1 := plog.NewLogRecord()
	lr1.SetTimestamp(base)
	lr1.Body().SetStr("test")

	lr2 := plog.NewLogRecord()
	lr2.SetTimestamp(base + 1) // 1 nanosecond later
	lr2.Body().SetStr("test")

	result1, err := SerializeLogRecord(lr1, SerializeLogRecordOptions{})
	require.NoError(t, err)

	result2, err := SerializeLogRecord(lr2, SerializeLogRecordOptions{})
	require.NoError(t, err)

	assert.NotEqual(t, result1, result2,
		"timestamps differing by 1ns must produce distinct canonical bytes")

	// Verify timestamp appears as quoted string
	assert.Contains(t, string(result1), `"timestamp":"1714041600000000000"`)
	assert.Contains(t, string(result2), `"timestamp":"1714041600000000001"`)
}

// TestSerializeLogRecord_ObservedTimestampPrecision is the same check for ObservedTimestamp.
func TestSerializeLogRecord_ObservedTimestampPrecision(t *testing.T) {
	base := pcommon.Timestamp(1714041600000000000)

	lr1 := plog.NewLogRecord()
	lr1.SetObservedTimestamp(base)
	lr1.Body().SetStr("test")

	lr2 := plog.NewLogRecord()
	lr2.SetObservedTimestamp(base + 1)
	lr2.Body().SetStr("test")

	result1, err := SerializeLogRecord(lr1, SerializeLogRecordOptions{})
	require.NoError(t, err)

	result2, err := SerializeLogRecord(lr2, SerializeLogRecordOptions{})
	require.NoError(t, err)

	assert.NotEqual(t, result1, result2,
		"observed_timestamps differing by 1ns must produce distinct canonical bytes")

	// Verify observed_timestamp appears as quoted string
	assert.Contains(t, string(result1), `"observed_timestamp":"1714041600000000000"`)
	assert.Contains(t, string(result2), `"observed_timestamp":"1714041600000000001"`)
}

// minVectors stops a bump that drops vectors from passing on fewer of them.
const minVectors = 50

var canonicalLeaf = regexp.MustCompile(`"(?:[^"\\]|\\.)*":?|true|false|-?\d[-+.0-9e]*`)

// TestRFC8785Vectors runs published RFC 8785 vectors through
// SerializeLogRecord, so a jcs or Go bump that moves the signed bytes fails
// here. The vectors come from a module pinned in go.mod and go.sum.
//
// Only the accept cases are run. The reject cases are lone surrogates,
// non-finite numbers and duplicate members, which never reach jcs.Transform
// on this path: TestSerializeLogRecord_BodyInvalidUTF8Rejected and float64
// special values tests cover the first two, and a pcommon.Map cannot hold a
// duplicate member.
func TestRFC8785Vectors(t *testing.T) {
	vectors, err := jcsvectors.Canonical()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(vectors), minVectors)

	for _, vector := range vectors {
		t.Run(vector.Corpus+"/"+vector.ID, func(t *testing.T) {
			var decoded any
			require.NoError(t, json.Unmarshal(vector.Input, &decoded))
			value := pcommon.NewValueEmpty()
			require.NoError(t, value.FromRaw(decoded))
			lr := plog.NewLogRecord()
			value.CopyTo(lr.Attributes().PutEmpty("v"))

			opts := SerializeLogRecordOptions{
				ExcludePrefixes: []string{"audit.integrity."},
			}
			got, err := SerializeLogRecord(lr, opts)
			require.NoError(t, err)
			require.Equal(t, `{"attributes":{"v":`+wrapLeaves(string(vector.Canonical))+`}}`, string(got))
		})
	}
}

// wrapLeaves gives canonical JSON the shape valueToInterface gives the same
// value: each scalar becomes a one-member object, and member order is unchanged.
func wrapLeaves(canonical string) string {
	return canonicalLeaf.ReplaceAllStringFunc(canonical, func(leaf string) string {
		switch {
		case strings.HasSuffix(leaf, `":`):
			return leaf // a member name
		case leaf[0] == '"':
			return `{"stringValue":` + leaf + `}`
		case leaf == "true" || leaf == "false":
			return `{"boolValue":` + leaf + `}`
		default:
			return `{"doubleValue":` + leaf + `}`
		}
	})
}

// The fixture is independent of the signing processor and includes both signed
// content and transport metadata that must never enter the canonical payload.
func newCanonicalTestSpan() ptrace.Span {
	s := ptrace.NewSpan()
	s.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	s.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
	s.SetParentSpanID(pcommon.SpanID{8, 7, 6, 5, 4, 3, 2, 1})
	s.SetName("tool.call")
	s.SetKind(ptrace.SpanKindClient)
	s.SetStartTimestamp(1714041600000000000)
	s.SetEndTimestamp(1714041600500000000)
	s.Attributes().PutStr("gen_ai.tool.name", "search")
	s.TraceState().FromRaw("vendor=value")
	s.SetFlags(1)
	s.SetDroppedAttributesCount(2)
	s.SetDroppedEventsCount(3)
	s.SetDroppedLinksCount(4)
	e := s.Events().AppendEmpty()
	e.SetName("decision")
	e.SetTimestamp(1714041600250000000)
	e.Attributes().PutStr("choice", "a")
	e.SetDroppedAttributesCount(5)
	l := s.Links().AppendEmpty()
	l.SetTraceID(pcommon.TraceID{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
	l.SetSpanID(pcommon.SpanID{2, 2, 2, 2, 2, 2, 2, 2})
	l.Attributes().PutStr("link.kind", "follows")
	l.TraceState().FromRaw("vendor=value")
	l.SetFlags(1)
	l.SetDroppedAttributesCount(6)
	s.Status().SetCode(ptrace.StatusCodeError)
	s.Status().SetMessage("tool failed")
	return s
}

const canonicalTestSpanJSON = `{"attributes":{"gen_ai.tool.name":{"stringValue":"search"}},"end_time":"1714041600500000000","events":[{"attributes":{"choice":{"stringValue":"a"}},"name":"decision","timestamp":"1714041600250000000"}],"kind":3,"links":[{"attributes":{"link.kind":{"stringValue":"follows"}},"span_id":"0202020202020202","trace_id":"100f0e0d0c0b0a090807060504030201"}],"name":"tool.call","parent_span_id":"0807060504030201","span_id":"0102030405060708","start_time":"1714041600000000000","status":{"code":2,"message":"tool failed"},"trace_id":"0102030405060708090a0b0c0d0e0f10"}`

func TestSerializeSpanFullPayload(t *testing.T) {
	got, err := SerializeSpan(newCanonicalTestSpan(), SerializeSpanOptions{})
	require.NoError(t, err)
	assert.Equal(t, canonicalTestSpanJSON, string(got))
}

func TestSerializeSpanEmptyFields(t *testing.T) {
	s := ptrace.NewSpan()
	got, err := SerializeSpan(s, SerializeSpanOptions{})
	require.NoError(t, err)
	assert.Equal(t, `{"kind":0,"status":{"code":0}}`, string(got))

	// Empty elements retain their positions but omit every zero-valued field.
	s.Events().AppendEmpty()
	s.Links().AppendEmpty()
	got, err = SerializeSpan(s, SerializeSpanOptions{})
	require.NoError(t, err)
	assert.Equal(t, `{"events":[{}],"kind":0,"links":[{}],"status":{"code":0}}`, string(got))
}

var spanAttributeLevels = []struct {
	name  string
	attrs func(ptrace.Span) pcommon.Map
}{
	{"span", func(s ptrace.Span) pcommon.Map { return s.Attributes() }},
	{"event", func(s ptrace.Span) pcommon.Map { return s.Events().AppendEmpty().Attributes() }},
	{"link", func(s ptrace.Span) pcommon.Map { return s.Links().AppendEmpty().Attributes() }},
}

func spanAttributesJSON(level, attrs string) string {
	switch level {
	case "event":
		return `{"events":[{"attributes":` + attrs + `}],"kind":0,"status":{"code":0}}`
	case "link":
		return `{"kind":0,"links":[{"attributes":` + attrs + `}],"status":{"code":0}}`
	default:
		return `{"attributes":` + attrs + `,"kind":0,"status":{"code":0}}`
	}
}

func TestSerializeSpanAttributeTypes(t *testing.T) {
	// Handwritten expected bytes pin typed primitives, empty values, nested
	// containers, integer precision, base64, and JCS's unescaped HTML characters.
	const wantAttrs = `{"bool":{"boolValue":false},"bytes":{"bytesValue":"3q0="},"double":{"doubleValue":1.5},"empty":null,"empty_bytes":{"bytesValue":""},"empty_map":{},"empty_slice":[],"int":{"intValue":"-9223372036854775808"},"map":{"a":{"intValue":"9223372036854775807"},"z":{"stringValue":"é<&>"}},"slice":[{"stringValue":"first"},{"boolValue":true},{"nested":[{"doubleValue":2.5},null]}],"string":{"stringValue":"hello\n\"\\"}}`
	for _, level := range spanAttributeLevels {
		t.Run(level.name, func(t *testing.T) {
			s := ptrace.NewSpan()
			a := level.attrs(s)
			a.PutStr("string", "hello\n\"\\")
			a.PutInt("int", math.MinInt64)
			a.PutDouble("double", 1.5)
			a.PutBool("bool", false)
			a.PutEmptyBytes("bytes").Append(0xde, 0xad)
			a.PutEmpty("empty")
			a.PutEmptyBytes("empty_bytes")
			a.PutEmptyMap("empty_map")
			a.PutEmptySlice("empty_slice")
			m := a.PutEmptyMap("map")
			m.PutStr("z", "é<&>")
			m.PutInt("a", math.MaxInt64)
			v := a.PutEmptySlice("slice")
			v.AppendEmpty().SetStr("first")
			v.AppendEmpty().SetBool(true)
			nested := v.AppendEmpty().SetEmptyMap().PutEmptySlice("nested")
			nested.AppendEmpty().SetDouble(2.5)
			nested.AppendEmpty()
			got, err := SerializeSpan(s, SerializeSpanOptions{})
			require.NoError(t, err)
			assert.Equal(t, spanAttributesJSON(level.name, wantAttrs), string(got))
		})
	}
}

func TestSerializeSpanExcludedFields(t *testing.T) {
	s := newCanonicalTestSpan()
	opts := SerializeSpanOptions{ExcludePrefixes: []string{"audit.integrity."}}
	before, err := SerializeSpan(s, opts)
	require.NoError(t, err)
	require.Equal(t, canonicalTestSpanJSON, string(before))
	s.TraceState().FromRaw("vendor=changed")
	s.SetFlags(0)
	s.SetDroppedAttributesCount(9)
	s.SetDroppedEventsCount(9)
	s.SetDroppedLinksCount(9)
	s.Events().At(0).SetDroppedAttributesCount(9)
	s.Links().At(0).TraceState().FromRaw("vendor=changed")
	s.Links().At(0).SetFlags(0)
	s.Links().At(0).SetDroppedAttributesCount(9)
	s.Attributes().PutStr("audit.integrity.value", "x")
	s.Events().At(0).Attributes().PutStr("audit.integrity.value", "x")
	s.Links().At(0).Attributes().PutStr("audit.integrity.value", "x")
	after, err := SerializeSpan(s, opts)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestSerializeSpanExcludePrefixes(t *testing.T) {
	for _, level := range spanAttributeLevels {
		t.Run(level.name, func(t *testing.T) {
			s := ptrace.NewSpan()
			a := level.attrs(s)
			a.PutStr("audit.integrity.value", "signature")
			a.PutStr("audit.verification.status", "verified")
			a.PutStr("audit.integrity", "keep") // Matching is by prefix, not namespace.
			a.PutEmptyMap("nested").PutStr("audit.integrity.value", "nested content")
			opts := SerializeSpanOptions{ExcludePrefixes: []string{"audit.integrity.", "audit.verification."}}
			got, err := SerializeSpan(s, opts)
			require.NoError(t, err)
			assert.Equal(t, spanAttributesJSON(level.name, `{"audit.integrity":{"stringValue":"keep"},"nested":{"audit.integrity.value":{"stringValue":"nested content"}}}`), string(got))

			// The library must not implicitly filter signing attributes.
			got, err = SerializeSpan(s, SerializeSpanOptions{})
			require.NoError(t, err)
			assert.Equal(t, spanAttributesJSON(level.name, `{"audit.integrity":{"stringValue":"keep"},"audit.integrity.value":{"stringValue":"signature"},"audit.verification.status":{"stringValue":"verified"},"nested":{"audit.integrity.value":{"stringValue":"nested content"}}}`), string(got))

			// All filtered attributes are omitted, not encoded as an empty object.
			got, err = SerializeSpan(s, SerializeSpanOptions{ExcludePrefixes: []string{""}})
			require.NoError(t, err)
			want := `{"kind":0,"status":{"code":0}}`
			if level.name == "event" {
				want = `{"events":[{}],"kind":0,"status":{"code":0}}`
			} else if level.name == "link" {
				want = `{"kind":0,"links":[{}],"status":{"code":0}}`
			}
			assert.Equal(t, want, string(got))

			// Filter before validation/conversion: excluded content is not signed.
			a.Clear()
			a.PutStr("audit.integrity.bad\xff", "bad\xff")
			a.PutDouble("audit.integrity.nan", math.NaN())
			deep := a.PutEmptyMap("audit.integrity.deep")
			for range DefaultMaxDepth + 1 {
				deep = deep.PutEmptyMap("n")
			}
			_, err = SerializeSpan(s, opts)
			require.NoError(t, err)
		})
	}
}

func TestSerializeSpanDeterministicInsertionOrder(t *testing.T) {
	makeSpan := func(keys []string) ptrace.Span {
		s := ptrace.NewSpan()
		for _, level := range spanAttributeLevels {
			a := level.attrs(s)
			for _, key := range keys {
				m := a.PutEmptyMap(key)
				for _, nestedKey := range keys {
					m.PutStr(nestedKey, nestedKey)
				}
			}
		}
		return s
	}
	a, err := SerializeSpan(makeSpan([]string{"z", "a", "m"}), SerializeSpanOptions{})
	require.NoError(t, err)
	b, err := SerializeSpan(makeSpan([]string{"m", "z", "a"}), SerializeSpanOptions{})
	require.NoError(t, err)
	assert.Equal(t, a, b)
	const nested = `{"a":{"stringValue":"a"},"m":{"stringValue":"m"},"z":{"stringValue":"z"}}`
	const attrs = `{"a":` + nested + `,"m":` + nested + `,"z":` + nested + `}`
	assert.Equal(t, `{"attributes":`+attrs+`,"events":[{"attributes":`+attrs+`}],"kind":0,"links":[{"attributes":`+attrs+`}],"status":{"code":0}}`, string(a))
}

func TestSerializeSpanPreservesEventAndLinkOrder(t *testing.T) {
	s := ptrace.NewSpan()
	s.Events().AppendEmpty().SetName("z")
	s.Events().AppendEmpty().SetName("a")
	s.Links().AppendEmpty().SetSpanID(pcommon.SpanID{2})
	s.Links().AppendEmpty().SetSpanID(pcommon.SpanID{1})
	got, err := SerializeSpan(s, SerializeSpanOptions{})
	require.NoError(t, err)
	assert.Equal(t, `{"events":[{"name":"z"},{"name":"a"}],"kind":0,"links":[{"span_id":"0200000000000000"},{"span_id":"0100000000000000"}],"status":{"code":0}}`, string(got))

	s.Events().At(0).SetName("a")
	s.Events().At(1).SetName("z")
	eventsReversed, err := SerializeSpan(s, SerializeSpanOptions{})
	require.NoError(t, err)
	assert.NotEqual(t, got, eventsReversed)
	s.Links().At(0).SetSpanID(pcommon.SpanID{1})
	s.Links().At(1).SetSpanID(pcommon.SpanID{2})
	linksReversed, err := SerializeSpan(s, SerializeSpanOptions{})
	require.NoError(t, err)
	assert.NotEqual(t, eventsReversed, linksReversed)
}

func TestSerializeSpanRejectsInvalidUTF8(t *testing.T) {
	for _, tc := range []struct {
		name    string
		set     func(ptrace.Span)
		context string
	}{
		{"span name", func(s ptrace.Span) { s.SetName("bad\xff") }, "span name"},
		{"event name", func(s ptrace.Span) { s.Events().AppendEmpty().SetName("bad\xff") }, "event name"},
		{"status message", func(s ptrace.Span) { s.Status().SetMessage("bad\xff") }, "status message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ptrace.NewSpan()
			tc.set(s)
			got, err := SerializeSpan(s, SerializeSpanOptions{})
			require.ErrorContains(t, err, "invalid UTF-8")
			assert.ErrorContains(t, err, tc.context)
			assert.Nil(t, got)
		})
	}
	for _, level := range spanAttributeLevels {
		for _, tc := range []struct {
			name string
			set  func(pcommon.Map)
		}{
			{"attribute key", func(a pcommon.Map) { a.PutStr("bad\xff", "valid") }},
			{"attribute string", func(a pcommon.Map) { a.PutStr("key", "bad\xff") }},
			{"nested map key", func(a pcommon.Map) { a.PutEmptyMap("map").PutStr("bad\xff", "valid") }},
			{"nested map string", func(a pcommon.Map) { a.PutEmptyMap("map").PutStr("key", "bad\xff") }},
			{"nested slice string", func(a pcommon.Map) { a.PutEmptySlice("slice").AppendEmpty().SetStr("bad\xff") }},
			{"map inside slice", func(a pcommon.Map) { a.PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("bad\xff", "valid") }},
		} {
			t.Run(level.name+"/"+tc.name, func(t *testing.T) {
				s := ptrace.NewSpan()
				tc.set(level.attrs(s))
				got, err := SerializeSpan(s, SerializeSpanOptions{})
				require.ErrorContains(t, err, "invalid UTF-8")
				assert.Nil(t, got)
			})
		}
	}
}

func TestSerializeSpanAcceptsValidUTF8(t *testing.T) {
	s := ptrace.NewSpan()
	s.SetName("é<&>")
	s.Events().AppendEmpty().SetName("決定")
	s.Status().SetMessage("✓")
	s.Attributes().PutEmptyMap("é").PutStr("決定", `\ud800`)
	s.Events().At(0).Attributes().PutStr("é", "✓")
	s.Links().AppendEmpty().Attributes().PutStr("é", "✓")
	got, err := SerializeSpan(s, SerializeSpanOptions{})
	require.NoError(t, err)
	assert.Equal(t, `{"attributes":{"é":{"決定":{"stringValue":"\\ud800"}}},"events":[{"attributes":{"é":{"stringValue":"✓"}},"name":"決定"}],"kind":0,"links":[{"attributes":{"é":{"stringValue":"✓"}}}],"name":"é<&>","status":{"code":0,"message":"✓"}}`, string(got))
}

func TestSerializeSpanRejectsNonFiniteAttributes(t *testing.T) {
	for _, level := range spanAttributeLevels {
		for _, number := range []struct {
			name  string
			value float64
		}{{"NaN", math.NaN()}, {"positive infinity", math.Inf(1)}, {"negative infinity", math.Inf(-1)}} {
			for _, container := range []string{"primitive", "map", "slice"} {
				t.Run(level.name+"/"+number.name+"/"+container, func(t *testing.T) {
					s := ptrace.NewSpan()
					a := level.attrs(s)
					v := a.PutEmpty("value")
					switch container {
					case "map":
						v = v.SetEmptyMap().PutEmpty("nested")
					case "slice":
						v = v.SetEmptySlice().AppendEmpty()
					}
					v.SetDouble(number.value)
					got, err := SerializeSpan(s, SerializeSpanOptions{})
					require.Error(t, err)
					assert.Nil(t, got)
				})
			}
		}
	}
}

func TestSerializeSpanDepthBoundaries(t *testing.T) {
	for _, level := range spanAttributeLevels {
		for _, container := range []string{"map", "slice"} {
			for _, limit := range []struct {
				name       string
				configured int
				effective  int
			}{{"configured", 3, 3}, {"default", 0, DefaultMaxDepth}} {
				t.Run(level.name+"/"+container+"/"+limit.name, func(t *testing.T) {
					for _, depth := range []int{limit.effective, limit.effective + 1} {
						s := ptrace.NewSpan()
						v := level.attrs(s).PutEmpty("nested")
						// Attribute values begin at depth zero; each container edge
						// adds one. The scalar exactly at the cap is valid.
						for range depth {
							if container == "map" {
								v = v.SetEmptyMap().PutEmpty("n")
							} else {
								v = v.SetEmptySlice().AppendEmpty()
							}
						}
						v.SetStr("leaf")
						got, err := SerializeSpan(s, SerializeSpanOptions{MaxDepth: limit.configured})
						if depth == limit.effective {
							require.NoError(t, err)
							assert.Contains(t, string(got), `{"stringValue":"leaf"}`)
						} else {
							require.ErrorContains(t, err, "nesting depth limit")
							assert.Nil(t, got)
						}
					}
				})
			}
		}
	}
}

func TestSerializeSpanSizeBoundaries(t *testing.T) {
	for _, level := range spanAttributeLevels {
		for _, limit := range []struct {
			name       string
			configured int
			effective  int
		}{{"configured", 256, 256}, {"default", 0, DefaultMaxInputBytes}} {
			t.Run(level.name+"/"+limit.name, func(t *testing.T) {
				const emptyAttrs = `{"value":{"stringValue":""}}`
				overhead := len(spanAttributesJSON(level.name, emptyAttrs))
				for _, extra := range []int{0, 1} {
					s := ptrace.NewSpan()
					value := strings.Repeat("a", limit.effective-overhead+extra)
					level.attrs(s).PutStr("value", value)
					got, err := SerializeSpan(s, SerializeSpanOptions{MaxInputBytes: limit.configured})
					if extra == 0 {
						require.NoError(t, err)
						assert.Len(t, got, limit.effective)
						assert.Equal(t, spanAttributesJSON(level.name, `{"value":{"stringValue":"`+value+`"}}`), string(got))
					} else {
						require.ErrorContains(t, err, "size limit")
						assert.Nil(t, got)
					}
				}
			})
		}
	}
}

func TestSerializeSpanSizeLimitBeforeCanonicalization(t *testing.T) {
	s := ptrace.NewSpan()
	s.SetName("<&>")
	const canonical = `{"kind":0,"name":"<&>","status":{"code":0}}`
	// encoding/json escapes each HTML character as six bytes before JCS
	// removes those escapes. The size cap applies to that larger input.
	const rawSize = len(canonical) + 3*5
	got, err := SerializeSpan(s, SerializeSpanOptions{MaxInputBytes: rawSize})
	require.NoError(t, err)
	assert.Equal(t, canonical, string(got))
	got, err = SerializeSpan(s, SerializeSpanOptions{MaxInputBytes: rawSize - 1})
	require.ErrorContains(t, err, "size limit")
	assert.Nil(t, got)
}

func TestSerializeSpanUint64Timestamps(t *testing.T) {
	for _, tc := range []struct {
		name      string
		timestamp pcommon.Timestamp
		decimal   string
	}{
		{"MaxInt64", math.MaxInt64, "9223372036854775807"},
		{"MaxInt64 plus one", pcommon.Timestamp(uint64(math.MaxInt64) + 1), "9223372036854775808"},
		{"MaxUint64", math.MaxUint64, "18446744073709551615"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ptrace.NewSpan()
			s.SetStartTimestamp(tc.timestamp)
			s.SetEndTimestamp(tc.timestamp)
			s.Events().AppendEmpty().SetTimestamp(tc.timestamp)
			got, err := SerializeSpan(s, SerializeSpanOptions{})
			require.NoError(t, err)
			assert.Equal(t, `{"end_time":"`+tc.decimal+`","events":[{"timestamp":"`+tc.decimal+`"}],"kind":0,"start_time":"`+tc.decimal+`","status":{"code":0}}`, string(got))
		})
	}
}

func TestSerializeSpanDoesNotMutate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		opts      SerializeSpanOptions
		wantError bool
	}{
		{"success", SerializeSpanOptions{ExcludePrefixes: []string{"audit.integrity."}}, false},
		{"depth error", SerializeSpanOptions{MaxDepth: 1}, true},
		{"size error", SerializeSpanOptions{MaxInputBytes: 1}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newCanonicalTestSpan()
			s.Attributes().PutStr("audit.integrity.value", "signature")
			s.Attributes().PutEmptyMap("nested").PutEmptySlice("list").AppendEmpty().SetStr("content")
			td := ptrace.NewTraces()
			s.CopyTo(td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty())
			marshaler := &ptrace.ProtoMarshaler{}
			before, err := marshaler.MarshalTraces(td)
			require.NoError(t, err)
			_, err = SerializeSpan(s, tc.opts)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			s.CopyTo(td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))
			after, err := marshaler.MarshalTraces(td)
			require.NoError(t, err)
			assert.Equal(t, before, after)
			// The serialized bytes are also ordinary valid JSON.
			if !tc.wantError {
				got, serializeErr := SerializeSpan(s, tc.opts)
				require.NoError(t, serializeErr)
				assert.True(t, json.Valid(got))
			}
		})
	}
}
