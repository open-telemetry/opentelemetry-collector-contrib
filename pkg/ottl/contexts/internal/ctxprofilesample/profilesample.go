// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilesample // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilesample"

import (
	"context"
	"errors"
	"math"
	"time"

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
	case "values":
		return accessValues[K](), nil
	case "attribute_indices":
		return accessAttributeIndices[K](), nil
	case "link_index":
		return accessLinkIndex[K](), nil
	case "timestamps_unix_nano":
		return accessTimestampsUnixNano[K](), nil
	case "timestamps":
		return accessTimestamps[K](), nil
	case "attributes":
		if path.Keys() == nil {
			return accessAttributes[K](), nil
		}
		return accessAttributesKey(path.Keys()), nil
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessValues[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[int64](tCtx.GetProfileSample().Values()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[int64](tCtx.GetProfileSample().Values(), val)
		},
	}
}

func accessAttributeIndices[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[int32](tCtx.GetProfileSample().AttributeIndices()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[int32](tCtx.GetProfileSample().AttributeIndices(), val)
		},
	}
}

func accessLinkIndex[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetProfileSample().LinkIndex()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int64); ok {
				if v >= math.MaxInt32 {
					return errMaxValueExceed
				}
				tCtx.GetProfileSample().SetLinkIndex(int32(v))
				return nil
			}
			return errInvalidValueType
		},
	}
}

func accessTimestampsUnixNano[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[uint64](tCtx.GetProfileSample().TimestampsUnixNano()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[uint64](tCtx.GetProfileSample().TimestampsUnixNano(), val)
		},
	}
}

func accessTimestamps[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			var ts []time.Time
			for _, t := range tCtx.GetProfileSample().TimestampsUnixNano().All() {
				ts = append(ts, time.Unix(0, int64(t)).UTC())
			}
			return ts, nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if ts, ok := val.([]time.Time); ok {
				tCtx.GetProfileSample().TimestampsUnixNano().FromRaw([]uint64{})
				for _, t := range ts {
					tCtx.GetProfileSample().TimestampsUnixNano().Append(uint64(t.UTC().UnixNano()))
				}
			}
			return nil
		},
	}
}

func accessAttributes[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample(), tCtx.GetProfilesDictionary()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			m, err := ctxutil.GetMap(val)
			if err != nil {
				return err
			}
			tCtx.GetProfileSample().AttributeIndices().FromRaw([]int32{})
			for k, v := range m.All() {
				if err := pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample(), tCtx.GetProfilesDictionary(), k, v); err != nil {
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
			return ctxutil.GetMapValue[K](ctx, tCtx, pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample(), tCtx.GetProfilesDictionary()), key)
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
			return pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample(), tCtx.GetProfilesDictionary(), *newKey, v)
		},
	}
}

func getAttributeValue[K Context](tCtx K, key string) pcommon.Value {
	// Find the index of the attribute in the profile's attribute indices
	// and return the corresponding value from the attribute table.
	table := tCtx.GetProfilesDictionary().AttributeTable()
	strTable := tCtx.GetProfilesDictionary().StringTable()

	for _, tableIndex := range tCtx.GetProfileSample().AttributeIndices().All() {
		attr := table.At(int(tableIndex))
		if strTable.At(int(attr.KeyStrindex())) == key {
			// Copy the value because OTTL expects to do inplace updates for the values.
			v := pcommon.NewValueEmpty()
			attr.Value().CopyTo(v)
			return v
		}
	}

	return pcommon.NewValueEmpty()
}
