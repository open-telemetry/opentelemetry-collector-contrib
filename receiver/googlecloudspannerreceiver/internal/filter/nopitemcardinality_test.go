// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNopItemCardinalityFilter_Filter(t *testing.T) {
	filter := NewNopItemCardinalityFilter()
	sourceItems := []*Item{{}}

	filteredItems, err := filter.Filter(sourceItems)

	require.NoError(t, err)
	assert.Equal(t, sourceItems, filteredItems)
}

func TestNopItemCardinalityFilter_Shutdown(t *testing.T) {
	filter := &nopItemCardinalityFilter{}

	err := filter.Shutdown()

	require.NoError(t, err)
}

func TestNopItemCardinalityFilter_TotalLimit(t *testing.T) {
	filter := &nopItemCardinalityFilter{}

	assert.Equal(t, 0, filter.TotalLimit())
}

func TestNopItemCardinalityFilter_LimitByTimestamp(t *testing.T) {
	filter := &nopItemCardinalityFilter{}

	assert.Equal(t, 0, filter.LimitByTimestamp())
}

func TestNopItemFilterResolver_Resolve(t *testing.T) {
	itemFilterResolver := NewNopItemFilterResolver()

	itemFilter, err := itemFilterResolver.Resolve("test")

	require.NoError(t, err)
	assert.Equal(t, &nopItemCardinalityFilter{}, itemFilter)
}

func TestNopItemFilterResolver_Shutdown(t *testing.T) {
	itemFilterResolver := NewNopItemFilterResolver()

	err := itemFilterResolver.Shutdown()

	require.NoError(t, err)
}
