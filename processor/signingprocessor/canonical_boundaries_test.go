// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// These tests pin the boundaries of the canonical payload: where nesting stops,
// what a string may contain, and which distinct records must not share bytes.
// Each one is a guard for a defect that was reproduced against an earlier head
// of this processor and is measured again here against the current code.

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

func recordWithAttr(set func(pcommon.Map)) plog.LogRecord {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("body")
	set(lr.Attributes())
	return lr
}

// The depth cap is consulted while the OTLP value is walked, before any of it
// reaches json.Marshal, so a value one level past the cap is refused and never
// grows a Go stack. A cap that sat after json.Marshal would overflow the stack
// on the same input before it was ever read.
func TestDepthCapIsCheckedBeforeMarshal(t *testing.T) {
	p := &signingProcessor{}
	lr := recordWithAttr(func(m pcommon.Map) {
		nestedSliceValue(jsonMaxDepth + 1).CopyTo(m.PutEmpty("deep"))
	})
	_, err := p.serializeLogRecord(lr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nesting depth limit")

	lr = recordWithAttr(func(m pcommon.Map) {
		nestedSliceValue(jsonMaxDepth - 1).CopyTo(m.PutEmpty("deep"))
	})
	_, err = p.serializeLogRecord(lr)
	require.NoError(t, err)
}

// Brace characters inside a string value are content, not structure. A record
// whose true depth is two and whose one string holds 200 '{' must be accepted,
// and a record whose true depth exceeds the cap must be refused even when a
// sibling string holds 400 '}'. A byte scan over the marshaled JSON gets both
// of these wrong, in opposite directions.
func TestStringBracesAreNotStructure(t *testing.T) {
	p := &signingProcessor{}

	shallow := recordWithAttr(func(m pcommon.Map) {
		m.PutStr("note", strings.Repeat("{", 200))
	})
	_, err := p.serializeLogRecord(shallow)
	require.NoError(t, err, "a string of 200 '{' is two levels deep, not 200")

	masked := recordWithAttr(func(m pcommon.Map) {
		m.PutStr("a", strings.Repeat("}", 400))
		nestedSliceValue(jsonMaxDepth + 100).CopyTo(m.PutEmpty("b"))
	})
	_, err = p.serializeLogRecord(masked)
	require.Error(t, err, "a sibling string of '}' must not mask real depth")
	require.Contains(t, err.Error(), "nesting depth limit")
}

// The text `\ud800` (six characters: a backslash, a u and four hex digits) is
// valid UTF-8 and must survive as text. json.Marshal escapes the backslash, so
// two records that differ only in which surrogate they spell out must stay
// distinct. The unescaped byte forms are refused earlier as invalid UTF-8.
func TestEscapedSurrogateTextDoesNotCollide(t *testing.T) {
	p := &signingProcessor{}
	a := recordWithAttr(func(m pcommon.Map) { m.PutStr("k", `\ud800\ud800`) })
	b := recordWithAttr(func(m pcommon.Map) { m.PutStr("k", `\udfff\udfff`) })
	outA, err := p.serializeLogRecord(a)
	require.NoError(t, err)
	outB, err := p.serializeLogRecord(b)
	require.NoError(t, err)
	require.NotEqual(t, string(outA), string(outB))

	bad := recordWithAttr(func(m pcommon.Map) { m.PutStr("k", string([]byte{0xed, 0xa0, 0x80})) })
	_, err = p.serializeLogRecord(bad)
	require.Error(t, err, "an encoded surrogate is invalid UTF-8 and must be refused")
}

// The UTF-8 check on string values has to hold for every string in the
// payload. json.Marshal coerces an invalid byte in a map key or in the event
// name to U+FFFD, so two records whose keys differ only in such bytes would
// canonicalize to the same bytes and share one signature. pdata copies keys
// from the wire without validation, so this input reaches the processor.
func TestKeysAndEventNameAreUTF8Checked(t *testing.T) {
	p := &signingProcessor{}
	bad := "k" + string([]byte{0xff})

	lr := recordWithAttr(func(m pcommon.Map) { m.PutStr(bad, "v") })
	_, err := p.serializeLogRecord(lr)
	require.Error(t, err, "an attribute key with invalid UTF-8 must be refused")
	require.Contains(t, err.Error(), "invalid UTF-8")

	lr = recordWithAttr(func(m pcommon.Map) { m.PutEmptyMap("m").PutStr(bad, "v") })
	_, err = p.serializeLogRecord(lr)
	require.Error(t, err, "a nested map key with invalid UTF-8 must be refused")
	require.Contains(t, err.Error(), "invalid UTF-8")

	lr = plog.NewLogRecord()
	lr.Body().SetStr("body")
	lr.SetEventName("ev" + string([]byte{0xff}))
	_, err = p.serializeLogRecord(lr)
	require.Error(t, err, "an event name with invalid UTF-8 must be refused")
	require.Contains(t, err.Error(), "invalid UTF-8")

	lr = recordWithAttr(func(m pcommon.Map) {
		m.PutEmptyMap("m").PutStr("ké", "v")
	})
	lr.SetEventName("evé")
	_, err = p.serializeLogRecord(lr)
	require.NoError(t, err, "valid non-ASCII UTF-8 in keys and the event name is accepted")
}
