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
	CONFLICT_REPLACE = "replace"
	CONFLICT_FAIL    = "fail"
	CONFLICT_IGNORE  = "ignore"
	CONFLICT_UPDATE  = "update"
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
	conflictStrategy := CONFLICT_REPLACE
	if !strategy.IsEmpty() {
		conflictStrategy = strategy.Get()
	}

	if conflictStrategy != CONFLICT_REPLACE &&
		conflictStrategy != CONFLICT_UPDATE &&
		conflictStrategy != CONFLICT_FAIL &&
		conflictStrategy != CONFLICT_IGNORE {
		return nil, fmt.Errorf("invalid value for conflict strategy, %v, must be %q, %q, %q or %q", conflictStrategy, CONFLICT_REPLACE, CONFLICT_UPDATE, CONFLICT_FAIL, CONFLICT_IGNORE)
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

			valueMissing := oldVal == nil
			switch conflictStrategy {
			case CONFLICT_REPLACE:
				// overwrites if present or create when missing
			case CONFLICT_UPDATE:
				// update existing field with a new value. If the field does not exist, then no action will be taken
				if valueMissing {
					return nil, nil
				}
			case CONFLICT_FAIL:
				// fail if target field present
				if !valueMissing {
					return nil, ErrValueAlreadyPresent
				}
			case CONFLICT_IGNORE:
				// noop if target field present
				if !valueMissing {
					return nil, nil
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
