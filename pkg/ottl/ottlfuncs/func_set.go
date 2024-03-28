// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var (
	ErrValueAlreadyPresent = errors.New("value already present")
)

const (
	setConflictUpsert = "upsert"
	setConflictFail   = "fail"
	setConflictInsert = "insert"
	setConflictUpdate = "update"
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
	conflictStrategy := setConflictUpsert
	if !strategy.IsEmpty() {
		conflictStrategy = strategy.Get()
	}

	if conflictStrategy != setConflictUpsert &&
		conflictStrategy != setConflictUpdate &&
		conflictStrategy != setConflictFail &&
		conflictStrategy != setConflictInsert {
		return nil, fmt.Errorf("invalid value for conflict strategy, %v, must be %q, %q, %q or %q", conflictStrategy, setConflictUpsert, setConflictUpdate, setConflictFail, setConflictUpsert)
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

			valueMissing := oldVal == nil || isDefaultValue(oldVal)
			switch conflictStrategy {
			case setConflictUpsert:
				// overwrites if present or create when missing
			case setConflictUpdate:
				// update existing field with a new value. If the field does not exist, then no action will be taken
				if valueMissing {
					return nil, nil
				}
			case setConflictFail:
				// fail if target field present
				if !valueMissing {
					return nil, ErrValueAlreadyPresent
				}
			case setConflictInsert:
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

func isDefaultValue(val any) bool {
	switch v := val.(type) {
	case string:
		return len(v) == 0
	case bool:
		return !v
	case float64:
		return v == 0
	case int64:
		return v == 0

	case []byte:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case []int64:
		return len(v) == 0
	case []float64:
		return len(v) == 0
	case [][]byte:
		return len(v) == 0
	case []any:
		return len(v) == 0

	case pcommon.ByteSlice:
		return v.Len() == 0
	case pcommon.Float64Slice:
		return v.Len() == 0
	case pcommon.UInt64Slice:
		return v.Len() == 0
	case pcommon.Slice:
		return v.Len() == 0

	case pcommon.Value:
		if v.Type() == pcommon.ValueTypeStr ||
			v.Type() == pcommon.ValueTypeBytes ||
			v.Type() == pcommon.ValueTypeSlice {
			return v.Bytes().Len() == 0
		}
		if v.Type() == pcommon.ValueTypeBool {
			return !v.Bool()
		}
		if v.Type() == pcommon.ValueTypeDouble {
			return v.Double() == 0
		}
		if v.Type() == pcommon.ValueTypeInt {
			return v.Int() == 0
		}

		if v.Type() == pcommon.ValueTypeEmpty {
			return true
		}

	case pcommon.Map:
		return v.Len() == 0
	case map[string]any:
		return v == nil
	}

	return false
}
