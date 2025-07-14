// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilesample // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilesample"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

type ProfileSampleContext interface {
	GetProfileSample() pprofile.Sample
	GetProfilesDictionary() pprofile.ProfilesDictionary
}

func PathGetSetter[K ProfileSampleContext](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", Name, DocRef)
	}
	switch path.Name() {
	case "locations_start_index":
		return accessLocationsStartIndex[K](), nil
	case "locations_length":
		return accessLocationsLength[K](), nil
	case "values":
		return accessValues[K](), nil
	case "attribute_indices":
		return accessAttributeIndices[K](), nil
	case "link_index":
		return accessLinkIndex[K](), nil
	case "timestamps_unix_nano":
		return accessTimestampsUnixNano[K](), nil
	case "attributes":
		if path.Keys() == nil {
			return accessAttributes[K](), nil
		}
		return accessAttributesKey(path.Keys()), nil
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessLocationsStartIndex[K ProfileSampleContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfileSample().LocationsStartIndex(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int32); ok {
				tCtx.GetProfileSample().SetLocationsStartIndex(v)
			}
			return nil
		},
	}
}

func accessLocationsLength[K ProfileSampleContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfileSample().LocationsLength(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int32); ok {
				tCtx.GetProfileSample().SetLocationsLength(v)
			}
			return nil
		},
	}
}

func accessValues[K ProfileSampleContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[int64](tCtx.GetProfileSample().Value()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[int64](tCtx.GetProfileSample().Value(), val)
		},
	}
}

func accessAttributeIndices[K ProfileSampleContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[int32](tCtx.GetProfileSample().AttributeIndices()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[int32](tCtx.GetProfileSample().AttributeIndices(), val)
		},
	}
}

func accessLinkIndex[K ProfileSampleContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfileSample().LinkIndex(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int32); ok {
				tCtx.GetProfileSample().SetLinkIndex(v)
			}
			return nil
		},
	}
}

func accessTimestampsUnixNano[K ProfileSampleContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[uint64](tCtx.GetProfileSample().TimestampsUnixNano()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[uint64](tCtx.GetProfileSample().TimestampsUnixNano(), val)
		},
	}
}

func accessAttributes[K ProfileSampleContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			m, err := ctxutil.GetMap(val)
			if err != nil {
				return err
			}
			tCtx.GetProfileSample().AttributeIndices().FromRaw([]int32{})
			for k, v := range m.All() {
				if err := pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample(), k, v); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func accessAttributesKey[K ProfileSampleContext](key []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return ctxutil.GetMapValue[K](ctx, tCtx, pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample()), key)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			newKey, err := ctxutil.GetMapKeyName(ctx, tCtx, key[0])
			if err != nil {
				return err
			}
			v := getAttributeValue(tCtx, *newKey)
			if err = ctxutil.SetIndexableValue[K](ctx, tCtx, v, val, key[1:]); err != nil {
				return err
			}
			return pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx.GetProfileSample(), *newKey, v)
		},
	}
}

func getAttributeValue[K ProfileSampleContext](tCtx K, key string) pcommon.Value {
	// Find the index of the attribute in the profile's attribute indices
	// and return the corresponding value from the attribute table.
	table := tCtx.GetProfilesDictionary().AttributeTable()
	indices := tCtx.GetProfileSample().AttributeIndices().AsRaw()

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
