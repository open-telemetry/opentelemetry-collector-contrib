package lru

import (
	"testing"
)

func TestLRU(t *testing.T) {
	cache, err := New(2)
	if err != nil {
		t.Fatalf("Error creating cache: %v", err)
	}

	// Test cases
	cache.Add("key1", "value1")
	cache.Add("key2", "value2")

	value, ok := cache.Get("key1")
	if !ok || value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	cache.Add("key3", "value3")

	_, ok = cache.Get("key4")
	if ok {
		t.Errorf("Expected key1 to be evicted")
	}
}
