// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// A context we use for parsing sub-expressions of a list comprehension.
type comprehensionContext[K any] struct {
	// Must of type K
	underlying     any
	currentValueID string
	currentValue   any
	index          int
}

// An expression like `[{yield} for {id} in {list} if {cond}]`.
type comprehensionExpr[K any] struct {
	currentValueID string
	// The list we're iterating over.
	listExpr Getter[K]
	// When is the iterator over. This is against the comprhension
	condExpr BoolExpr[comprehensionContext[any]]
	// The expression to evaluate at each item.
	yieldExpr Getter[comprehensionContext[any]]
}

func (c comprehensionExpr[K]) Get(ctx context.Context, tCtx K) (any, error) {
	listResult, err := c.listExpr.Get(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	var list []any
	switch l := listResult.(type) {
	case pcommon.Slice:
		list = l.AsRaw()
	case []any:
		list = l
	default:
		return nil, fmt.Errorf("unsupported list type: %s", reflect.TypeOf(listResult).Name())
	}
	result := []any{}
	for i, v := range list {
		subCtx := comprehensionContext[any]{tCtx, c.currentValueID, v, i}

		cond, err2 := c.condExpr.Eval(ctx, subCtx)
		if err2 != nil {
			return nil, err2
		}
		if cond {
			// Evaluate next value for result.
			item, err := c.yieldExpr.Get(ctx, subCtx)
			if err != nil {
				return nil, err
			}
			// TODO - cast the item?
			result = append(result, item)
		}
	}
	// We always return results in a pslice.
	sr := pcommon.NewSlice()
	err3 := sr.FromRaw(result)
	if err3 != nil {
		return nil, err3
	}
	return sr, nil
}

// Constructs a new list comprehension getter, where we will construct a new parser for list
// sub expressions that can delegate to list context.
func newListComprehensionGetter[K any](p *Parser[K], c *listComprehension) (Getter[K], error) {
	list, err := p.newGetter(c.List)
	if err != nil {
		return nil, err
	}
	// Now we create a new parser that uses this list.
	newParser, err := newComprehensionParser(p, c.Ident)
	if err != nil {
		return nil, err
	}

	yield, err := newParser.newGetter(c.Yield)
	if err != nil {
		return nil, err
	}

	cond, err := newParser.newBoolExpr(c.Cond)
	if err != nil {
		return nil, err
	}

	return &comprehensionExpr[K]{
		currentValueID: c.Ident,
		listExpr:       list,
		yieldExpr:      yield,
		condExpr:       cond,
	}, nil
}

// Builds a new list-comprehension as an argument to a function.
// c: the list comprehension.
// argType: The type expected.
func (p *Parser[K]) buildComprehensionSliceArg(c *listComprehension, _ reflect.Type) (any, error) {
	getter, err := newListComprehensionGetter(p, c)
	// TODO - deal with argType reflection.
	if err != nil {
		panic(err)
	}
	return getter, err
}

type wrappedGetter[K any] struct {
	g Getter[comprehensionContext[any]]
}

// Get implements Getter.
func (w *wrappedGetter[K]) Get(ctx context.Context, tCtx K) (any, error) {
	// Note - we know this cannot be called from the comprehensions context, so it HAS
	// to be an underlying one.  We cannot force-cast because go reflection/generic will kill us.
	// We can only construct a broken context that we know won't be abused because
	// the getter had to be wrapped by the nested parser.
	return w.g.Get(ctx, comprehensionContext[any]{underlying: tCtx})
}

type wrappedKey[K any] struct {
	k Key[comprehensionContext[any]]
}

// ExpressionGetter implements Key.
func (w *wrappedKey[K]) ExpressionGetter(ctx context.Context, kCtx K) (Getter[K], error) {
	// Note - we know this cannot be called from the comprehensions context, so it HAS
	// to be an underlying one.  We cannot force-cast because go reflection/generic will kill us.
	// We can only construct a broken context that we know won't be abused because
	// the getter had to be wrapped by the nested parser.
	wrapCtx := comprehensionContext[any]{
		underlying: kCtx,
	}
	underlying, err := w.k.ExpressionGetter(ctx, wrapCtx)
	if err != nil {
		return nil, err
	}
	return &wrappedGetter[K]{underlying}, err
}

// Int implements Key.
func (w *wrappedKey[K]) Int(ctx context.Context, kCtx K) (*int64, error) {
	// Note - we know this cannot be called from the comprehensions context, so it HAS
	// to be an underlying one.  We cannot force-cast because go reflection/generic will kill us.
	// We can only construct a broken context that we know won't be abused because
	// the getter had to be wrapped by the nested parser.
	wrapCtx := comprehensionContext[any]{
		underlying: kCtx,
	}
	return w.k.Int(ctx, wrapCtx)
}

// String implements Key.
func (w *wrappedKey[K]) String(ctx context.Context, kCtx K) (*string, error) {
	// Note - we know this cannot be called from the comprehensions context, so it HAS
	// to be an underlying one.  We cannot force-cast because go reflection/generic will kill us.
	// We can only construct a broken context that we know won't be abused because
	// the getter had to be wrapped by the nested parser.
	wrapCtx := comprehensionContext[any]{
		underlying: kCtx,
	}
	return w.k.String(ctx, wrapCtx)
}

type wrappedPath[K any] struct {
	p Path[comprehensionContext[any]]
}

// Context implements Path.
func (w *wrappedPath[K]) Context() string {
	return w.p.Context()
}

// Keys implements Path.
func (w *wrappedPath[K]) Keys() []Key[K] {
	keys := w.p.Keys()
	result := make([]Key[K], len(keys))
	for i, key := range keys {
		result[i] = &wrappedKey[K]{key}
	}
	return result
}

// Name implements Path.
func (w *wrappedPath[K]) Name() string {
	return w.p.Name()
}

// Next implements Path.
func (w *wrappedPath[K]) Next() Path[K] {
	return &wrappedPath[K]{w.p.Next()}
}

// String implements Path.
func (w *wrappedPath[K]) String() string {
	return w.p.String()
}

// Create a new parser that uses our loop-comprehension context instead of the original.
func newComprehensionParser[K any](p *Parser[K], ident string) (Parser[comprehensionContext[any]], error) {
	forceFunc := map[string]Factory[comprehensionContext[any]]{}
	for n, f := range p.functions {
		forceFunc[n] = NewFactory(
			f.Name(),
			f.CreateDefaultArguments(),
			func(fCtx FunctionContext, args Arguments) (ExprFunc[comprehensionContext[any]], error) {
				underlying, err := f.CreateFunction(fCtx, args)
				if err != nil {
					return nil, err
				}
				return func(ctx context.Context, tCtx comprehensionContext[any]) (any, error) {
					return underlying(ctx, tCtx.underlying.(K))
				}, nil
			},
		)
	}
	pathParser := func(path Path[comprehensionContext[any]]) (GetSetter[comprehensionContext[any]], error) {
		if path.Name() == ident {
			return &StandardGetSetter[comprehensionContext[any]]{
				Getter: func(_ context.Context, tCtx comprehensionContext[any]) (any, error) {
					// TODO - Remove this limitation in the future.
					// Likely forces rewrite of Getter/Setter pattern.
					if tCtx.currentValueID != ident {
						return nil, errors.New("list comprehensions cannot be nested")
					}
					return tCtx.currentValue, nil
				},
				Setter: func(context.Context, comprehensionContext[any], any) error {
					return fmt.Errorf("cannot set loop value: %s", ident)
				},
			}, nil
		}
		var forcedPath Path[K] = &wrappedPath[K]{path}
		res, err := p.pathParser(forcedPath)
		if err != nil {
			return nil, err
		}
		return &StandardGetSetter[comprehensionContext[any]]{
			Getter: func(ctx context.Context, tCtx comprehensionContext[any]) (any, error) {
				return res.Get(ctx, tCtx.underlying.(K))
			},
			Setter: func(ctx context.Context, tCtx comprehensionContext[any], val any) error {
				return res.Set(ctx, tCtx.underlying.(K), val)
			},
		}, nil
	}
	return NewParser(
		forceFunc,
		pathParser,
		p.telemetrySettings,
	)
}
