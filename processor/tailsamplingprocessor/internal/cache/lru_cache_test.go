package cache

import (
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"testing"
)

func TestSinglePut(t *testing.T) {
	c, err := NewLRUDecisionCache(2, func(uint64, bool) {})
	require.NoError(t, err)
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	require.NoError(t, c.Put(id))
	assert.Equal(t, true, c.Get(id))
}

func TestExceedsSizeLimit(t *testing.T) {
	c, err := NewLRUDecisionCache(2, func(uint64, bool) {})
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

	require.NoError(t, c.Put(id1))
	require.NoError(t, c.Put(id2))
	require.NoError(t, c.Put(id3))

	assert.False(t, c.Get(id1)) // evicted
	assert.True(t, c.Get(id2))
	assert.True(t, c.Get(id3))
}

func TestLeastRecentlyUsedIsEvicted(t *testing.T) {
	c, err := NewLRUDecisionCache(2, func(uint64, bool) {})
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

	require.NoError(t, c.Put(id1))
	require.NoError(t, c.Put(id2))
	assert.Equal(t, true, c.Get(id1)) // use id1
	require.NoError(t, c.Put(id3))

	assert.True(t, c.Get(id1))
	assert.False(t, c.Get(id2)) // evicted
	assert.True(t, c.Get(id3))
}

func traceIDFromHex(idStr string) (pcommon.TraceID, error) {
	id := pcommon.NewTraceIDEmpty()
	_, err := hex.Decode(id[:], []byte(idStr))
	return id, err
}
