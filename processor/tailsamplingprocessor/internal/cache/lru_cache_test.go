// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestSinglePut(t *testing.T) {
	c, err := NewLRUDecisionCache[int](2)
	require.NoError(t, err)
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	c.Put(id, 123)
	v, ok := c.Get(id)
	assert.Equal(t, 123, v)
	assert.True(t, ok)
}

func TestExceedsSizeLimit(t *testing.T) {
	c, err := NewLRUDecisionCache[bool](2)
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

	c.Put(id1, true)
	c.Put(id2, true)
	c.Put(id3, true)

	v, ok := c.Get(id1)
	assert.False(t, v)  // evicted
	assert.False(t, ok) // evicted
	v, ok = c.Get(id2)
	assert.True(t, v)
	assert.True(t, ok)
	v, ok = c.Get(id3)
	assert.True(t, v)
	assert.True(t, ok)
}

func TestLeastRecentlyUsedIsEvicted(t *testing.T) {
	c, err := NewLRUDecisionCache[bool](2)
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

	c.Put(id1, true)
	c.Put(id2, true)
	v, ok := c.Get(id1) // use id1
	assert.True(t, v)
	assert.True(t, ok)
	c.Put(id3, true)

	v, ok = c.Get(id1)
	assert.True(t, v)
	assert.True(t, ok)
	v, ok = c.Get(id2)
	assert.False(t, v)  // evicted, returns zero-value
	assert.False(t, ok) // evicted, not OK
	v, ok = c.Get(id3)
	assert.True(t, v)
	assert.True(t, ok)
}

func traceIDFromHex(idStr string) (pcommon.TraceID, error) {
	id := pcommon.NewTraceIDEmpty()
	_, err := hex.Decode(id[:], []byte(idStr))
	return id, err
}
