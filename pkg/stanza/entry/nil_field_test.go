// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNilFieldGet(t *testing.T) {
	entry := &Entry{}
	nilField := NewNilField()
	value, ok := nilField.Get(entry)
	require.True(t, ok)
	require.Nil(t, value)
}

func TestNilFieldSet(t *testing.T) {
	entry := &Entry{}
	nilField := NewNilField()
	err := nilField.Set(entry, "value")
	require.NoError(t, err)
	require.Equal(t, *entry, Entry{})
}

func TestNilFieldDelete(t *testing.T) {
	entry := &Entry{}
	nilField := NewNilField()
	value, ok := nilField.Delete(entry)
	require.True(t, ok)
	require.Nil(t, value)
	require.Equal(t, *entry, Entry{})
}

func TestNilFieldString(t *testing.T) {
	nilField := NewNilField()
	require.Equal(t, "$nil", nilField.String())
}
