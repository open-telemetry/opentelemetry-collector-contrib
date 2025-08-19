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
			return fromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			m, err := ctxutil.GetMap(val)
			if err != nil {
				return err
			}
			for k, v := range m.All() {
				if err := putAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx, k, v); err != nil {
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
			return ctxutil.GetMapValue[K](ctx, tCtx, fromAttributeIndices(tCtx.GetProfilesDictionary().AttributeTable(), tCtx), key)
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
			return putAttribute(tCtx.GetProfilesDictionary().AttributeTable(), tCtx, *newKey, v)
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

// fromAttributeIndices creates a pcommon.Map from the attribute indices in the provided context
func fromAttributeIndices(attributeTable pprofile.AttributeTableSlice, ctx ProfileAttributeContext) pcommon.Map {
	m := pcommon.NewMap()
	indices := ctx.AttributeIndices().AsRaw()

	for _, tableIndex := range indices {
		if int(tableIndex) < attributeTable.Len() {
			attr := attributeTable.At(int(tableIndex))
			attr.Value().CopyTo(m.PutEmpty(attr.Key()))
		}
	}

	return m
}

// putAttribute adds or updates an attribute in the attribute table and updates the context's attribute indices
func putAttribute(attributeTable pprofile.AttributeTableSlice, ctx ProfileAttributeContext, key string, value pcommon.Value) error {
	// First, check if the attribute already exists in the context's indices
	indices := ctx.AttributeIndices()
	existingIndex := -1

	for i := 0; i < indices.Len(); i++ {
		tableIndex := indices.At(i)
		if int(tableIndex) < attributeTable.Len() {
			attr := attributeTable.At(int(tableIndex))
			if attr.Key() == key {
				existingIndex = int(tableIndex)
				break
			}
		}
	}

	if existingIndex >= 0 {
		// Update existing attribute
		attr := attributeTable.At(existingIndex)
		value.CopyTo(attr.Value())
	} else {
		// Add new attribute to the table
		newAttr := attributeTable.AppendEmpty()
		newAttr.SetKey(key)
		value.CopyTo(newAttr.Value())

		// Add the new index to the context's attribute indices
		newIndex := int32(attributeTable.Len() - 1)
		indices.Append(newIndex)
	}

	return nil
}
