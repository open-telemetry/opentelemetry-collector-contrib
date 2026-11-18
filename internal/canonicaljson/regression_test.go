// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package canonicaljson - Regression Tests for Canonicalization Bugs
//
// This file contains regression tests for specific bugs that were found and
// fixed in the canonical JSON serialization implementation.
//
// Each test documents:
// - The bug that occurred
// - How it was triggered  
// - The PR that fixed it

package canonicaljson

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

// TestInvalidUTF8Rejected_Body verifies that SerializeLogRecord rejects log records
// whose body contains invalid UTF-8, closing the processor-reachable collision path
// where json.Marshal would otherwise coerce distinct byte sequences to the same
// U+FFFD replacement character.
//
// Regression test for PR #50548.
func TestInvalidUTF8Rejected_Body(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr(string([]byte{0xed, 0xa0, 0x80})) // encoded high surrogate, invalid UTF-8

	opts := SerializeLogRecordOptions{}
	_, err := SerializeLogRecord(lr, opts)

	require.Error(t, err, "expected error for invalid-UTF-8 body")
	require.Contains(t, err.Error(), "invalid UTF-8")
}

// TestInvalidUTF8Rejected_Attribute verifies that SerializeLogRecord rejects log records
// whose attribute values contain invalid UTF-8, closing the processor-reachable collision
// path where json.Marshal would otherwise coerce distinct byte sequences to the same
// U+FFFD replacement character.
//
// Regression test for PR #50548.
func TestInvalidUTF8Rejected_Attribute(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("body")
	lr.Attributes().PutStr("key", string([]byte{0xed, 0xa0, 0x80})) // invalid UTF-8

	opts := SerializeLogRecordOptions{}
	_, err := SerializeLogRecord(lr, opts)

	require.Error(t, err, "expected error for invalid-UTF-8 attribute")
	require.Contains(t, err.Error(), "invalid UTF-8")
}

// TestInvalidUTF8Rejected_NestedAttribute verifies that SerializeLogRecord rejects log records
// whose nested map attribute values contain invalid UTF-8, closing the processor-reachable
// collision path where json.Marshal would otherwise coerce distinct byte sequences to the same
// U+FFFD replacement character.
//
// Regression test for PR #50548.
func TestInvalidUTF8Rejected_NestedAttribute(t *testing.T) {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("body")

	m := lr.Attributes().PutEmptyMap("nested")
	m.PutStr("inner", string([]byte{0xed, 0xa0, 0x80})) // invalid UTF-8

	opts := SerializeLogRecordOptions{}
	_, err := SerializeLogRecord(lr, opts)

	require.Error(t, err, "expected error for invalid-UTF-8 nested attribute")
	require.Contains(t, err.Error(), "invalid UTF-8")
}
