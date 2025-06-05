// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Our types are bool, int, float, string, Bytes, nil, so we compare all types in both directions.
var (
	ta   = false
	tb   = true
	sa   = "1"
	sb   = "2"
	sn   = ""
	ba   = []byte("1")
	bb   = []byte("2")
	bn   []byte
	i64a = int64(1)
	i64b = int64(2)
	f64a = float64(1)
	f64b = float64(2)

	m1 = map[string]any{
		"test": true,
	}
	m2 = map[string]any{
		"test": false,
	}

	sl1 = []any{"value1"}
	sl2 = []any{"value2"}
)

type testA struct {
	Value string
}

type testB struct {
	Value string
}

// This is one giant table-driven test that is designed to check almost all of the possibilities for
// comparing two different kinds of items. Each test compares two objects of any type using all
// of the comparison operators, and has a hand-constructed list of expected values.
// The test is not *quite* exhaustive, but it does attempt to check every basic type against
// every other basic type, and includes a pretty good set of tests on the pointers to all the
// basic types as well.
func Test_comparison(t *testing.T) {
	pm1 := pcommon.NewMap()
	pm1.PutBool("test", true)

	pm2 := pcommon.NewMap()
	pm2.PutBool("test", false)

	psl1 := pcommon.NewSlice()
	psl1.AppendEmpty().SetStr("value1")

	psl2 := pcommon.NewSlice()
	psl2.AppendEmpty().SetStr("value2")

	tests := []struct {
		name string
		a    any
		b    any
		want []bool // in order of eq, ne, lt, lte, gte, gt.
	}{
		{"identity string", sa, sa, []bool{true, false, false, true, true, false}},
		{"identity int64", i64a, i64a, []bool{true, false, false, true, true, false}},
		{"identity float64", f64a, f64a, []bool{true, false, false, true, true, false}},
		{"identity bytes", ba, ba, []bool{true, false, false, true, true, false}},
		{"identity nil", nil, nil, []bool{true, false, false, true, true, false}},

		{"diff strings", sa, sb, []bool{false, true, true, true, false, false}},
		{"string bytes", sa, ba, []bool{false, true, false, false, false, false}},
		{"string int64", sa, i64a, []bool{false, true, false, false, false, false}},
		{"string float64", sa, f64a, []bool{false, true, false, false, false, false}},
		{"string nil", sa, nil, []bool{false, true, false, false, false, false}},

		{"diff bytes", ba, bb, []bool{false, true, true, true, false, false}},
		{"bytes string", ba, sa, []bool{false, true, false, false, false, false}},
		{"bytes int64", ba, i64a, []bool{false, true, false, false, false, false}},
		{"bytes float64", ba, f64a, []bool{false, true, false, false, false, false}},
		{"bytes nil", ba, nil, []bool{false, true, false, false, false, false}},
		{"bytes nilbytes", ba, bn, []bool{false, true, false, false, false, false}},

		{"false true", ta, tb, []bool{false, true, true, true, false, false}},
		{"true false", tb, ta, []bool{false, true, false, false, true, true}},
		{"true true", ta, ta, []bool{true, false, false, true, true, false}},
		{"false false", ta, ta, []bool{true, false, false, true, true, false}},

		{"bool string", ta, sa, []bool{false, true, false, false, false, false}},
		{"bool int64", ta, i64a, []bool{false, true, false, false, false, false}},
		{"bool float64", ta, f64a, []bool{false, true, false, false, false, false}},
		{"bool and nil bytes", ta, bn, []bool{false, true, false, false, false, false}},
		{"bool and empty string", ta, sn, []bool{false, true, false, false, false, false}},

		{"nil string", nil, sa, []bool{false, true, false, false, false, false}},
		{"nil int64", nil, i64a, []bool{false, true, false, false, false, false}},
		{"nil float64", nil, f64a, []bool{false, true, false, false, false, false}},
		{"nil and nil bytes", nil, bn, []bool{true, false, false, true, true, false}},
		{"nil and empty string", nil, sn, []bool{false, true, false, false, false, false}},

		{"int64 string", i64a, sa, []bool{false, true, false, false, false, false}},
		{"int64 bytes", i64a, ba, []bool{false, true, false, false, false, false}},
		{"int64 nil", i64a, nil, []bool{false, true, false, false, false, false}},
		{"int64 int64", i64a, i64b, []bool{false, true, true, true, false, false}},
		{"int64 float64", i64a, f64b, []bool{false, true, true, true, false, false}},

		{"diff float64s", f64a, f64b, []bool{false, true, true, true, false, false}},
		{"float64 string", f64a, sa, []bool{false, true, false, false, false, false}},
		{"float64 bytes", f64a, ba, []bool{false, true, false, false, false, false}},
		{"float64 nil", f64a, nil, []bool{false, true, false, false, false, false}},
		{"float64 int64", f64a, i64b, []bool{false, true, true, true, false, false}},

		{"non-prim, same type, equal", testA{"hi"}, testA{"hi"}, []bool{true, false, false, false, false, false}},
		{"non-prim, same type, not equal", testA{"hi"}, testA{"byte"}, []bool{false, true, false, false, false, false}},
		{"non-prim, diff type", testA{"hi"}, testB{"hi"}, []bool{false, true, false, false, false, false}},
		{"non-prim, int type", testA{"hi"}, 5, []bool{false, true, false, false, false, false}},
		{"int, non-prim", 5, testA{"hi"}, []bool{false, true, false, false, false, false}},

		{"maps diff", m1, m2, []bool{false, true, false, false, false, false}},
		{"maps same", m1, m1, []bool{true, false, false, false, false, false}},
		{"pmaps diff", pm1, pm2, []bool{false, true, false, false, false, false}},
		{"pmaps same", pm1, pm1, []bool{true, false, false, false, false, false}},
		{"mixed map diff pmap", m1, pm2, []bool{false, true, false, false, false, false}},
		{"mixed pmap diff map", pm2, m1, []bool{false, true, false, false, false, false}},
		{"mixed map same pmap", m1, pm1, []bool{true, false, false, false, false, false}},
		{"mixed pmap same map", pm1, m1, []bool{true, false, false, false, false, false}},
		{"map and other type", m1, sa, []bool{false, true, false, false, false, false}},
		{"pmap and other type", pm1, sa, []bool{false, true, false, false, false, false}},

		{"slice diff", sl1, sl2, []bool{false, true, false, false, false, false}},
		{"slice same", sl1, sl1, []bool{true, false, false, false, false, false}},
		{"pslice diff", psl1, psl2, []bool{false, true, false, false, false, false}},
		{"pslice same", psl1, psl1, []bool{true, false, false, false, false, false}},
		{"mixed slice diff pslice", sl1, psl2, []bool{false, true, false, false, false, false}},
		{"mixed pslice diff slice", psl2, sl1, []bool{false, true, false, false, false, false}},
		{"mixed slice same pslice", sl1, psl1, []bool{true, false, false, false, false, false}},
		{"mixed pslice same slice", psl1, sl1, []bool{true, false, false, false, false, false}},
		{"slice and other type", sl1, sa, []bool{false, true, false, false, false, false}},
		{"pslice and other type", psl1, sa, []bool{false, true, false, false, false, false}},
	}
	ops := []compareOp{eq, ne, lt, lte, gte, gt}
	comp := NewValueComparator()
	for _, tt := range tests {
		for _, op := range ops {
			t.Run(fmt.Sprintf("%s %v", tt.name, op), func(t *testing.T) {
				if got := comp.compare(tt.a, tt.b, op); got != tt.want[op] {
					t.Errorf("compare(%v, %v, %v) = %v, want %v", tt.a, tt.b, op, got, tt.want[op])
				}
				var got bool
				var funcName string
				switch op {
				case eq:
					funcName = "Equal"
					got = comp.Equal(tt.a, tt.b)
				case ne:
					funcName = "NotEqual"
					got = comp.NotEqual(tt.a, tt.b)
				case gt:
					funcName = "Greater"
					got = comp.Greater(tt.a, tt.b)
				case gte:
					funcName = "GreaterEqual"
					got = comp.GreaterEqual(tt.a, tt.b)
				case lt:
					funcName = "Less"
					got = comp.Less(tt.a, tt.b)
				case lte:
					funcName = "LessEqual"
					got = comp.LessEqual(tt.a, tt.b)
				default:
					t.Fatalf("unknown compare operator: %v", op)
				}
				if got != tt.want[op] {
					t.Errorf("%s(%v, %v) = %v, want %v, op %v", funcName, tt.a, tt.b, got, tt.want[op], op)
				}
			})
		}
	}
}

// Benchmarks -- these benchmarks compare the performance of comparisons of a variety of data types.
// It's not attempting to be exhaustive, but again, it hits most of the major types and combinations.
// The summary is that they're pretty fast; all the calls to compare are 12 ns/op or less on a 2019 intel
// mac pro laptop, and none of them have any allocations.
func BenchmarkCompareEQInt64(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(i64a, i64b, eq)
	}
}

func BenchmarkCompareEQFloat(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(f64a, f64b, eq)
	}
}

func BenchmarkCompareEQString(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(sa, sb, eq)
	}
}

func BenchmarkCompareEQPString(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(&sa, &sb, eq)
	}
}

func BenchmarkCompareEQBytes(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(ba, bb, eq)
	}
}

func BenchmarkCompareEQNil(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(nil, nil, eq)
	}
}

func BenchmarkCompareNEInt(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(i64a, i64b, ne)
	}
}

func BenchmarkCompareNEFloat(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(f64a, f64b, ne)
	}
}

func BenchmarkCompareNEString(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(sa, sb, ne)
	}
}

func BenchmarkCompareLTFloat(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(f64a, f64b, lt)
	}
}

func BenchmarkCompareLTString(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(sa, sb, lt)
	}
}

func BenchmarkCompareLTNil(b *testing.B) {
	c := NewValueComparator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compare(nil, nil, lt)
	}
}

// this is only used for benchmarking, and is a rough equivalent of the original compare function
// before adding lt, lte, gte, and gt.
func compareEq(a any, b any, op compareOp) bool {
	switch op {
	case eq:
		return a == b
	case ne:
		return a != b
	default:
		return false
	}
}

func BenchmarkCompareEQFunction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compareEq(sa, sb, eq)
	}
}
