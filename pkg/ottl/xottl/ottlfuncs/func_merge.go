// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs/internal/funcutil"
)

type mergeArguments[K any] struct {
	Maps     xottl.SliceGetter[K, ottl.PMapGetter[K]]
	Resolver ottl.Optional[*xottl.LambdaExpression[K]]
}

// NewMergeFactory returns a factory for the Merge OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/xottl/ottlfuncs/README.md#merge
//
// The function requires the pkg.ottl.functions.enableExperimental feature gate to be enabled.
// The optional resolver argument additionally requires the ottl.functions.enableLambda feature gate to be enabled.
func NewMergeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Merge", &mergeArguments[K]{}, createMergeFunction[K], ottl.WithExperimental[K]())
}

func createMergeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*mergeArguments[K])
	if !ok {
		return nil, errors.New("MergeFactory args must be of type *mergeArguments[K]")
	}
	return merge(&args.Maps, &args.Resolver)
}

func merge[K any](maps *xottl.SliceGetter[K, ottl.PMapGetter[K]], resolver *ottl.Optional[*xottl.LambdaExpression[K]]) (ottl.ExprFunc[K], error) {
	if !resolver.IsEmpty() {
		if err := resolver.Get().ValidateArity(3); err != nil {
			return nil, fmt.Errorf("invalid resolver: %w", err)
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		var lb *xottl.LambdaActivation[K]
		if !resolver.IsEmpty() {
			var err error
			lb, err = resolver.Get().Activate(ctx)
			if err != nil {
				return nil, err
			}
			defer lb.Close()
		}

		result := pcommon.NewMap()
		var mergeErr error
		_, err := maps.Range(ctx, tCtx, func(getter ottl.PMapGetter[K]) bool {
			m, getErr := getter.Get(ctx, tCtx)
			if getErr != nil {
				mergeErr = getErr
				return false
			}
			mergeErr = mergeInto(tCtx, result, m, lb)
			return mergeErr == nil
		})
		if err != nil {
			return nil, err
		}
		if mergeErr != nil {
			return nil, mergeErr
		}
		return result, nil
	}, nil
}

// mergeInto copies all entries of next into result. Conflicting keys are either overwritten or,
// when lb is not nil, resolved by evaluating the lambda with (key, merged, next).
func mergeInto[K any](tCtx K, result, next pcommon.Map, lb *xottl.LambdaActivation[K]) error {
	for k, v := range next.All() {
		existing, found := result.Get(k)
		if !found {
			v.CopyTo(result.PutEmpty(k))
			continue
		}
		if lb == nil {
			v.CopyTo(existing)
			continue
		}
		// EvaluateFunction is not used here because it rejects nil results, which are valid for a resolver.
		err := funcutil.SetLambdaArgs(lb, k, existing, v)
		var resolved any
		if err == nil {
			resolved, err = lb.Eval(tCtx)
		}
		if err != nil {
			return fmt.Errorf("error while evaluating conflict resolver on key %q: %w", k, err)
		}
		// The lambda may return a view of existing (e.g. `(_, merged, _) => merged` with a map or slice
		// value). Copying directly into existing clears it before the copy, and when the
		// pdata.useProtoPooling feature gate is enabled the view then reads emptied pooled memory,
		// so the result is copied into a fresh value before replacing existing.
		fresh := pcommon.NewValueEmpty()
		if err = ottlcommon.CopyValueTo(resolved, fresh); err != nil {
			return fmt.Errorf("error while converting conflict resolver result on key %q: %w", k, err)
		}
		fresh.MoveTo(existing)
	}
	return nil
}
