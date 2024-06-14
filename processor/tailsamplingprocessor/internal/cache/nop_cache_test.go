package cache

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNopCache(t *testing.T) {
	c := NewNopDecisionCache()
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	require.NoError(t, c.Put(id))
	assert.False(t, c.Get(id))
}
