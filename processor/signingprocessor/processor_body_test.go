// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// serializeBody builds a record with the given body and returns its canonical
// signed payload.
func serializeBody(t *testing.T, set func(plog.LogRecord)) string {
	t.Helper()
	p := &signingProcessor{config: &Config{}}
	lr := plog.NewLogRecord()
	lr.SetEventName("e")
	set(lr)
	b, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serializeLogRecord: %v", err)
	}
	return string(b)
}

// TestBodyIsSignedForEveryType verifies that the log body is part of the canonical
// payload whatever its type. Before this was fixed only a string body was included,
// so a record with a structured body and a record with no body at all produced
// identical bytes and therefore shared a signature.
func TestBodyIsSignedForEveryType(t *testing.T) {
	empty := serializeBody(t, func(plog.LogRecord) {})

	tests := []struct {
		name string
		set  func(plog.LogRecord)
		want string
	}{
		{
			name: "string body",
			set:  func(lr plog.LogRecord) { lr.Body().SetStr("hello") },
			want: `"body":"hello"`,
		},
		{
			name: "int body",
			set:  func(lr plog.LogRecord) { lr.Body().SetInt(42) },
			want: `"body":42`,
		},
		{
			name: "double body",
			set:  func(lr plog.LogRecord) { lr.Body().SetDouble(1.5) },
			want: `"body":1.5`,
		},
		{
			name: "bool body",
			set:  func(lr plog.LogRecord) { lr.Body().SetBool(true) },
			want: `"body":true`,
		},
		{
			name: "bytes body",
			set:  func(lr plog.LogRecord) { lr.Body().SetEmptyBytes().Append(0xDE, 0xAD) },
			want: `"body":"3q0="`,
		},
		{
			name: "slice body",
			set:  func(lr plog.LogRecord) { lr.Body().SetEmptySlice().AppendEmpty().SetStr("x") },
			want: `"body":["x"]`,
		},
		{
			name: "map body",
			set:  func(lr plog.LogRecord) { lr.Body().SetEmptyMap().PutStr("action", "delete-all") },
			want: `"body":{"action":"delete-all"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := serializeBody(t, tc.set)
			if got == empty {
				t.Fatalf("body is absent from the signed payload: got %s, same as a record with no body", got)
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("payload does not carry the body:\n  got  %s\n  want it to contain %s", got, tc.want)
			}
		})
	}
}

// TestBodyDistinguishesDistinctRecords is the property that matters: two records
// differing only in their body must not share a signature.
func TestBodyDistinguishesDistinctRecords(t *testing.T) {
	a := serializeBody(t, func(lr plog.LogRecord) { lr.Body().SetEmptyMap().PutStr("action", "read") })
	b := serializeBody(t, func(lr plog.LogRecord) { lr.Body().SetEmptyMap().PutStr("action", "delete-all") })
	if a == b {
		t.Errorf("records with different map bodies canonicalize identically: %s", a)
	}
}

// TestEmptyBodyIsOmitted confirms an unset body stays out of the payload rather
// than being encoded as a null, so this change does not alter records without one.
func TestEmptyBodyIsOmitted(t *testing.T) {
	p := &signingProcessor{config: &Config{}}
	lr := plog.NewLogRecord()
	lr.SetEventName("e")
	if lr.Body().Type() != pcommon.ValueTypeEmpty {
		t.Fatalf("expected an unset body to be ValueTypeEmpty, got %v", lr.Body().Type())
	}
	b, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serializeLogRecord: %v", err)
	}
	if strings.Contains(string(b), `"body"`) {
		t.Errorf("unset body should be omitted, got %s", b)
	}
}

// TestBodyInvalidUTF8IsRejected confirms the UTF-8 validation that guarded the
// string-only path still applies now that the body goes through valueToInterface.
func TestBodyInvalidUTF8IsRejected(t *testing.T) {
	p := &signingProcessor{config: &Config{}}
	lr := plog.NewLogRecord()
	lr.Body().SetStr(string([]byte{0xff, 0xfe}))
	_, err := p.serializeLogRecord(lr)
	if err == nil {
		t.Fatal("expected an error for a body containing invalid UTF-8")
	}
	if !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("expected a UTF-8 error, got %v", err)
	}
}

// TestBodyNestingDepthIsCapped pins the bound the body inherits from
// valueToInterface. Without it a deeply nested body would reach encoding/json
// and overflow the stack, which is a fatal error the collector cannot recover
// from. The entry depth is the assertion a refactor is most likely to get wrong.
func TestBodyNestingDepthIsCapped(t *testing.T) {
	p := &signingProcessor{config: &Config{}}
	lr := plog.NewLogRecord()
	m := lr.Body().SetEmptyMap()
	for range jsonMaxDepth + 1 {
		m = m.PutEmptyMap("n")
	}
	_, err := p.serializeLogRecord(lr)
	if err == nil {
		t.Fatal("expected a depth-limit error for a deeply nested body")
	}
	if !strings.Contains(err.Error(), "nesting depth limit") {
		t.Errorf("expected a nesting depth error, got %v", err)
	}
}
