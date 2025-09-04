package lrucache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCache_OldestItemIsRemoved(t *testing.T) {
	capacity := 10
	c := New(WithCapacity(capacity))

	for i := 0; i < capacity; i++ {
		c.Add(i, i)
	}

	// ensure that all entries can be retrieved
	for i := 0; i < capacity; i++ {
		value, ok := c.Get(i)

		require.True(t, ok)
		require.Equal(t, i, value)
	}

	// add one more element and verify that the oldest can not be retrieved afterward
	c.Add(capacity+1, capacity+1)

	value, ok := c.Get(0)
	require.False(t, ok)
	require.Nil(t, value)

	value, ok = c.Get(capacity + 1)
	require.True(t, ok)
	require.Equal(t, capacity+1, value)
}

func TestCache_UpdateItem(t *testing.T) {
	c := New(WithCapacity(2))

	c.Add("a", "a")
	c.Add("b", "b")

	// update item
	c.Add("a", "aa")
	val, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, "aa", val)

	// add another item to force eviction of oldest item
	c.Add("c", "c")

	// item "b" should not be accessible because "a" has been accessed in the meantime
	_, ok = c.Get("b")
	require.False(t, ok)
}
