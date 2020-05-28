package mapWithExpiry

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"testing"
	"time"
)

func TestMapWithExpiry_add(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set("key1", "value1")
	val, ok := store.Get("key1")
	assert.Equal(t, true, ok)
	assert.Equal(t, "value1", val.(string))

	val, ok = store.Get("key2")
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, val)
}

func TestMapWithExpiry_cleanup(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	store.Set("key1", "value1")

	store.CleanUp(time.Now())
	val, ok := store.Get("key1")
	assert.Equal(t, true, ok)
	assert.Equal(t, "value1", val.(string))
	assert.Equal(t, 1, store.Size())

	time.Sleep(time.Second)
	store.CleanUp(time.Now())
	val, ok = store.Get("key1")
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, val)
	assert.Equal(t, 0, store.Size())
}
