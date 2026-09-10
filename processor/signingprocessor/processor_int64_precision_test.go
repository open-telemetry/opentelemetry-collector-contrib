// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// TestTimestampPrecision verifies that two records whose timestamps differ by
// less than 256 nanoseconds produce distinct canonical bytes. With naive
// int64-to-float64 conversion (IEEE-754 double) values in the ~1.7e18 range
// lose the low bits and can round to the same float, making distinct records
// produce identical signatures.
func TestTimestampPrecision(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	base := pcommon.Timestamp(1714041600000000000)

	r1 := plog.NewLogRecord()
	r1.SetTimestamp(base)

	r2 := plog.NewLogRecord()
	r2.SetTimestamp(base + 1) // 1 nanosecond later

	b1, err := p.serializeLogRecord(r1)
	if err != nil {
		t.Fatalf("serialize r1: %v", err)
	}
	b2, err := p.serializeLogRecord(r2)
	if err != nil {
		t.Fatalf("serialize r2: %v", err)
	}

	if bytes.Equal(b1, b2) {
		t.Errorf("timestamps differing by 1ns produced identical canonical bytes: %s", b1)
	}
}

// TestObservedTimestampPrecision is the same check for ObservedTimestamp.
func TestObservedTimestampPrecision(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	base := pcommon.Timestamp(1714041600000000000)

	r1 := plog.NewLogRecord()
	r1.SetObservedTimestamp(base)

	r2 := plog.NewLogRecord()
	r2.SetObservedTimestamp(base + 1)

	b1, err := p.serializeLogRecord(r1)
	if err != nil {
		t.Fatalf("serialize r1: %v", err)
	}
	b2, err := p.serializeLogRecord(r2)
	if err != nil {
		t.Fatalf("serialize r2: %v", err)
	}

	if bytes.Equal(b1, b2) {
		t.Errorf("observed_timestamps differing by 1ns produced identical canonical bytes: %s", b1)
	}
}

// TestInt64AttributePrecision verifies that two int64 attribute values that
// differ by less than 64 (63 apart) produce distinct canonical bytes. In the
// range >2^53 consecutive int64s collapse to the same IEEE-754 double.
func TestInt64AttributePrecision(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	const base int64 = 9_007_199_254_740_992 // 2^53: first int64 where consecutive values collide in float64

	r1 := plog.NewLogRecord()
	r1.Attributes().PutInt("snowflake.id", base)

	r2 := plog.NewLogRecord()
	r2.Attributes().PutInt("snowflake.id", base+1)

	b1, err := p.serializeLogRecord(r1)
	if err != nil {
		t.Fatalf("serialize r1: %v", err)
	}
	b2, err := p.serializeLogRecord(r2)
	if err != nil {
		t.Fatalf("serialize r2: %v", err)
	}

	if bytes.Equal(b1, b2) {
		t.Errorf("int64 attrs differing by 1 beyond 2^53 produced identical canonical bytes: %s", b1)
	}
}

// TestMaxInt64SerializedAsString verifies that math.MaxInt64 appears in the
// canonical output as a quoted decimal string, not as a rounded float literal.
// The rounded float "9223372036854776000" is out-of-range for some JSON parsers
// and silently loses information.
func TestMaxInt64SerializedAsString(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	lr := plog.NewLogRecord()
	lr.Attributes().PutInt("big.value", math.MaxInt64)

	b, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	want := `"9223372036854775807"`
	if !strings.Contains(string(b), want) {
		t.Errorf("canonical output does not contain quoted MaxInt64 %s; got: %s", want, b)
	}
}

// TestTimestampSerializedAsString verifies that a nanosecond timestamp appears
// in the canonical output as a quoted decimal string.
func TestTimestampSerializedAsString(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	lr := plog.NewLogRecord()
	lr.SetTimestamp(pcommon.Timestamp(1714041600000000000))

	b, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	want := `"1714041600000000000"`
	if !strings.Contains(string(b), want) {
		t.Errorf("canonical output does not contain quoted timestamp %s; got: %s", want, b)
	}
}
