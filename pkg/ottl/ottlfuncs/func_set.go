// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	ErrValueAlreadyPresent = fmt.Errorf("value already present")
)

const (
	REPLACE = "replace"
	FAIL    = "fail"
	IGNORE  = "ignore"
)

type SetArguments[K any] struct {
	Target           ottl.GetSetter[K]
	Value            ottl.Getter[K]
	ConflictStrategy ottl.Optional[string]
}

func NewSetFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("set", &SetArguments[K]{}, createSetFunction[K])
}

func createSetFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SetArguments[K])

	if !ok {
		return nil, fmt.Errorf("SetFactory args must be of type *SetArguments[K]")
	}

	return set(args.Target, args.Value, args.ConflictStrategy)
}

func set[K any](target ottl.GetSetter[K], value ottl.Getter[K], strategy ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	conflictStrategy := REPLACE
	if !strategy.IsEmpty() {
		conflictStrategy = strategy.Get()
	}

	if conflictStrategy != REPLACE && conflictStrategy != FAIL && conflictStrategy != IGNORE {
		return nil, fmt.Errorf("invalid value for conflict strategy, %v, must be %q, %q or %q", conflictStrategy, REPLACE, FAIL, IGNORE)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := value.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		// No fields currently support `null` as a valid type.
		if val != nil {
			oldVal, err := target.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}

			if oldVal != val {
				// there is value conflict
				switch conflictStrategy {
				case FAIL:
					return nil, ErrValueAlreadyPresent
				case IGNORE:
					// do nothing
					return nil, nil
				case REPLACE:
					// continue with set
				}
			}

			err = target.Set(ctx, tCtx, val)
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	}, nil
}
