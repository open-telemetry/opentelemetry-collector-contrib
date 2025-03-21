// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/exp/constraints"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func GetSliceValue[K any](ctx context.Context, tCtx K, s pcommon.Slice, keys []ottl.Key[K]) (any, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("cannot get slice value without key")
	}

	i, err := keys[0].Int(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	if i == nil {
		resInt, err := FetchValueFromExpression[K, int64](ctx, tCtx, keys[0])
		if err != nil {
			return nil, fmt.Errorf("unable to resolve an integer index in slice: %w", err)
		}
		i = resInt
	}

	idx := int(*i)

	if idx < 0 || idx >= s.Len() {
		return nil, fmt.Errorf("index %d out of bounds", idx)
	}

	return getIndexableValue[K](ctx, tCtx, s.At(idx), keys[1:])
}

func SetSliceValue[K any](ctx context.Context, tCtx K, s pcommon.Slice, keys []ottl.Key[K], val any) error {
	if len(keys) == 0 {
		return fmt.Errorf("cannot set slice value without key")
	}

	i, err := keys[0].Int(ctx, tCtx)
	if err != nil {
		return err
	}
	if i == nil {
		resInt, err := FetchValueFromExpression[K, int64](ctx, tCtx, keys[0])
		if err != nil {
			return fmt.Errorf("unable to resolve an integer index in slice: %w", err)
		}
		i = resInt
	}

	idx := int(*i)

	if idx < 0 || idx >= s.Len() {
		return fmt.Errorf("index %d out of bounds", idx)
	}

	return SetIndexableValue[K](ctx, tCtx, s.At(idx), val, keys[1:])
}

const msgInvalidType = "failed to convert type %T to %T"

func AsIntegerRawSlice[T constraints.Integer](val any) ([]T, error) {
	switch typeVal := val.(type) {
	case []T:
		return typeVal, nil
	case []any:
		res := make([]T, 0, len(typeVal))
		for _, v := range typeVal {
			switch v := v.(type) {
			case int:
				res = append(res, T(v))
			case int32:
				res = append(res, T(v))
			case int64:
				res = append(res, T(v))
			default:
				var typ T
				return nil, fmt.Errorf(msgInvalidType, v, typ)
			}
		}
		return res, nil
	case []int64:
		res := make([]T, 0, len(typeVal))
		for _, v := range typeVal {
			res = append(res, T(v))
		}
		return res, nil
	case pcommon.Int32Slice:
		res := make([]T, 0, typeVal.Len())
		for i := 0; i < typeVal.Len(); i++ {
			v := typeVal.At(i)
			res = append(res, T(v))
		}
		return res, nil
	default:
		var typ []T
		return nil, fmt.Errorf(msgInvalidType, val, typ)
	}
}

func AsRawSlice[T any](val any) ([]T, error) {
	switch typeVal := val.(type) {
	case []T:
		return typeVal, nil
	case []any:
		res := make([]T, 0, len(typeVal))
		for _, v := range typeVal {
			v, ok := v.(T)
			if !ok {
				var typ T
				return nil, fmt.Errorf(msgInvalidType, v, typ)
			}
			res = append(res, v)
		}
		return res, nil
	case pcommon.Slice:
		raw := typeVal.AsRaw()
		res := make([]T, 0, len(raw))
		for _, v := range raw {
			v, ok := v.(T)
			if !ok {
				var typ T
				return nil, fmt.Errorf(msgInvalidType, v, typ)
			}
			res = append(res, v)
		}
		return res, nil
	default:
		var typ []T
		return nil, fmt.Errorf(msgInvalidType, val, typ)
	}
}
