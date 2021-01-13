package metricsdimensioncache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDimensionCache(t *testing.T) {
	dc := NewDimensionCache()
	assert.True(t, dc.Empty())

	// Insert dimensions into the trie.
	// Excerise inserting dimensions from both the trie as well as trie nodes.
	d := dc.InsertDimensions("foo")
	d = d.InsertDimensions("bar")

	// After inserting all the dimensions, verify that no cached key is present.
	assert.False(t, d.FoundCachedDimensionKey())

	// Store a JSON-serialized key in cache.
	jsonKey := `{"a": "foo", "b": "bar"`
	d.SetCachedDimensionKey(jsonKey)

	// Simulate building the metrics dimension key again.
	d = dc.InsertDimensions([]string{"foo", "bar"}...)

	// Verify the key was found in cache.
	assert.True(t, d.FoundCachedDimensionKey())
	assert.Equal(t, jsonKey, d.GetCachedDimensionKey())
}
