package tql

import (
	"fmt"
	"testing"
)

// Our types are bool, int, float, string, Bytes, nil, so we compare all types in both directions.
// We touch every pointer type but don't test all the cases.
var (
	ta             = false
	tb             = true
	sa             = "1"
	sb             = "2"
	sn             = ""
	ba             = []byte("1")
	bb             = []byte("2")
	bn    []byte   = nil
	ia             = int(1)
	i32a           = int32(1)
	i64a           = int64(1)
	ib             = int(2)
	i32b           = int32(2)
	i64b           = int64(2)
	f32a           = float32(1)
	f64a           = float64(1)
	f32b           = float32(2)
	f64b           = float64(2)
	nilpf *float64 = nil
	nilpi *int64   = nil
	nilps *string  = nil
	nilpt *bool    = nil
)

// This is one giant table-driven test that is designed to check almost all of the possibilities for
// comparing two different kinds of items. Each test compares two objects of any type using all
// of the comparison operators, and has a hand-constructed list of expected values.
// The test is not *quite* exhaustive, but it does attempt to check every basic type against
// every other basic type, and includes a pretty good set of tests on the pointers to all the
// basic types as well.
func Test_compare(t *testing.T) {
	tests := []struct {
		name string
		a    any
		b    any
		want []bool // in order of EQ, NE, LT, LTE, GTE, GT.
	}{
		{"identity string", sa, sa, []bool{true, false, false, true, true, false}},
		{"identity int", ia, ia, []bool{true, false, false, true, true, false}},
		{"identity int32", i32a, i32a, []bool{true, false, false, true, true, false}},
		{"identity int64", i64a, i64a, []bool{true, false, false, true, true, false}},
		{"identity float32", f32a, f32a, []bool{true, false, false, true, true, false}},
		{"identity float64", f64a, f64a, []bool{true, false, false, true, true, false}},
		{"identity bytes", ba, ba, []bool{true, false, false, true, true, false}},
		{"identity nil", nil, nil, []bool{true, false, false, true, true, false}},

		{"identity string *string", sa, &sa, []bool{true, false, false, true, true, false}},
		{"identity int *int", ia, &ia, []bool{true, false, false, true, true, false}},
		{"identity int32 *int32", i32a, &i32a, []bool{true, false, false, true, true, false}},
		{"identity int64 *int64", i64a, &i64a, []bool{true, false, false, true, true, false}},
		{"identity float32 *float32", f32a, &f32a, []bool{true, false, false, true, true, false}},
		{"identity float64 *float64", f64a, &f64a, []bool{true, false, false, true, true, false}},

		{"identity *string string", &sa, sa, []bool{true, false, false, true, true, false}},
		{"identity *int int", &ia, ia, []bool{true, false, false, true, true, false}},
		{"identity *int32 int32", &i32a, i32a, []bool{true, false, false, true, true, false}},
		{"identity *int64 int64", &i64a, i64a, []bool{true, false, false, true, true, false}},
		{"identity *float32 float32", &f32a, f32a, []bool{true, false, false, true, true, false}},
		{"identity *float64 float64", &f64a, f64a, []bool{true, false, false, true, true, false}},

		{"diff strings", sa, sb, []bool{false, true, true, true, false, false}},
		{"string bytes", sa, ba, []bool{false, true, false, false, false, false}},
		{"string int", sa, ia, []bool{false, true, false, false, false, false}},
		{"string int32", sa, i32a, []bool{false, true, false, false, false, false}},
		{"string int64", sa, i64a, []bool{false, true, false, false, false, false}},
		{"string float32", sa, f32a, []bool{false, true, false, false, false, false}},
		{"string float64", sa, f64a, []bool{false, true, false, false, false, false}},
		{"string nil", sa, nil, []bool{false, true, false, false, false, false}},
		{"string nilps", sa, nilps, []bool{false, true, false, false, false, false}},
		{"nilps string", nilps, sa, []bool{false, true, false, false, false, false}},

		{"diff bytes", ba, bb, []bool{false, true, true, true, false, false}},
		{"bytes string", ba, sa, []bool{false, true, false, false, false, false}},
		{"bytes int", ba, ia, []bool{false, true, false, false, false, false}},
		{"bytes int32", ba, i32a, []bool{false, true, false, false, false, false}},
		{"bytes int64", ba, i64a, []bool{false, true, false, false, false, false}},
		{"bytes float32", ba, f32a, []bool{false, true, false, false, false, false}},
		{"bytes float64", ba, f64a, []bool{false, true, false, false, false, false}},
		{"bytes nil", ba, nil, []bool{false, true, false, false, false, false}},
		{"bytes nilbytes", ba, bn, []bool{false, true, false, false, false, false}},

		{"false true", ta, tb, []bool{false, true, true, true, false, false}},
		{"true false", tb, ta, []bool{false, true, false, false, true, true}},
		{"true true", ta, ta, []bool{true, false, false, true, true, false}},
		{"false false", ta, ta, []bool{true, false, false, true, true, false}},
		{"true *true", ta, &ta, []bool{true, false, false, true, true, false}},

		{"bool string", ta, sa, []bool{false, true, false, false, false, false}},
		{"bool int", ta, ia, []bool{false, true, false, false, false, false}},
		{"bool int32", ta, i32a, []bool{false, true, false, false, false, false}},
		{"bool int64", ta, i64a, []bool{false, true, false, false, false, false}},
		{"bool float32", ta, f32a, []bool{false, true, false, false, false, false}},
		{"bool float64", ta, f64a, []bool{false, true, false, false, false, false}},
		{"bool and nil bytes", ta, bn, []bool{false, true, false, false, false, false}},
		{"bool and empty string", ta, sn, []bool{false, true, false, false, false, false}},
		{"bool nilpt", sa, nilpt, []bool{false, true, false, false, false, false}},
		{"nilpt bool", nilpt, sa, []bool{false, true, false, false, false, false}},

		{"nil string", nil, sa, []bool{false, true, false, false, false, false}},
		{"nil int", nil, ia, []bool{false, true, false, false, false, false}},
		{"nil int32", nil, i32a, []bool{false, true, false, false, false, false}},
		{"nil int64", nil, i64a, []bool{false, true, false, false, false, false}},
		{"nil float32", nil, f32a, []bool{false, true, false, false, false, false}},
		{"nil float64", nil, f64a, []bool{false, true, false, false, false, false}},
		{"nil and nil bytes", nil, bn, []bool{true, false, false, true, true, false}},
		{"nil and empty string", nil, sn, []bool{false, true, false, false, false, false}},
		{"nil and nilpf", nil, nilpf, []bool{true, false, false, true, true, false}},
		{"nil and nilpi", nil, nilpi, []bool{true, false, false, true, true, false}},
		{"nilpt and nil", nil, nilpi, []bool{true, false, false, true, true, false}},

		{"diff ints", ia, ib, []bool{false, true, true, true, false, false}},
		{"int string", ia, sa, []bool{false, true, false, false, false, false}},
		{"int bytes", ia, ba, []bool{false, true, false, false, false, false}},
		{"int nil", ia, nil, []bool{false, true, false, false, false, false}},
		{"int int32", ia, i32b, []bool{false, true, true, true, false, false}},
		{"int int64", ia, i64b, []bool{false, true, true, true, false, false}},
		{"int float32", ia, f32b, []bool{false, true, true, true, false, false}},
		{"int float64", ia, f64b, []bool{false, true, true, true, false, false}},

		{"diff int32s", i32a, ib, []bool{false, true, true, true, false, false}},
		{"int32 string", i32a, sa, []bool{false, true, false, false, false, false}},
		{"int32 bytes", i32a, ba, []bool{false, true, false, false, false, false}},
		{"int32 nil", i32a, nil, []bool{false, true, false, false, false, false}},
		{"int32 int", i32a, ib, []bool{false, true, true, true, false, false}},
		{"int32 int64", i32a, i64b, []bool{false, true, true, true, false, false}},
		{"int32 float32", i32a, f32b, []bool{false, true, true, true, false, false}},
		{"int32 float64", i32a, f64b, []bool{false, true, true, true, false, false}},

		{"diff int64s", i64a, ib, []bool{false, true, true, true, false, false}},
		{"int64 string", i64a, sa, []bool{false, true, false, false, false, false}},
		{"int64 bytes", i64a, ba, []bool{false, true, false, false, false, false}},
		{"int64 nil", i64a, nil, []bool{false, true, false, false, false, false}},
		{"int64 int64", i64a, i64b, []bool{false, true, true, true, false, false}},
		{"int64 int", i64a, ib, []bool{false, true, true, true, false, false}},
		{"int64 float32", i64a, f32b, []bool{false, true, true, true, false, false}},
		{"int64 float64", i64a, f64b, []bool{false, true, true, true, false, false}},
		{"int64 nilint", nilpi, f64b, []bool{false, true, false, false, false, false}},
		{"nilint int64", f64b, nilpi, []bool{false, true, false, false, false, false}},

		{"diff float32s", f32a, f32b, []bool{false, true, true, true, false, false}},
		{"float32 string", f32a, sa, []bool{false, true, false, false, false, false}},
		{"float32 bytes", f32a, ba, []bool{false, true, false, false, false, false}},
		{"float32 nil", f32a, nil, []bool{false, true, false, false, false, false}},
		{"float32 int32", f32a, i32b, []bool{false, true, true, true, false, false}},
		{"float32 int64", f32a, i64b, []bool{false, true, true, true, false, false}},
		{"float32 float64", f32a, f64b, []bool{false, true, true, true, false, false}},

		{"diff float64s", f64a, f64b, []bool{false, true, true, true, false, false}},
		{"float64 string", f64a, sa, []bool{false, true, false, false, false, false}},
		{"float64 bytes", f64a, ba, []bool{false, true, false, false, false, false}},
		{"float64 nil", f64a, nil, []bool{false, true, false, false, false, false}},
		{"float64 int32", f64a, i32b, []bool{false, true, true, true, false, false}},
		{"float64 int64", f64a, i64b, []bool{false, true, true, true, false, false}},
		{"float64 float32", f64a, f32b, []bool{false, true, true, true, false, false}},
		{"float64 nilfloat", nilpf, f64b, []bool{false, true, false, false, false, false}},
		{"nilfloat float64", f64b, nilpf, []bool{false, true, false, false, false, false}},
	}
	ops := []compareOp{EQ, NE, LT, LTE, GTE, GT}
	for _, tt := range tests {
		for _, op := range ops {
			t.Run(fmt.Sprintf("%s %v", tt.name, op), func(t *testing.T) {
				if got := compare(tt.a, tt.b, op); got != tt.want[op] {
					t.Errorf("compare(%v, %v, %v) = %v, want %v", tt.a, tt.b, op, got, tt.want[op])
				}
			})
		}
	}
}

// Benchmarks -- these benchmarks compare the performance of comparisons of a variety of data types.
// It's not attempting to be exhaustive, but again, it hits most of the major types and combinations.
// The summary is that they're pretty fast; all the calls to compare are 10 ns/op or less on a 2019 intel
// mac pro laptop, and none of them have any allocations.
func BenchmarkCompareEQInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(ia, i64b, EQ)
	}
}

func BenchmarkCompareEQFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(f64a, f32b, EQ)
	}
}
func BenchmarkCompareEQString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(sa, sb, EQ)
	}
}
func BenchmarkCompareEQPString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(&sa, &sb, EQ)
	}
}
func BenchmarkCompareEQBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(ba, bb, EQ)
	}
}
func BenchmarkCompareEQNil(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(nil, nil, EQ)
	}
}

func BenchmarkCompareNEInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(i32a, i64b, NE)
	}
}

func BenchmarkCompareNEFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(f64a, f32b, NE)
	}
}
func BenchmarkCompareNEPFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(&f64a, &f32b, NE)
	}
}
func BenchmarkCompareNEString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(sa, sb, NE)
	}
}
func BenchmarkCompareNENil(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(nil, i64a, NE)
	}
}

func BenchmarkCompareLTInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(ia, ib, LT)
	}
}

func BenchmarkCompareLTPInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(&ia, &ib, LT)
	}
}

func BenchmarkCompareLTFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(f64a, f64b, LT)
	}
}
func BenchmarkCompareLTString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(sa, sb, LT)
	}
}
func BenchmarkCompareLTNil(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(nil, nil, LT)
	}
}

func BenchmarkCompareLTDiff(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compare(sa, f64b, LT)
	}
}

// this is only used for benchmarking, and is a rough equivalent of the original compare function
// before adding LT, LTE, GTE, and GT.
func compareEq(a any, b any, op compareOp) bool {
	switch op {
	case EQ:
		return a == b
	case NE:
		return a != b
	default:
		return false
	}
}

func BenchmarkCompareEQFunction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compareEq(sa, sb, EQ)
	}
}
