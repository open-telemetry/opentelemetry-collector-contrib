// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopFieldGet(t *testing.T) {
	entry := &Entry{}
	noopField := NewNoopField()
	value, ok := noopField.Get(entry)
	require.True(t, ok)
	require.Nil(t, value)
}

func TestNoopFieldSet(t *testing.T) {
	entry := &Entry{}
	noopField := NewNoopField()
	err := noopField.Set(entry, "value")
	require.NoError(t, err)
	require.Equal(t, Entry{}, *entry)
}

func TestNoopFieldDelete(t *testing.T) {
	entry := &Entry{}
	noopField := NewNoopField()
	value, ok := noopField.Delete(entry)
	require.True(t, ok)
	require.Nil(t, value)
	require.Equal(t, Entry{}, *entry)
}

func TestNoopFieldString(t *testing.T) {
	noopField := NewNoopField()
	require.Equal(t, "$nil", noopField.String())
}

func TestNewNilField(t *testing.T) {
	nilField := NewNilField()
	require.Equal(t, Field{}, nilField)
}
