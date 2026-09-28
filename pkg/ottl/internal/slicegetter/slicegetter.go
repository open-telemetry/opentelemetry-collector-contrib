// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package slicegetter implements the SliceGetter OTTL function argument. It is exposed
// publicly by the xottl module and used internally by stable OTTL functions.
package slicegetter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/slicegetter"

import (
	"context"
	"fmt"
	"reflect"
)

// Source resolves a slice argument that was given as a path or expression. It is
// implemented by the OTTL parser, which owns coercing items into the element type.
type Source[K any] interface {
	// IsLiteral reports whether Get returns the same value regardless of the transform context.
	IsLiteral() bool
	Get(ctx context.Context, tCtx K) (any, error)
	// Len returns the length of slice and whether it is a slice.
	Len(slice any) (int, bool)
	// Range coerces each item of slice to the element type and yields it, stopping when
	// yield returns false. The returned boolean reports whether slice is non-nil.
	Range(slice any, yield func(item any) bool) (bool, error)
}

// SliceGetter represents a slice argument in OTTL functions. Unlike bare []V
// parameters, which only accept literal lists [value], it also accepts a single
// getter (path or expression) that resolves to []V at runtime.
// V is the element type of the resolved slice. It may be a typed Getter
// (e.g.: StringGetter[K]) or a scalar type supported by OTTL.
type SliceGetter[K, V any] struct {
	typedValues []V
	source      Source[K]
}

// arg is implemented by every SliceGetter instantiation so the parser can populate one
// without knowing its type parameters.
type arg interface {
	itemType() reflect.Type
	set(val any) error
}

var _ arg = (*SliceGetter[any, any])(nil)

// ItemType returns the element type of the SliceGetter pointed to by ptr, and false if
// ptr is not a pointer to a SliceGetter.
func ItemType(ptr any) (reflect.Type, bool) {
	a, ok := ptr.(arg)
	if !ok {
		return nil, false
	}
	return a.itemType(), true
}

// Set stores val, which must be a []V or a Source[K], into the SliceGetter pointed to by ptr.
func Set(ptr, val any) error {
	a, ok := ptr.(arg)
	if !ok {
		return fmt.Errorf("cannot set a slice argument on %T", ptr)
	}
	return a.set(val)
}

func (*SliceGetter[K, V]) itemType() reflect.Type {
	return reflect.TypeFor[V]()
}

func (s *SliceGetter[K, V]) set(val any) error {
	switch v := val.(type) {
	case []V:
		s.typedValues = v
	case Source[K]:
		if typedValues, ok := getLiterals[K, V](v); ok {
			s.typedValues = typedValues
		} else {
			s.source = v
		}
	default:
		return fmt.Errorf("cannot set value of type %T to a slice of type %s", val, reflect.TypeFor[V]())
	}
	return nil
}

// getLiterals extracts the items of a literal Source. It returns the slice and a boolean
// indicating if the extraction was successful. The returned values can be either scalar or
// typed getters, which might not hold literal values. In this context, literals mean items
// can be retrieved from the slice without evaluating it.
func getLiterals[K, V any](source Source[K]) ([]V, bool) {
	if !source.IsLiteral() {
		return nil, false
	}
	sliceValues, err := source.Get(context.Background(), *new(K))
	if err != nil {
		return nil, false
	}
	if sliceValues == nil {
		return nil, true
	}
	if typedValues, ok := sliceValues.([]V); ok {
		return typedValues, true
	}

	var result []V
	if size, ok := source.Len(sliceValues); ok {
		result = make([]V, 0, size)
	}
	complete := true
	nonNil, err := source.Range(sliceValues, func(val any) bool {
		if tv, ok := val.(V); ok {
			result = append(result, tv)
			return true
		}
		complete = false
		return false
	})
	if err != nil {
		return nil, false
	}
	if !nonNil {
		return nil, true
	}
	if !complete {
		return nil, false
	}
	return result, complete
}

// GetScalarLiteralValues retrieves the literal values from the given slice of scalars.
// If the values cannot be retrieved, it returns the zero value of []V and false.
// [V] must be a scalar type supported by OTTL slice arguments.
func GetScalarLiteralValues[
	K any,
	V ~uint8 | ~int64 | ~float64 | ~string,
](slice *SliceGetter[K, V]) ([]V, bool) {
	if slice.source != nil {
		return nil, false
	}
	var result []V
	_, err := slice.Range(
		context.Background(),
		*new(K),
		func(value V) bool {
			result = append(result, value)
			return true
		},
	)
	if err != nil {
		return nil, false
	}
	return result, true
}

// GetLiteralValues retrieves the literal values from the given slice of getters, using
// literalValue to read each item. If an item is not a literal, it returns the zero value
// of []V and false.
func GetLiteralValues[K, V, G any](slice *SliceGetter[K, G], literalValue func(G) (V, bool)) ([]V, bool) {
	if slice.source != nil {
		return nil, false
	}
	var result []V
	allLiterals := true
	_, err := slice.Range(context.Background(), *new(K), func(value G) bool {
		val, ok := literalValue(value)
		if !ok {
			allLiterals = false
			return false
		}
		result = append(result, val)
		return true
	})
	if err != nil {
		return nil, false
	}
	if !allLiterals {
		return nil, false
	}
	return result, true
}

// Range iterates over elements in the slice and applies the yield function to each item.
// The yield function returns true to continue iterating, false to stop.
//
// The returned boolean reports whether the underlying slice is non-nil, even if
// it is empty or iteration stops early. Ignore the boolean on error.
func (s *SliceGetter[K, V]) Range(ctx context.Context, tCtx K, yield func(value V) bool) (bool, error) {
	if s.source != nil {
		return s.rangeSource(ctx, tCtx, yield)
	}
	return rangeTypedSlice(s.typedValues, yield)
}

func (s *SliceGetter[K, V]) rangeSource(ctx context.Context, tCtx K, yield func(value V) bool) (bool, error) {
	values, err := s.source.Get(ctx, tCtx)
	if err != nil {
		return false, err
	}
	if values == nil {
		return false, nil
	}

	if typedValues, ok := values.([]V); ok {
		return rangeTypedSlice(typedValues, yield)
	}

	var rangeErr error
	nonNil, err := s.source.Range(values, func(val any) bool {
		if v, ok := val.(V); ok {
			return yield(v)
		}
		rangeErr = itemTypeError[V](val)
		return false
	})
	if err != nil {
		return false, err
	}
	if rangeErr != nil {
		return false, rangeErr
	}

	return nonNil, nil
}

// rangeTypedSlice iterates over elements in the typed slice and applies the yield function to each item.
// The returned boolean reports whether the underlying slice is non-nil.
func rangeTypedSlice[V any](typedValues []V, yield func(value V) bool) (bool, error) {
	if typedValues == nil {
		return false, nil
	}
	for _, v := range typedValues {
		if !yield(v) {
			return true, nil
		}
	}
	return true, nil
}

// Len returns the length of the slice when it can be determined without evaluation.
// For literal slices it returns the length and true, otherwise it returns 0 and false.
func (s *SliceGetter[K, V]) Len() (int, bool) {
	if s.typedValues != nil {
		return len(s.typedValues), true
	}
	return 0, false
}

// Get retrieves all values as []V.
// If any slice element is not coercible to V, it returns an error.
func (s *SliceGetter[K, V]) Get(ctx context.Context, tCtx K) ([]V, error) {
	if s.source != nil {
		return s.getSourceValue(ctx, tCtx)
	}
	return s.typedValues, nil
}

func (s *SliceGetter[K, V]) getSourceValue(ctx context.Context, tCtx K) ([]V, error) {
	values, err := s.source.Get(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}

	if typedValues, ok := values.([]V); ok {
		return typedValues, nil
	}

	var result []V
	if count, ok := s.source.Len(values); ok {
		result = make([]V, 0, count)
	}

	var rangeErr error
	nonNil, err := s.source.Range(values, func(val any) bool {
		if v, ok := val.(V); ok {
			result = append(result, v)
			return true
		}
		rangeErr = itemTypeError[V](val)
		return false
	})
	if err != nil {
		return nil, err
	}
	if rangeErr != nil {
		return nil, rangeErr
	}
	if !nonNil {
		return nil, nil
	}

	return result, nil
}

func itemTypeError[V any](val any) error {
	return fmt.Errorf("expected slice item of type %s, got %s", reflect.TypeFor[V](), reflect.TypeOf(val))
}

// NewTesting creates a SliceGetter for use in tests. When literal is true the values are
// stored as a literal list, otherwise they are resolved at runtime.
func NewTesting[K, T any](literal bool, values []T) *SliceGetter[K, T] {
	if literal {
		return &SliceGetter[K, T]{typedValues: values}
	}
	return &SliceGetter[K, T]{source: testingSource[K, T]{values: values}}
}

type testingSource[K, T any] struct {
	values []T
}

func (testingSource[K, T]) IsLiteral() bool {
	return false
}

func (s testingSource[K, T]) Get(context.Context, K) (any, error) {
	return s.values, nil
}

func (s testingSource[K, T]) Len(any) (int, bool) {
	return len(s.values), true
}

func (s testingSource[K, T]) Range(_ any, yield func(item any) bool) (bool, error) {
	if s.values == nil {
		return false, nil
	}
	for _, v := range s.values {
		if !yield(v) {
			break
		}
	}
	return true, nil
}
