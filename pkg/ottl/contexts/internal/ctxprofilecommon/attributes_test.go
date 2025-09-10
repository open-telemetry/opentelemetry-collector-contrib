// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilecommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilecommon"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

// Mock implementations for AttributeContext and dependencies

type mockAttributeContext struct {
	indices    pcommon.Int32Slice
	dictionary pprofile.ProfilesDictionary
}

func (m *mockAttributeContext) AttributeIndices() pcommon.Int32Slice {
	return m.indices
}

func (m *mockAttributeContext) GetProfilesDictionary() pprofile.ProfilesDictionary {
	return m.dictionary
}

func mockAttributeSource(ctx *mockAttributeContext) (pprofile.ProfilesDictionary, ProfileAttributable) {
	return ctx.dictionary, ctx
}

func TestAccessAttributes_Getter(t *testing.T) {
	dict := pprofile.NewProfilesDictionary()
	attrTable := dict.AttributeTable()
	attr1 := attrTable.AppendEmpty()
	attr1.SetKey("foo")
	attr1.Value().SetStr("bar")
	attr2 := attrTable.AppendEmpty()
	attr2.SetKey("baz")
	attr2.Value().SetInt(42)

	indices := pcommon.NewInt32Slice()
	indices.Append(0)
	indices.Append(1)

	ctx := &mockAttributeContext{
		indices:    indices,
		dictionary: dict,
	}

	getSetter := AccessAttributes[*mockAttributeContext](mockAttributeSource)

	got, err := getSetter.Getter(t.Context(), ctx)
	assert.NoError(t, err)

	m, ok := got.(pcommon.Map)
	assert.True(t, ok)

	fooValue, ok := m.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", fooValue.Str())

	bazValue, ok := m.Get("baz")
	assert.True(t, ok)
	assert.Equal(t, int64(42), bazValue.Int())
}

func TestAccessAttributes_Setter(t *testing.T) {
	dict := pprofile.NewProfilesDictionary()
	attrTable := dict.AttributeTable()
	indices := pcommon.NewInt32Slice()

	ctx := &mockAttributeContext{
		indices:    indices,
		dictionary: dict,
	}

	getSetter := AccessAttributes[*mockAttributeContext](mockAttributeSource)

	// Prepare map to set
	m := pcommon.NewMap()
	m.PutStr("alpha", "beta")
	m.PutInt("num", 123)

	err := getSetter.Setter(t.Context(), ctx, m)
	assert.NoError(t, err)

	// Check that attributes were set in the table
	foundAlpha := false
	foundNum := false
	for i := 0; i < attrTable.Len(); i++ {
		attr := attrTable.At(i)
		if attr.Key() == "alpha" {
			foundAlpha = true
			assert.Equal(t, "beta", attr.Value().Str())
		}
		if attr.Key() == "num" {
			foundNum = true
			assert.Equal(t, int64(123), attr.Value().Int())
		}
	}
	assert.True(t, foundAlpha)
	assert.True(t, foundNum)
}

func TestAccessAttributes_Setter_InvalidValue(t *testing.T) {
	dict := pprofile.NewProfilesDictionary()
	indices := pcommon.NewInt32Slice()

	ctx := &mockAttributeContext{
		indices:    indices,
		dictionary: dict,
	}

	getSetter := AccessAttributes[*mockAttributeContext](mockAttributeSource)

	// Pass a value that is not a ctxutil.Map
	err := getSetter.Setter(t.Context(), ctx, "not_a_map")
	assert.Error(t, err)
}

func TestAccessAttributesKey_Getter(t *testing.T) {
	dict := pprofile.NewProfilesDictionary()
	attrTable := dict.AttributeTable()
	attr := attrTable.AppendEmpty()
	attr.SetKey("foo")
	attr.Value().SetStr("bar")
	indices := pcommon.NewInt32Slice()
	indices.Append(0)

	ctx := &mockAttributeContext{
		indices:    indices,
		dictionary: dict,
	}

	t.Run("non-existing-key", func(t *testing.T) {
		path := pathtest.Path[*mockAttributeContext]{
			KeySlice: []ottl.Key[*mockAttributeContext]{
				&pathtest.Key[*mockAttributeContext]{
					S: ottltest.Strp("key1"),
				},
			},
		}
		getSetter := AccessAttributesKey[*mockAttributeContext](path.Keys(), mockAttributeSource)
		got, err := getSetter.Getter(t.Context(), ctx)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("existing-key", func(t *testing.T) {
		path := pathtest.Path[*mockAttributeContext]{
			KeySlice: []ottl.Key[*mockAttributeContext]{
				&pathtest.Key[*mockAttributeContext]{
					S: ottltest.Strp("foo"),
				},
			},
		}
		getSetter := AccessAttributesKey[*mockAttributeContext](path.Keys(), mockAttributeSource)
		got, err := getSetter.Getter(t.Context(), ctx)
		assert.NoError(t, err)
		assert.Equal(t, "bar", got)
	})
}

func TestAccessAttributesKey_Setter(t *testing.T) {
	dict := pprofile.NewProfilesDictionary()
	attrTable := dict.AttributeTable()
	attr := attrTable.AppendEmpty()
	attr.SetKey("foo")
	attr.Value().SetStr("bar")
	indices := pcommon.NewInt32Slice()
	indices.Append(0)

	ctx := &mockAttributeContext{
		indices:    indices,
		dictionary: dict,
	}

	t.Run("non-existing-key", func(t *testing.T) {
		path := pathtest.Path[*mockAttributeContext]{
			KeySlice: []ottl.Key[*mockAttributeContext]{
				&pathtest.Key[*mockAttributeContext]{
					S: ottltest.Strp("key1"),
				},
			},
		}
		getSetter := AccessAttributesKey[*mockAttributeContext](path.Keys(), mockAttributeSource)
		err := getSetter.Setter(t.Context(), ctx, "value1")
		assert.NoError(t, err)
	})

	t.Run("update-existing-key", func(t *testing.T) {
		path := pathtest.Path[*mockAttributeContext]{
			KeySlice: []ottl.Key[*mockAttributeContext]{
				&pathtest.Key[*mockAttributeContext]{
					S: ottltest.Strp("foo"),
				},
			},
		}
		getSetter := AccessAttributesKey[*mockAttributeContext](path.Keys(), mockAttributeSource)
		err := getSetter.Setter(t.Context(), ctx, "bazinga")
		assert.NoError(t, err)
	})

	t.Run("insert-new-key", func(t *testing.T) {
		lenAttrsBefore := ctx.AttributeIndices().Len()
		lenIndicesBefore := ctx.indices.Len()

		path := pathtest.Path[*mockAttributeContext]{
			KeySlice: []ottl.Key[*mockAttributeContext]{
				&pathtest.Key[*mockAttributeContext]{
					S: ottltest.Strp("bazinga"),
				},
			},
		}
		getSetter := AccessAttributesKey[*mockAttributeContext](path.Keys(), mockAttributeSource)
		err := getSetter.Setter(t.Context(), ctx, 42)
		assert.NoError(t, err)
		lenAttrsAfter := ctx.AttributeIndices().Len()
		lenIndicesAfter := ctx.indices.Len()

		if lenAttrsBefore+1 != lenAttrsAfter {
			t.Fatal("expected additional attribute after inserting it")
		}

		if lenIndicesBefore+1 != lenIndicesAfter {
			t.Fatal("expected additional index after inserting it")
		}
	})
}
