// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilecommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilecommon"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
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

	getSetter := AccessAttributes[*mockAttributeContext]()

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

	getSetter := AccessAttributes[*mockAttributeContext]()

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

	getSetter := AccessAttributes[*mockAttributeContext]()

	// Pass a value that is not a ctxutil.Map
	err := getSetter.Setter(t.Context(), ctx, "not_a_map")
	assert.Error(t, err)
}
