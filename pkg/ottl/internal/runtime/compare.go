// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package runtime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/runtime"

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/exp/constraints"
)

// CompareOp is the type of a comparison operator.
type CompareOp int

// These are the allowed values of a CompareOp
const (
	Eq CompareOp = iota
	Ne
	Lt
	Lte
	Gte
	Gt
)

// a fast way to get from a string to a CompareOp
var CompareOpTable = map[string]CompareOp{
	"==": Eq,
	"!=": Ne,
	"<":  Lt,
	"<=": Lte,
	">":  Gt,
	">=": Gte,
}

// Capture is how the parser converts an operator string to a CompareOp.
func (c *CompareOp) Capture(values []string) error {
	op, ok := CompareOpTable[values[0]]
	if !ok {
		return fmt.Errorf("'%s' is not a valid operator", values[0])
	}
	*c = op
	return nil
}

// String() for CompareOp gives us more legible test results and error messages.
func (c *CompareOp) String() string {
	switch *c {
	case Eq:
		return "Eq"
	case Ne:
		return "Ne"
	case Lt:
		return "Lt"
	case Lte:
		return "Lte"
	case Gte:
		return "Gte"
	case Gt:
		return "Gt"
	default:
		return "UNKNOWN OP!"
	}
}

// ottlValueComparator is the default implementation of the ValueComparator
type ottlValueComparator struct{}

// The functions in this file implement a general-purpose comparison of two
// values of type any, which for the purposes of OTTL mean values that are one of
// int, float, string, bool, or pointers to those, or []byte, or nil.

// invalidComparison returns false for everything except Ne (where it returns true to indicate that the
// objects were definitely not equivalent).
func (*ottlValueComparator) invalidComparison(op CompareOp) bool {
	return op == Ne
}

// comparePrimitives implements a generic comparison helper for all Ordered types (derived from Float, Int, or string).
// According to benchmarks, it's faster than explicit comparison functions for these types.
func comparePrimitives[T constraints.Ordered](a, b T, op CompareOp) bool {
	switch op {
	case Eq:
		return a == b
	case Ne:
		return a != b
	case Lt:
		return a < b
	case Lte:
		return a <= b
	case Gte:
		return a >= b
	case Gt:
		return a > b
	default:
		return false
	}
}

func (*ottlValueComparator) compareBools(a, b bool, op CompareOp) bool {
	switch op {
	case Eq:
		return a == b
	case Ne:
		return a != b
	case Lt:
		return !a && b
	case Lte:
		return !a || b
	case Gte:
		return a || !b
	case Gt:
		return a && !b
	default:
		return false
	}
}

func (*ottlValueComparator) compareBytes(a, b []byte, op CompareOp) bool {
	switch op {
	case Eq:
		return bytes.Equal(a, b)
	case Ne:
		return !bytes.Equal(a, b)
	case Lt:
		return bytes.Compare(a, b) < 0
	case Lte:
		return bytes.Compare(a, b) <= 0
	case Gte:
		return bytes.Compare(a, b) >= 0
	case Gt:
		return bytes.Compare(a, b) > 0
	default:
		return false
	}
}

func (p *ottlValueComparator) compareBool(a bool, b any, op CompareOp) bool {
	switch v := b.(type) {
	case bool:
		return p.compareBools(a, v, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareString(a string, b any, op CompareOp) bool {
	switch v := b.(type) {
	case string:
		return comparePrimitives(a, v, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareByte(a []byte, b any, op CompareOp) bool {
	switch v := b.(type) {
	case nil:
		return op == Ne
	case []byte:
		if v == nil {
			return op == Ne
		}
		return p.compareBytes(a, v, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareInt64(a int64, b any, op CompareOp) bool {
	switch v := b.(type) {
	case int64:
		return comparePrimitives(a, v, op)
	case float64:
		return comparePrimitives(float64(a), v, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareFloat64(a float64, b any, op CompareOp) bool {
	switch v := b.(type) {
	case int64:
		return comparePrimitives(a, float64(v), op)
	case float64:
		return comparePrimitives(a, v, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareDuration(a time.Duration, b any, op CompareOp) bool {
	switch v := b.(type) {
	case time.Duration:
		ansecs := a.Nanoseconds()
		vnsecs := v.Nanoseconds()
		return comparePrimitives(ansecs, vnsecs, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareTime(a time.Time, b any, op CompareOp) bool {
	switch v := b.(type) {
	case time.Time:
		switch op {
		case Eq:
			return a.Equal(v)
		case Ne:
			return !a.Equal(v)
		case Lt:
			return a.Before(v)
		case Lte:
			return a.Before(v) || a.Equal(v)
		case Gte:
			return a.After(v) || a.Equal(v)
		case Gt:
			return a.After(v)
		default:
			return p.invalidComparison(op)
		}
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareMap(a map[string]any, b any, op CompareOp) bool {
	switch v := b.(type) {
	case pcommon.Map:
		switch op {
		case Eq:
			return reflect.DeepEqual(a, v.AsRaw())
		case Ne:
			return !reflect.DeepEqual(a, v.AsRaw())
		default:
			return p.invalidComparison(op)
		}
	case map[string]any:
		switch op {
		case Eq:
			return reflect.DeepEqual(a, v)
		case Ne:
			return !reflect.DeepEqual(a, v)
		default:
			return p.invalidComparison(op)
		}
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) comparePMap(a pcommon.Map, b any, op CompareOp) bool {
	switch v := b.(type) {
	case pcommon.Map:
		switch op {
		case Eq:
			return a.Equal(v)
		case Ne:
			return !a.Equal(v)
		default:
			return p.invalidComparison(op)
		}
	case map[string]any:
		return p.compareMap(a.AsRaw(), v, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareSlice(a []any, b any, op CompareOp) bool {
	switch v := b.(type) {
	case pcommon.Slice:
		switch op {
		case Eq:
			return reflect.DeepEqual(a, v.AsRaw())
		case Ne:
			return !reflect.DeepEqual(a, v.AsRaw())
		default:
			return p.invalidComparison(op)
		}
	case []any:
		switch op {
		case Eq:
			return reflect.DeepEqual(a, v)
		case Ne:
			return !reflect.DeepEqual(a, v)
		default:
			return p.invalidComparison(op)
		}
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) comparePSlice(a pcommon.Slice, b any, op CompareOp) bool {
	switch v := b.(type) {
	case pcommon.Slice:
		switch op {
		case Eq:
			return a.Equal(v)
		case Ne:
			return !a.Equal(v)
		default:
			return p.invalidComparison(op)
		}
	case []any:
		return p.compareSlice(a.AsRaw(), v, op)
	default:
		return p.invalidComparison(op)
	}
}

// a and b are the return values from a Getter; we try to compare them
// according to the given operator.
func (p *ottlValueComparator) compare(a, b any, op CompareOp) bool {
	// nils are equal to each other and never equal to anything else,
	// so if they're both nil, report equality.
	if a == nil && b == nil {
		return op == Eq || op == Lte || op == Gte
	}
	// Anything else, we switch on the left side first.
	switch v := a.(type) {
	case nil:
		// If a was nil, it means b wasn't and inequalities don't apply,
		// so let's swap and give it the chance to get evaluated.
		return p.compare(b, nil, op)
	case bool:
		return p.compareBool(v, b, op)
	case int64:
		return p.compareInt64(v, b, op)
	case float64:
		return p.compareFloat64(v, b, op)
	case string:
		return p.compareString(v, b, op)
	case []byte:
		if v == nil {
			return p.compare(b, nil, op)
		}
		return p.compareByte(v, b, op)
	case time.Duration:
		return p.compareDuration(v, b, op)
	case time.Time:
		return p.compareTime(v, b, op)
	case map[string]any:
		return p.compareMap(v, b, op)
	case pcommon.Map:
		return p.comparePMap(v, b, op)
	case []any:
		return p.compareSlice(v, b, op)
	case pcommon.Slice:
		return p.comparePSlice(v, b, op)
	default:
		// If we don't know what type it is, we can't do inequalities yet. So we can fall back to the old behavior where we just
		// use Go's standard equality.
		switch op {
		case Eq:
			return a == b
		case Ne:
			return a != b
		default:
			return p.invalidComparison(op)
		}
	}
}

func (p *ottlValueComparator) Equal(a, b any) bool {
	return p.compare(a, b, Eq)
}

func (p *ottlValueComparator) NotEqual(a, b any) bool {
	return p.compare(a, b, Ne)
}

func (p *ottlValueComparator) Less(a, b any) bool {
	return p.compare(a, b, Lt)
}

func (p *ottlValueComparator) LessEqual(a, b any) bool {
	return p.compare(a, b, Lte)
}

func (p *ottlValueComparator) Greater(a, b any) bool {
	return p.compare(a, b, Gt)
}

func (p *ottlValueComparator) GreaterEqual(a, b any) bool {
	return p.compare(a, b, Gte)
}

// NewValueComparator creates a new ValueComparator instance that can be used to compare
// values using the OTTL comparison rules.
func NewValueComparator() *ottlValueComparator {
	return &ottlValueComparator{}
}
