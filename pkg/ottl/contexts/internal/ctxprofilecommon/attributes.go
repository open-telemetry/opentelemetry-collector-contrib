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

type ProfileAttributeContext interface {
	AttributeIndices() pcommon.Int32Slice
	GetProfilesDictionary() pprofile.ProfilesDictionary
}

func AccessAttributes[K ProfileAttributeContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			m, err := ctxutil.GetMap(val)
			if err != nil {
				return err
			}
			for k, v := range m.All() {
				if err := pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx, k, v); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func AccessAttributesKey[K ProfileAttributeContext](key []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return ctxutil.GetMapValue[K](ctx, tCtx,
				pprofile.FromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx), key)
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
			return pprofile.PutAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx, *newKey, v)
		},
	}
}

func getAttributeValue[K ProfileAttributeContext](tCtx K, key string) pcommon.Value {
	// Find the index of the attribute in the profile's attribute indices
	// and return the corresponding value from the attribute table.
	table := tCtx.GetProfilesDictionary().AttributeTable()
	indices := tCtx.AttributeIndices().AsRaw()

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
