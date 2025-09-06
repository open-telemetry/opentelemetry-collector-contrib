// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilelocation // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilelocation"

import (
	"context"
	"errors"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

var (
	errMaxValueExceed   = errors.New("exceeded max value")
	errInvalidValueType = errors.New("invalid value type")
)

func PathGetSetter[K Context](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", Name, DocRef)
	}
	switch path.Name() {
	case "mapping_index":
		return accessMappingIndex[K](), nil
	case "address":
		return accessAddress[K](), nil
	case "line":
		return accessLine[K](), nil
	case "attribute_indices":
		return accessAttributeIndices[K](), nil
	case "attributes":
		if path.Keys() == nil {
			return accessAttributes[K](), nil
		}
		return accessAttributesKey(path.Keys()), nil
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessMappingIndex[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetProfileLocation().MappingIndex()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int64); ok {
				if v >= math.MaxInt32 {
					return errMaxValueExceed
				}
				tCtx.GetProfileLocation().SetMappingIndex(int32(v))
				return nil
			}
			return errInvalidValueType
		},
	}
}

func accessAddress[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfileLocation().Address(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(uint64); ok {
				tCtx.GetProfileLocation().SetAddress(v)
				return nil
			}
			return errInvalidValueType
		},
	}
}

func accessLine[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfileLocation().Line(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			lines, ok := val.(pprofile.LineSlice)
			if !ok {
				return errInvalidValueType
			}
			tCtx.GetProfileLocation().Line().RemoveIf(func(_ pprofile.Line) bool { return true })
			for _, line := range lines.All() {
				newLine := tCtx.GetProfileLocation().Line().AppendEmpty()
				line.CopyTo(newLine)
			}
			return nil
		},
	}
}

func accessAttributeIndices[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[int32](tCtx.GetProfileLocation().AttributeIndices()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[int32](tCtx.GetProfileLocation().AttributeIndices(), val)
		},
	}
}

func accessAttributes[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileLocation()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			m, err := ctxutil.GetMap(val)
			if err != nil {
				return err
			}
			tCtx.GetProfileLocation().AttributeIndices().FromRaw([]int32{})
			for k, v := range m.All() {
				if err := pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileLocation(), k, v); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func accessAttributesKey[K Context](key []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return ctxutil.GetMapValue[K](ctx, tCtx, pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileLocation()), key)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			newKey, err := ctxutil.GetMapKeyName(ctx, tCtx, key[0])
			if err != nil {
				return err
			}
			v := getAttributeValue(tCtx, *newKey)
			if err := ctxutil.SetIndexableValue[K](ctx, tCtx, v, val, key[1:]); err != nil {
				return err
			}
			return pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileLocation(), *newKey, v)
		},
	}
}

func getAttributeValue[K Context](tCtx K, key string) pcommon.Value {
	// Find the index of the attribute in the profile's attribute indices
	// and return the corresponding value from the attribute table.
	table := tCtx.GetProfilesDictionary().AttributeTable()
	indices := tCtx.GetProfileLocation().AttributeIndices().AsRaw()

	for _, tableIndex := range indices {
		attr := table.At(int(tableIndex))
		if attr.Key() == key {
			v := pcommon.NewValueEmpty()
			attr.Value().CopyTo(v)
			return v
		}
	}

	return pcommon.NewValueEmpty()
}
