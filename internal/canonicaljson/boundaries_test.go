// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package canonicaljson - Serialization Edge Case and Boundary Tests
//
// This file contains tests that pin the boundaries of the canonical payload:
// where nesting stops, what a string may contain, and which distinct records
// must not share bytes.
//
// Each test is a guard for a defect that was reproduced against an earlier
// implementation and is measured again here against the current code.
//
// These tests cover edge cases across multiple source files (serialize.go, convert.go, validate.go)
// to ensure the complete serialization pipeline handles boundary conditions correctly.

package canonicaljson

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// nestedSliceValue creates a nested slice structure of the given depth.
func nestedSliceValue(depth int) pcommon.Value {
	root := pcommon.NewValueSlice()
	cur := root
	for range depth {
		child := cur.Slice().AppendEmpty()
		child.SetEmptySlice()
		cur = child
	}
	return root
}

// recordWithAttr creates a log record with a body and attributes set by the given function.
func recordWithAttr(set func(pcommon.Map)) plog.LogRecord {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("body")
	set(lr.Attributes())
	return lr
}

// TestDepthCapIsCheckedBeforeMarshal verifies that the depth cap is consulted
// while the OTLP value is walked, before any of it reaches json.Marshal.
// A value one level past the cap is refused and never grows a Go stack.
// A cap that sat after json.Marshal would overflow the stack on the same input
// before it was ever read.
func TestDepthCapIsCheckedBeforeMarshal(t *testing.T) {
	// Test with depth exceeding the limit
	lr := recordWithAttr(func(m pcommon.Map) {
		nestedSliceValue(DefaultMaxDepth + 1).CopyTo(m.PutEmpty("deep"))
	})
	opts := SerializeLogRecordOptions{
		ExcludePrefixes: []string{"audit.integrity."},
	}
	_, err := SerializeLogRecord(lr, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nesting depth limit")

	// Test with depth within the limit
	lr = recordWithAttr(func(m pcommon.Map) {
		nestedSliceValue(DefaultMaxDepth - 1).CopyTo(m.PutEmpty("deep"))
	})
	_, err = SerializeLogRecord(lr, opts)
	require.NoError(t, err)
}

// TestStringBracesAreNotStructure verifies that brace characters inside a
// string value are content, not structure. A record whose true depth is two
// and whose one string holds 200 '{' must be accepted, and a record whose
// true depth exceeds the cap must be refused even when a sibling string holds
// 400 '}'. A byte scan over the marshaled JSON gets both of these wrong, in
// opposite directions.
func TestStringBracesAreNotStructure(t *testing.T) {
	opts := SerializeLogRecordOptions{}

	// A string of 200 '{' is two levels deep, not 200
	shallow := recordWithAttr(func(m pcommon.Map) {
		m.PutStr("note", strings.Repeat("{", 200))
	})
	_, err := SerializeLogRecord(shallow, opts)
	require.NoError(t, err, "a string of 200 '{' is two levels deep, not 200")

	// A sibling string of '}' must not mask real depth
	masked := recordWithAttr(func(m pcommon.Map) {
		m.PutStr("a", strings.Repeat("}", 400))
		nestedSliceValue(DefaultMaxDepth + 100).CopyTo(m.PutEmpty("b"))
	})
	_, err = SerializeLogRecord(masked, opts)
	require.Error(t, err, "a sibling string of '}' must not mask real depth")
	require.Contains(t, err.Error(), "nesting depth limit")
}

// TestEscapedSurrogateTextDoesNotCollide verifies that the text `\ud800`
// (six characters: a backslash, a u and four hex digits) is valid UTF-8 and
// must survive as text. json.Marshal escapes the backslash, so two records
// that differ only in which surrogate they spell out must stay distinct.
// The unescaped byte forms are refused earlier as invalid UTF-8.
func TestEscapedSurrogateTextDoesNotCollide(t *testing.T) {
	opts := SerializeLogRecordOptions{}

	// Two different escaped surrogate texts must produce different output
	a := recordWithAttr(func(m pcommon.Map) { m.PutStr("k", `\ud800\ud800`) })
	b := recordWithAttr(func(m pcommon.Map) { m.PutStr("k", `\udfff\udfff`) })
	outA, err := SerializeLogRecord(a, opts)
	require.NoError(t, err)
	outB, err := SerializeLogRecord(b, opts)
	require.NoError(t, err)
	require.NotEqual(t, string(outA), string(outB),
		"two records with different escaped surrogates must produce different output")

	// An encoded surrogate is invalid UTF-8 and must be refused
	bad := recordWithAttr(func(m pcommon.Map) { m.PutStr("k", string([]byte{0xed, 0xa0, 0x80})) })
	_, err = SerializeLogRecord(bad, opts)
	require.Error(t, err, "an encoded surrogate is invalid UTF-8 and must be refused")
	require.Contains(t, err.Error(), "UTF-8")
}

// TestKeysAndEventNameAreUTF8Checked verifies that the UTF-8 check on string
// values holds for every string in the payload. json.Marshal coerces an
// invalid byte in a map key or in the event name to U+FFFD, so two records
// whose keys differ only in such bytes would canonicalize to the same bytes
// and share one signature. pdata copies keys from the wire without validation,
// so this input reaches the serializer.
func TestKeysAndEventNameAreUTF8Checked(t *testing.T) {
	opts := SerializeLogRecordOptions{}
	bad := "k" + string([]byte{0xff})

	// An attribute key with invalid UTF-8 must be refused
	lr := recordWithAttr(func(m pcommon.Map) { m.PutStr(bad, "v") })
	_, err := SerializeLogRecord(lr, opts)
	require.Error(t, err, "an attribute key with invalid UTF-8 must be refused")
	require.Contains(t, err.Error(), "invalid UTF-8")

	// A nested map key with invalid UTF-8 must be refused
	lr = recordWithAttr(func(m pcommon.Map) { m.PutEmptyMap("m").PutStr(bad, "v") })
	_, err = SerializeLogRecord(lr, opts)
	require.Error(t, err, "a nested map key with invalid UTF-8 must be refused")
	require.Contains(t, err.Error(), "invalid UTF-8")

	// An event name with invalid UTF-8 must be refused
	lr = plog.NewLogRecord()
	lr.Body().SetStr("body")
	lr.SetEventName("ev" + string([]byte{0xff}))
	_, err = SerializeLogRecord(lr, opts)
	require.Error(t, err, "an event name with invalid UTF-8 must be refused")
	require.Contains(t, err.Error(), "invalid UTF-8")

	// Valid non-ASCII UTF-8 in keys and the event name is accepted
	lr = recordWithAttr(func(m pcommon.Map) {
		m.PutEmptyMap("m").PutStr("ké", "v")
	})
	lr.SetEventName("evé")
	_, err = SerializeLogRecord(lr, opts)
	require.NoError(t, err, "valid non-ASCII UTF-8 in keys and the event name is accepted")
}
