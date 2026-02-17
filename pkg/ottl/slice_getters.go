// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
)

// StringSliceGetter is a Getter that must return []string.
type StringSliceGetter[K any] interface {
	// Get retrieves a []string value.
	Get(ctx context.Context, tCtx K) ([]string, error)
}

// newStandardStringSliceGetter creates a new StandardStringSliceGetter from a Getter[K],
// also checking if the Getter is a literalGetter.
func newStandardStringSliceGetter[K any](getter Getter[K]) (StringSliceGetter[K], error) {
	g := StandardStringSliceGetter[K]{
		Getter: getter.Get,
	}
	if isLiteralGetter(getter) {
		val, err := g.Get(context.Background(), *new(K))
		if err != nil {
			return nil, err
		}
		return newLiteral[K, []string](val), nil
	}
	return g, nil
}

type stringGetterListAsSliceGetter[K any] struct {
	elems []StringGetter[K]
}

func newStringSliceGetterFromStringGetters[K any](elems []StringGetter[K]) StringSliceGetter[K] {
	values := make([]string, 0, len(elems))
	for _, elem := range elems {
		val, isLiteral := GetLiteralValue[K, string](elem)
		if !isLiteral {
			return &stringGetterListAsSliceGetter[K]{elems: elems}
		}
		values = append(values, val)
	}
	return newLiteral[K, []string](values)
}

func (g *stringGetterListAsSliceGetter[K]) Get(ctx context.Context, tCtx K) ([]string, error) {
	values := make([]string, len(g.elems))
	for i, elem := range g.elems {
		val, err := elem.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		values[i] = val
	}
	return values, nil
}

// StandardStringSliceGetter is a basic implementation of StringSliceGetter.
type StandardStringSliceGetter[K any] struct {
	Getter func(ctx context.Context, tCtx K) (any, error)
}

func (g StandardStringSliceGetter[K]) Get(ctx context.Context, tCtx K) ([]string, error) {
	val, err := g.Getter(ctx, tCtx)
	if err != nil {
		return nil, fmt.Errorf("error getting value in %T: %w", g, err)
	}
	sliceVal, err := valueToPSlice(val)
	if err != nil {
		return nil, err
	}
	return pSliceToStringSlice(sliceVal)
}

// StringLikeSliceGetter is a Getter that must return []*string.
type StringLikeSliceGetter[K any] interface {
	// Get retrieves a []*string value.
	Get(ctx context.Context, tCtx K) ([]*string, error)
}

// newStandardStringLikeSliceGetter creates a new StandardStringLikeSliceGetter from a Getter[K],
// also checking if the Getter is a literalGetter.
func newStandardStringLikeSliceGetter[K any](getter Getter[K]) (StringLikeSliceGetter[K], error) {
	g := StandardStringLikeSliceGetter[K]{
		Getter: getter.Get,
	}
	if isLiteralGetter(getter) {
		val, err := g.Get(context.Background(), *new(K))
		if err != nil {
			return nil, err
		}
		return newLiteral[K, []*string](val), nil
	}
	return g, nil
}

type stringLikeGetterListAsSliceGetter[K any] struct {
	elems []StringLikeGetter[K]
}

func newStringLikeSliceGetterFromStringLikeGetters[K any](elems []StringLikeGetter[K]) StringLikeSliceGetter[K] {
	values := make([]*string, 0, len(elems))
	for _, elem := range elems {
		val, isLiteral := GetLiteralValue[K, *string](elem)
		if !isLiteral {
			return &stringLikeGetterListAsSliceGetter[K]{elems: elems}
		}
		values = append(values, val)
	}
	return newLiteral[K, []*string](values)
}

func (g *stringLikeGetterListAsSliceGetter[K]) Get(ctx context.Context, tCtx K) ([]*string, error) {
	values := make([]*string, len(g.elems))
	for i, elem := range g.elems {
		val, err := elem.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		values[i] = val
	}
	return values, nil
}

// StandardStringLikeSliceGetter is a basic implementation of StringLikeSliceGetter.
type StandardStringLikeSliceGetter[K any] struct {
	Getter func(ctx context.Context, tCtx K) (any, error)
}

func (g StandardStringLikeSliceGetter[K]) Get(ctx context.Context, tCtx K) ([]*string, error) {
	val, err := g.Getter(ctx, tCtx)
	if err != nil {
		return nil, fmt.Errorf("error getting value in %T: %w", g, err)
	}
	sliceVal, err := valueToPSlice(val)
	if err != nil {
		return nil, err
	}
	return pSliceToStringLikeSlice(sliceVal)
}

func pSliceToStringSlice(sliceVal pcommon.Slice) ([]string, error) {
	values := make([]string, sliceVal.Len())
	for i := 0; i < sliceVal.Len(); i++ {
		strVal, err := valueToString(sliceVal.At(i))
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		values[i] = strVal
	}
	return values, nil
}

func pSliceToStringLikeSlice(sliceVal pcommon.Slice) ([]*string, error) {
	values := make([]*string, sliceVal.Len())
	for i := 0; i < sliceVal.Len(); i++ {
		// Unwrap pcommon.Value so slice-path elements stringify like literal-list
		// elements, including bytes (hex here, not pcommon.Value.AsString base64).
		stringLike, err := valueToStringLike(ottlcommon.GetValue(sliceVal.At(i)))
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		values[i] = stringLike
	}
	return values, nil
}
