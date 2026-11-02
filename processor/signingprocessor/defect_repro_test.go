// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Reproductions and regression tests for canonicalization defects in the JCS
// signing processor, verified against PR #50548 head f61bc9bd05.
//
// Each test is self-contained and drops into the package's own internal test
// package (package signingprocessor), so it can reach the unexported
// marshalJCS / valueToInterface / signingProcessor symbols directly.
package signingprocessor

import (
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// TestInvalidUTF8Rejected verifies that serializeLogRecord rejects log records
// whose body or attribute strings contain invalid UTF-8, closing the
// processor-reachable collision path where json.Marshal would otherwise coerce
// distinct byte sequences to the same U+FFFD replacement character.
func TestInvalidUTF8Rejected(t *testing.T) {
	p := &signingProcessor{}

	invalidBody := string([]byte{0xed, 0xa0, 0x80}) // encoded high surrogate, invalid UTF-8

	t.Run("body", func(t *testing.T) {
		lr := plog.NewLogRecord()
		lr.Body().SetStr(invalidBody)
		_, err := p.serializeLogRecord(lr)
		if err == nil {
			t.Fatal("expected error for invalid-UTF-8 body, got nil")
		}
		if !strings.Contains(err.Error(), "invalid UTF-8") {
			t.Fatalf("expected 'invalid UTF-8' error, got: %v", err)
		}
	})

	t.Run("attribute", func(t *testing.T) {
		lr := plog.NewLogRecord()
		lr.Attributes().PutStr("key", invalidBody)
		_, err := p.serializeLogRecord(lr)
		if err == nil {
			t.Fatal("expected error for invalid-UTF-8 attribute, got nil")
		}
		if !strings.Contains(err.Error(), "invalid UTF-8") {
			t.Fatalf("expected 'invalid UTF-8' error, got: %v", err)
		}
	})

	t.Run("nested_attribute", func(t *testing.T) {
		v := pcommon.NewValueEmpty()
		m := v.SetEmptyMap()
		m.PutStr("inner", invalidBody)
		_, err := p.valueToInterface(v, 0)
		if err == nil {
			t.Fatal("expected error for invalid-UTF-8 nested attribute, got nil")
		}
		if !strings.Contains(err.Error(), "invalid UTF-8") {
			t.Fatalf("expected 'invalid UTF-8' error, got: %v", err)
		}
	})
}
