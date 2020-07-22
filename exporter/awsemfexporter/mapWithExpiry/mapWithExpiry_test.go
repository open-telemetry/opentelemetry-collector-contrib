package mapWithExpiry

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"sync"
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

func TestMapWithExpiry_concurrency(t *testing.T) {
	store := NewMapWithExpiry(time.Second)
	key := "key"
	store.Set(key, 0)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for add := 0; add < 100; add++ {
			v, _ := store.Get(key)
			store.Set(key, v.(int) + 1)
		}
		wg.Done()
	}()

	go func() {
		for minus := 0; minus < 100; minus++ {
			v, _ := store.Get(key)
			store.Set(key, v.(int) - 1)
		}
		wg.Done()
	}()

	wg.Wait()
	v, _ := store.Get(key)
	assert.Equal(t, 0, v.(int))
}
