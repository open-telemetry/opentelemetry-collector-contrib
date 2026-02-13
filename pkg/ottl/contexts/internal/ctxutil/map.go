// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
)

type mapKeyGetSetter[K any] struct {
	keys            []ottl.Key[K]
	firstLiteralKey *string
	mapGetter       func(K) pcommon.Map
}

// NewMapKeyGetSetter creates a map-backed accessor for keyed paths.
// The first key literal status is resolved once at creation time.
func NewMapKeyGetSetter[K any](keys []ottl.Key[K], mapGetter func(K) pcommon.Map) ottl.GetSetter[K] {
	return &mapKeyGetSetter[K]{
		keys:            keys,
		firstLiteralKey: getFirstLiteralMapKey(keys),
		mapGetter:       mapGetter,
	}
}

func (m *mapKeyGetSetter[K]) Get(ctx context.Context, tCtx K) (any, error) {
	return getMapValue[K](ctx, tCtx, m.mapGetter(tCtx), m.keys, m.firstLiteralKey)
}

func (m *mapKeyGetSetter[K]) Set(ctx context.Context, tCtx K, val any) error {
	return setMapValue[K](ctx, tCtx, m.mapGetter(tCtx), m.keys, m.firstLiteralKey, val)
}

func getFirstLiteralMapKey[K any](keys []ottl.Key[K]) *string {
	if len(keys) == 0 {
		return nil
	}
	literalKey, ok := ottl.GetLiteralKeyString(keys[0])
	if !ok {
		return nil
	}
	return literalKey
}

func GetMapValue[K any](ctx context.Context, tCtx K, m pcommon.Map, keys []ottl.Key[K]) (any, error) {
	return getMapValue[K](ctx, tCtx, m, keys, getFirstLiteralMapKey(keys))
}

func getMapValue[K any](ctx context.Context, tCtx K, m pcommon.Map, keys []ottl.Key[K], firstLiteralKey *string) (any, error) {
	if len(keys) == 0 {
		return nil, errors.New("cannot get map value without keys")
	}

	if firstLiteralKey != nil {
		val, exists := m.Get(*firstLiteralKey)
		if !exists {
			return nil, nil
		}

		if len(keys) == 1 {
			return ottlcommon.GetValue(val), nil
		}

		return getIndexableValue[K](ctx, tCtx, val, keys[1:])
	}

	s, err := GetMapKeyName(ctx, tCtx, keys[0])
	if err != nil {
		return nil, fmt.Errorf("cannot get map value: %w", err)
	}

	val, ok := m.Get(*s)
	if !ok {
		return nil, nil
	}

	return getIndexableValue[K](ctx, tCtx, val, keys[1:])
}

func SetMapValue[K any](ctx context.Context, tCtx K, m pcommon.Map, keys []ottl.Key[K], val any) error {
	return setMapValue[K](ctx, tCtx, m, keys, getFirstLiteralMapKey(keys), val)
}

func setMapValue[K any](ctx context.Context, tCtx K, m pcommon.Map, keys []ottl.Key[K], firstLiteralKey *string, val any) error {
	if len(keys) == 0 {
		return errors.New("cannot set map value without keys")
	}

	if firstLiteralKey != nil {
		currentValue, exists := m.Get(*firstLiteralKey)
		if !exists {
			currentValue = m.PutEmpty(*firstLiteralKey)
		}

		return SetIndexableValue[K](ctx, tCtx, currentValue, val, keys[1:])
	}

	s, err := GetMapKeyName(ctx, tCtx, keys[0])
	if err != nil {
		return fmt.Errorf("cannot set map value: %w", err)
	}

	currentValue, ok := m.Get(*s)
	if !ok {
		currentValue = m.PutEmpty(*s)
	}
	return SetIndexableValue[K](ctx, tCtx, currentValue, val, keys[1:])
}

func GetMapKeyName[K any](ctx context.Context, tCtx K, key ottl.Key[K]) (*string, error) {
	if literalKey, ok := ottl.GetLiteralKeyString(key); ok {
		return literalKey, nil
	}

	resolvedKey, err := key.String(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	if resolvedKey == nil {
		resolvedKey, err = FetchValueFromExpression[K, string](ctx, tCtx, key)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve a string index in map: %w", err)
		}
	}
	return resolvedKey, nil
}

func FetchValueFromExpression[K any, T int64 | string](ctx context.Context, tCtx K, key ottl.Key[K]) (*T, error) {
	p, err := key.ExpressionGetter(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("invalid key type")
	}
	res, err := p.Get(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	resVal, ok := res.(T)
	if !ok {
		return nil, fmt.Errorf("could not resolve key for map/slice, expecting '%T' but got '%T'", resVal, res)
	}
	return &resVal, nil
}

func SetMap(target pcommon.Map, val any) error {
	if cm, ok := val.(pcommon.Map); ok {
		cm.CopyTo(target)
		return nil
	}
	if rm, ok := val.(map[string]any); ok {
		return target.FromRaw(rm)
	}
	return nil
}

func GetMap(val any) (pcommon.Map, error) {
	if m, ok := val.(pcommon.Map); ok {
		return m, nil
	}
	if rm, ok := val.(map[string]any); ok {
		m := pcommon.NewMap()
		if err := m.FromRaw(rm); err != nil {
			return pcommon.Map{}, err
		}
		return m, nil
	}
	return pcommon.Map{}, fmt.Errorf("failed to convert type %T into pcommon.Map", val)
}
