// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	ErrValueAlreadyPresent = errors.New("value already present")
)

const (
	conflictReplace = "replace"
	conflictFail    = "fail"
	conflictIgnore  = "ignore"
	conflictUpdate  = "update"
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
	conflictStrategy := conflictReplace
	if !strategy.IsEmpty() {
		conflictStrategy = strategy.Get()
	}

	if conflictStrategy != conflictReplace &&
		conflictStrategy != conflictUpdate &&
		conflictStrategy != conflictFail &&
		conflictStrategy != conflictIgnore {
		return nil, fmt.Errorf("invalid value for conflict strategy, %v, must be %q, %q, %q or %q", conflictStrategy, conflictReplace, conflictUpdate, conflictFail, conflictIgnore)
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
			case conflictReplace:
				// overwrites if present or create when missing
			case conflictUpdate:
				// update existing field with a new value. If the field does not exist, then no action will be taken
				if valueMissing {
					return nil, nil
				}
			case conflictFail:
				// fail if target field present
				if !valueMissing {
					return nil, ErrValueAlreadyPresent
				}
			case conflictIgnore:
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
