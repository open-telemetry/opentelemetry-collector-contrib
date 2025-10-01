// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilecommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilecommon"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

type ProfileAttributable interface {
	AttributeIndices() pcommon.Int32Slice
}

type attributeSource[K any] = func(ctx K) (pprofile.ProfilesDictionary, ProfileAttributable)

func AccessAttributes[K any](source attributeSource[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			dict, attributable := source(tCtx)
			return pprofile.FromAttributeIndices(dict.AttributeTable(), attributable, dict), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			m, err := ctxutil.GetMap(val)
			if err != nil {
				return err
			}

			dict, attributable := source(tCtx)
			attributable.AttributeIndices().FromRaw([]int32{})
			for k, v := range m.All() {
				if err := pprofile.PutAttribute(dict.AttributeTable(), attributable, dict, k, v); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func AccessAttributesKey[K any](key []ottl.Key[K], source attributeSource[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			dict, attributable := source(tCtx)
			return ctxutil.GetMapValue[K](ctx, tCtx, pprofile.FromAttributeIndices(dict.AttributeTable(), attributable, dict), key)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			newKey, err := ctxutil.GetMapKeyName(ctx, tCtx, key[0])
			if err != nil {
				return err
			}

			dict, attributable := source(tCtx)
			v := getAttributeValue(dict, attributable.AttributeIndices(), *newKey)
			if err := ctxutil.SetIndexableValue[K](ctx, tCtx, v, val, key[1:]); err != nil {
				return err
			}

			return pprofile.PutAttribute(dict.AttributeTable(), attributable, dict, *newKey, v)
		},
	}
}

func getAttributeValue(dict pprofile.ProfilesDictionary, indices pcommon.Int32Slice, key string) pcommon.Value {
	strTable := dict.StringTable()
	kvuTable := dict.AttributeTable()

	for _, tableIndex := range indices.All() {
		attr := kvuTable.At(int(tableIndex))
		attrKey := strTable.At(int(attr.KeyStrindex()))
		if attrKey == key {
			// Copy the value because OTTL expects to do inplace updates for the values.
			v := pcommon.NewValueEmpty()
			attr.Value().CopyTo(v)
			return v
		}
	}

	return pcommon.NewValueEmpty()
}
