// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNopCache(t *testing.T) {
	c := NewNopDecisionCache[bool]()
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	ok := c.Put(id, true)
	assert.False(t, ok)
	v, ok := c.Get(id)
	assert.False(t, v)
	assert.False(t, ok)
}
