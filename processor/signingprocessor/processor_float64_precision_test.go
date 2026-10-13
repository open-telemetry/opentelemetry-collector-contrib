// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

// Float64 precision audit for the signing processor.
//
// JCS (RFC 8785) serializes float64 via strconv.FormatFloat(..., -1, 64) which
// produces the shortest decimal that round-trips (Ryu algorithm). This is a
// bijection over the finite, non-zero IEEE-754 doubles: two distinct values
// always produce distinct strings.
//
// The only real exception: -0.0 and +0.0 are both serialized as "0" per the
// JCS spec (ES6 §7.1.12.1). TestFloat64NegativeZeroCollision proves this.

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
)

// TestFloat64AdjacentValuesDistinct verifies that adjacent IEEE-754 doubles
// (differing by 1 ULP) produce distinct canonical bytes. Because JCS uses the
// shortest round-trip representation, every finite non-zero double maps to a
// unique string — there is no float64 analog of the int64->float64 precision
// loss that affects large integers.
func TestFloat64AdjacentValuesDistinct(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	cases := []struct {
		name string
		a, b float64
	}{
		{
			name: "small positive: 1.0 vs next ULP",
			a:    1.0,
			b:    math.Nextafter(1.0, math.MaxFloat64),
		},
		{
			name: "large positive near float64 max",
			a:    1e300,
			b:    math.Nextafter(1e300, math.MaxFloat64),
		},
		{
			name: "small negative: -1.0 vs next ULP toward zero",
			a:    -1.0,
			b:    math.Nextafter(-1.0, 0),
		},
		{
			name: "values near int64 max (1.7e18 range where int64 would collide)",
			a:    float64(int64(1 << 62)),
			b:    math.Nextafter(float64(int64(1<<62)), math.MaxFloat64),
		},
		{
			name: "subnormal vs next normal",
			a:    math.SmallestNonzeroFloat64,
			b:    math.Nextafter(math.SmallestNonzeroFloat64, math.MaxFloat64),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rA := plog.NewLogRecord()
			rA.Attributes().PutDouble("v", tc.a)

			rB := plog.NewLogRecord()
			rB.Attributes().PutDouble("v", tc.b)

			bA, err := p.serializeLogRecord(rA)
			if err != nil {
				t.Fatalf("serialize A: %v", err)
			}
			bB, err := p.serializeLogRecord(rB)
			if err != nil {
				t.Fatalf("serialize B: %v", err)
			}

			if bytes.Equal(bA, bB) {
				t.Errorf("adjacent float64 values produced identical canonical bytes: a=%v b=%v bytes=%s", tc.a, tc.b, bA)
			}
		})
	}
}

// TestFloat64NegativeZeroCollision documents and confirms that -0.0 and +0.0
// produce identical canonical bytes. This is mandated by the JCS spec (ES6
// §7.1.12.1) and is the ONLY float64 precision issue that exists. It is not a
// bug in our implementation; it is a deliberate spec requirement.
//
// Consequence: an attribute changed from +0.0 to -0.0 (or vice versa) will NOT
// change the signature. If distinguishing zero signs matters,
// store the value as a string attribute instead.
func TestFloat64NegativeZeroCollision(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	rPos := plog.NewLogRecord()
	rPos.Attributes().PutDouble("v", +0.0)

	rNeg := plog.NewLogRecord()
	rNeg.Attributes().PutDouble("v", math.Copysign(0, -1)) // -0.0

	bPos, err := p.serializeLogRecord(rPos)
	if err != nil {
		t.Fatalf("serialize +0.0: %v", err)
	}
	bNeg, err := p.serializeLogRecord(rNeg)
	if err != nil {
		t.Fatalf("serialize -0.0: %v", err)
	}

	// This collision is expected and spec-mandated. The test documents it.
	if !bytes.Equal(bPos, bNeg) {
		t.Errorf("unexpected: +0.0 and -0.0 produced distinct bytes — JCS spec requires they collapse to '0'")
	}
	if !strings.Contains(string(bPos), `"doubleValue":0`) {
		t.Errorf("expected +0.0 to serialize as 0, got: %s", bPos)
	}
}

// TestFloat64SpecialValuesRejected verifies that NaN and ±Infinity attributes
// are rejected at serialization time (JCS forbids them as invalid JSON numbers).
func TestFloat64SpecialValuesRejected(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	specials := []struct {
		name string
		v    float64
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
	}

	for _, tc := range specials {
		t.Run(tc.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			lr.Attributes().PutDouble("v", tc.v)

			_, err := p.serializeLogRecord(lr)
			if err == nil {
				t.Errorf("expected error for %s attribute, got nil", tc.name)
			}
		})
	}
}

// TestFloat64SerializationFormat verifies the literal form of some well-known
// doubles in canonical output, matching RFC 8785 §3.2.2.3 test vectors.
func TestFloat64SerializationFormat(t *testing.T) {
	p := &signingProcessor{config: &Config{}}

	cases := []struct {
		name        string
		v           float64
		wantContain string
	}{
		{"1.0 serializes without trailing .0 if integer", 1.0, `"doubleValue":1`},
		{"1.5 keeps decimal", 1.5, `"doubleValue":1.5`},
		{"large exponent uses e notation", 1e21, `"doubleValue":1e+21`},
		{"small value below 1e-6 uses e notation", 5e-7, `"doubleValue":5e-7`},
		{"negative value", -2.5, `"doubleValue":-2.5`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			lr.Attributes().PutDouble("v", tc.v)

			b, err := p.serializeLogRecord(lr)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			if !strings.Contains(string(b), tc.wantContain) {
				t.Errorf("canonical output does not contain %q; got: %s", tc.wantContain, b)
			}
		})
	}
}
