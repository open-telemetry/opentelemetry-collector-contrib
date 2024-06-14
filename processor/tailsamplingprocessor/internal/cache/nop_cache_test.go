package cache

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNopCache(t *testing.T) {
	c := NewNopDecisionCache[bool]()
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	c.Put(id, true)
	v, ok := c.Get(id)
	assert.False(t, v)
	assert.False(t, ok)
}
