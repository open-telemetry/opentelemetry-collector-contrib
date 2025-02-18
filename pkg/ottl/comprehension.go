// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// A context we use for parsing sub-expressions of a list comprehension.
type comprehensionContext[K any] struct {
	// Must of type K
	underlying     any
	currentValueId string
	currentValue   any
	index          int
}

// An expression like `[{yield} for {list} where {cond}]`.
type comprehensionExpr[K any] struct {
	currentValueId string
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
		subCtx := comprehensionContext[any]{tCtx, c.currentValueId, v, i}

		cond, err := c.condExpr.Eval(ctx, subCtx)
		if err != nil {
			return nil, err
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
	sr.FromRaw(result)
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
	cond := BoolExpr[comprehensionContext[any]]{alwaysTrue[comprehensionContext[any]]}
	if c.Cond != nil {
		cond, err = newParser.newBoolExpr(c.Cond)
		if err != nil {
			return nil, err
		}
	}
	return &comprehensionExpr[K]{
		currentValueId: c.Ident,
		listExpr:       list,
		yieldExpr:      yield,
		condExpr:       cond,
	}, nil
}

// Builds a new list-comprehension as an argument to a function.
// c: the list comprehension.
// argType: The type expected.
func (p *Parser[K]) buildComprehensionSliceArg(c *listComprehension, argType reflect.Type) (any, error) {
	getter, err := newListComprehensionGetter(p, c)
	// TODO - deal with argType reflection.
	return getter, err
}

// Hackily construct a new parser that uses our loop-comprehension context instead of the original.
func newComprehensionParser[K any](p *Parser[K], ident string) (Parser[comprehensionContext[any]], error) {
	forceFunc := map[string]Factory[comprehensionContext[any]]{}
	for n, f := range p.functions {
		forceFunc[n] = NewFactory[comprehensionContext[any]](
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
				Getter: func(ctx context.Context, tCtx comprehensionContext[any]) (any, error) {
					// TODO - Remove this limitation in the future.
					// Likely forces rewrite of Getter/Setter pattern.
					if tCtx.currentValueId != ident {
						return nil, fmt.Errorf("list comprehensions cannot be nested!")
					}
					return tCtx.currentValue, nil
				},
				Setter: func(ctx context.Context, tCtx comprehensionContext[any], val any) error {
					return fmt.Errorf("cannot set loop index value: %s", ident)
				},
			}, nil
		}
		var forcedPath any = path
		res, err := p.pathParser(forcedPath.(Path[K]))
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
	return NewParser[comprehensionContext[any]](
		forceFunc,
		pathParser,
		p.telemetrySettings,
	)
}
