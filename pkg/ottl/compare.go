// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"bytes"
	"cmp"
	"reflect"
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// ValueComparator defines methods for comparing values using the OTTL comparison rules
// (https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/LANGUAGE.md#comparison-rules)
type ValueComparator interface {
	// Equal compares two values for equality, returning true if they are equals
	// according to the OTTL comparison rules.
	Equal(a, b any) bool
	// NotEqual compares two values for equality, returning true if they are different
	// according to the OTTL comparison rules.
	NotEqual(a, b any) bool
	// Less compares two values, returning true if the first value is less than the second
	// value, using the OTTL comparison rules.
	Less(a, b any) bool
	// LessEqual compares two values, returning true if the first value is less or equal
	// to the second value, using the OTTL comparison rules.
	LessEqual(a, b any) bool
	// Greater compares two values, returning true if the first value is greater than the
	// second value, using the OTTL comparison rules.
	Greater(a, b any) bool
	// GreaterEqual compares two values, returning true if the first value is greater or
	// equal to the second value, using the OTTL comparison rules.
	GreaterEqual(a, b any) bool
	// compare is a private method that compares two values using the grammar compareOp,
	// it also restricts custom implementations outside of this package.
	compare(a, b any, op compareOp) bool
}

// ottlValueComparator is the default implementation of the ValueComparator
type ottlValueComparator struct{}

// The functions in this file implement a general-purpose comparison of two
// values of type any, which for the purposes of OTTL mean values that are one of
// int, float, string, bool, or pointers to those, or []byte, or nil.

// invalidComparison returns false for everything except ne (where it returns true to indicate that the
// objects were definitely not equivalent).
func (*ottlValueComparator) invalidComparison(op compareOp) bool {
	return op == ne
}

// comparePrimitives implements a generic comparison helper for all Ordered types (derived from Float, Int, or string).
// According to benchmarks, it's faster than explicit comparison functions for these types.
func comparePrimitives[T cmp.Ordered](a, b T, op compareOp) bool {
	switch op {
	case eq:
		return a == b
	case ne:
		return a != b
	case lt:
		return a < b
	case lte:
		return a <= b
	case gte:
		return a >= b
	case gt:
		return a > b
	default:
		return false
	}
}

func (*ottlValueComparator) compareBools(a, b bool, op compareOp) bool {
	switch op {
	case eq:
		return a == b
	case ne:
		return a != b
	case lt:
		return !a && b
	case lte:
		return !a || b
	case gte:
		return a || !b
	case gt:
		return a && !b
	default:
		return false
	}
}

func (*ottlValueComparator) compareBytes(a, b []byte, op compareOp) bool {
	switch op {
	case eq:
		return bytes.Equal(a, b)
	case ne:
		return !bytes.Equal(a, b)
	case lt:
		return bytes.Compare(a, b) < 0
	case lte:
		return bytes.Compare(a, b) <= 0
	case gte:
		return bytes.Compare(a, b) >= 0
	case gt:
		return bytes.Compare(a, b) > 0
	default:
		return false
	}
}

func (p *ottlValueComparator) compareBool(a bool, b any, op compareOp) bool {
	switch v := b.(type) {
	case bool:
		return p.compareBools(a, v, op)
	case pcommon.Value:
		if v.Type() != pcommon.ValueTypeBool {
			return p.invalidComparison(op)
		}
		return p.compareBools(a, v.Bool(), op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareString(a string, b any, op compareOp) bool {
	switch v := b.(type) {
	case string:
		return comparePrimitives(a, v, op)
	case pcommon.Value:
		if v.Type() != pcommon.ValueTypeStr {
			return p.invalidComparison(op)
		}
		return comparePrimitives(a, v.Str(), op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareByte(a []byte, b any, op compareOp) bool {
	if a == nil {
		return p.compare(b, nil, op)
	}
	switch v := b.(type) {
	case nil:
		return op == ne
	case []byte:
		if v == nil {
			return op == ne
		}
		return p.compareBytes(a, v, op)
	case pcommon.Value:
		if v.Type() != pcommon.ValueTypeBytes {
			return p.invalidComparison(op)
		}
		return p.compareBytes(a, v.Bytes().AsRaw(), op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareInt64(a int64, b any, op compareOp) bool {
	switch v := b.(type) {
	case int64:
		return comparePrimitives(a, v, op)
	case float64:
		return comparePrimitives(float64(a), v, op)
	case pcommon.Value:
		switch v.Type() {
		case pcommon.ValueTypeInt:
			return comparePrimitives(a, v.Int(), op)
		case pcommon.ValueTypeDouble:
			return comparePrimitives(float64(a), v.Double(), op)
		default:
			return p.invalidComparison(op)
		}
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareFloat64(a float64, b any, op compareOp) bool {
	switch v := b.(type) {
	case int64:
		return comparePrimitives(a, float64(v), op)
	case float64:
		return comparePrimitives(a, v, op)
	case pcommon.Value:
		switch v.Type() {
		case pcommon.ValueTypeInt:
			return comparePrimitives(a, float64(v.Int()), op)
		case pcommon.ValueTypeDouble:
			return comparePrimitives(a, v.Double(), op)
		default:
			return p.invalidComparison(op)
		}
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareDuration(a time.Duration, b any, op compareOp) bool {
	switch v := b.(type) {
	case time.Duration:
		ansecs := a.Nanoseconds()
		vnsecs := v.Nanoseconds()
		return comparePrimitives(ansecs, vnsecs, op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareTime(a time.Time, b any, op compareOp) bool {
	switch v := b.(type) {
	case time.Time:
		switch op {
		case eq:
			return a.Equal(v)
		case ne:
			return !a.Equal(v)
		case lt:
			return a.Before(v)
		case lte:
			return a.Before(v) || a.Equal(v)
		case gte:
			return a.After(v) || a.Equal(v)
		case gt:
			return a.After(v)
		default:
			return p.invalidComparison(op)
		}
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareMap(a map[string]any, b any, op compareOp) bool {
	switch v := b.(type) {
	case pcommon.Map:
		switch op {
		case eq:
			return reflect.DeepEqual(a, v.AsRaw())
		case ne:
			return !reflect.DeepEqual(a, v.AsRaw())
		default:
			return p.invalidComparison(op)
		}
	case map[string]any:
		switch op {
		case eq:
			return reflect.DeepEqual(a, v)
		case ne:
			return !reflect.DeepEqual(a, v)
		default:
			return p.invalidComparison(op)
		}
	case pcommon.Value:
		if v.Type() != pcommon.ValueTypeMap {
			return p.invalidComparison(op)
		}
		return p.compareMap(a, v.Map().AsRaw(), op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) comparePMap(a pcommon.Map, b any, op compareOp) bool {
	switch v := b.(type) {
	case pcommon.Map:
		switch op {
		case eq:
			return a.Equal(v)
		case ne:
			return !a.Equal(v)
		default:
			return p.invalidComparison(op)
		}
	case map[string]any:
		return p.compareMap(a.AsRaw(), v, op)
	case pcommon.Value:
		if v.Type() != pcommon.ValueTypeMap {
			return p.invalidComparison(op)
		}
		return p.comparePMap(a, v.Map(), op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) compareSlice(a []any, b any, op compareOp) bool {
	switch v := b.(type) {
	case pcommon.Slice:
		switch op {
		case eq:
			return reflect.DeepEqual(a, v.AsRaw())
		case ne:
			return !reflect.DeepEqual(a, v.AsRaw())
		default:
			return p.invalidComparison(op)
		}
	case []any:
		switch op {
		case eq:
			return reflect.DeepEqual(a, v)
		case ne:
			return !reflect.DeepEqual(a, v)
		default:
			return p.invalidComparison(op)
		}
	case pcommon.Value:
		if v.Type() != pcommon.ValueTypeSlice {
			return p.invalidComparison(op)
		}
		return p.compareSlice(a, v.Slice().AsRaw(), op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) comparePSlice(a pcommon.Slice, b any, op compareOp) bool {
	switch v := b.(type) {
	case pcommon.Slice:
		switch op {
		case eq:
			return a.Equal(v)
		case ne:
			return !a.Equal(v)
		default:
			return p.invalidComparison(op)
		}
	case []any:
		return p.compareSlice(a.AsRaw(), v, op)
	case pcommon.Value:
		if v.Type() != pcommon.ValueTypeSlice {
			return p.invalidComparison(op)
		}
		return p.comparePSlice(a, v.Slice(), op)
	default:
		return p.invalidComparison(op)
	}
}

func (p *ottlValueComparator) comparePValue(a pcommon.Value, b any, op compareOp) bool {
	//exhaustive:enforce
	switch a.Type() {
	case pcommon.ValueTypeInt:
		return p.compareInt64(a.Int(), b, op)
	case pcommon.ValueTypeDouble:
		return p.compareFloat64(a.Double(), b, op)
	case pcommon.ValueTypeStr:
		return p.compareString(a.Str(), b, op)
	case pcommon.ValueTypeBool:
		return p.compareBool(a.Bool(), b, op)
	case pcommon.ValueTypeBytes:
		return p.compareByte(a.Bytes().AsRaw(), b, op)
	case pcommon.ValueTypeMap:
		return p.comparePMap(a.Map(), b, op)
	case pcommon.ValueTypeSlice:
		return p.comparePSlice(a.Slice(), b, op)
	case pcommon.ValueTypeEmpty:
		return p.compare(b, nil, op)
	}
	return p.invalidComparison(op)
}

// a and b are the return values from a Getter; we try to compare them
// according to the given operator.
func (p *ottlValueComparator) compare(a, b any, op compareOp) bool {
	// nils are equal to each other and never equal to anything else,
	// so if they're both nil, report equality.
	if a == nil && b == nil {
		return op == eq || op == lte || op == gte
	}
	// Normalize mixed collection representations before dispatching on the left side.
	if (op == eq || op == ne) && reflect.TypeOf(a) != reflect.TypeOf(b) {
		kind := comparisonCollectionKind(a)
		if (kind == reflect.Slice || kind == reflect.Map) && kind == comparisonCollectionKind(b) {
			a = normalizeComparisonCollection(a)
			b = normalizeComparisonCollection(b)
		}
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
		return p.compareByte(v, b, op)
	case []string:
		other, ok := b.([]string)
		if !ok || (op != eq && op != ne) {
			return p.invalidComparison(op)
		}
		equal := (v == nil) == (other == nil) && slices.Equal(v, other)
		return equal == (op == eq)
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
	case pcommon.Value:
		return p.comparePValue(v, b, op)
	default:
		// Go equality can panic for values with uncomparable dynamic types.
		switch op {
		case eq:
			return equalUnknown(a, b)
		case ne:
			return !equalUnknown(a, b)
		default:
			return p.invalidComparison(op)
		}
	}
}

func comparisonCollectionKind(value any) reflect.Kind {
	switch v := value.(type) {
	case []byte:
		return reflect.Invalid
	case pcommon.Slice:
		return reflect.Slice
	case pcommon.Map:
		return reflect.Map
	case pcommon.Value:
		switch v.Type() {
		case pcommon.ValueTypeSlice:
			return reflect.Slice
		case pcommon.ValueTypeMap:
			return reflect.Map
		}
		return reflect.Invalid
	default:
		return reflect.ValueOf(value).Kind()
	}
}

// normalizeComparisonCollection converts a typed slice or string-keyed map
// to its OTTL literal representation, preserving nil and the element types.
func normalizeComparisonCollection(value any) any {
	switch value.(type) {
	case []any, map[string]any:
		return value
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			return []any(nil)
		}
		items := make([]any, v.Len())
		for i := range items {
			items[i] = v.Index(i).Interface()
		}
		return items
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return value
		}
		if v.IsNil() {
			return map[string]any(nil)
		}
		items := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			items[iter.Key().String()] = iter.Value().Interface()
		}
		return items
	default:
		return value
	}
}

func equalUnknown(a, b any) bool {
	// a is never nil here, but b can be.
	if b == nil || (reflect.ValueOf(a).Comparable() && reflect.ValueOf(b).Comparable()) {
		return a == b
	}
	return reflect.DeepEqual(a, b)
}

func (p *ottlValueComparator) Equal(a, b any) bool {
	return p.compare(a, b, eq)
}

func (p *ottlValueComparator) NotEqual(a, b any) bool {
	return p.compare(a, b, ne)
}

func (p *ottlValueComparator) Less(a, b any) bool {
	return p.compare(a, b, lt)
}

func (p *ottlValueComparator) LessEqual(a, b any) bool {
	return p.compare(a, b, lte)
}

func (p *ottlValueComparator) Greater(a, b any) bool {
	return p.compare(a, b, gt)
}

func (p *ottlValueComparator) GreaterEqual(a, b any) bool {
	return p.compare(a, b, gte)
}

// NewValueComparator creates a new ValueComparator instance that can be used to compare
// values using the OTTL comparison rules.
func NewValueComparator() ValueComparator {
	return &ottlValueComparator{}
}
