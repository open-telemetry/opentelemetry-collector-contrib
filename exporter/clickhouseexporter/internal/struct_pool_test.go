// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"testing"
)

// Test struct for testing the pool
type poolTestStruct struct {
	ID int
}

func TestNewExporterStructPool_Success(t *testing.T) {
	poolSize := 3
	counter := 0

	newInstance := func() (poolTestStruct, error) {
		counter++
		return poolTestStruct{ID: counter}, nil
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if pool == nil {
		t.Fatal("expected pool to be non-nil")
	}

	if cap(pool.pool) != poolSize {
		t.Errorf("expected pool capacity %d, got %d", poolSize, cap(pool.pool))
	}

	if len(pool.pool) != poolSize {
		t.Errorf("expected pool length %d, got %d", poolSize, len(pool.pool))
	}
}

func TestNewExporterStructPool_ErrorOnCreation(t *testing.T) {
	poolSize := 2

	newInstance := func() (poolTestStruct, error) {
		return poolTestStruct{}, errors.New("creation failed")
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if pool != nil {
		t.Errorf("expected pool to be nil on error, got %v", pool)
	}
}

func TestExporterStructPoolAcquire(t *testing.T) {
	poolSize := 2
	counter := 0

	newInstance := func() (poolTestStruct, error) {
		counter++
		return poolTestStruct{ID: counter}, nil
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Acquire first item
	item1 := pool.Acquire()
	if item1.ID == 0 {
		t.Error("expected non-zero ID for acquired item")
	}

	// Acquire second item
	item2 := pool.Acquire()
	if item2.ID == 0 {
		t.Error("expected non-zero ID for acquired item")
	}

	// Verify they are different instances
	if item1.ID == item2.ID {
		t.Error("expected different items from pool")
	}

	// Pool should now be empty
	if len(pool.pool) != 0 {
		t.Errorf("expected empty pool after acquiring all items, got length %d", len(pool.pool))
	}
}

func TestExporterStructPoolRelease(t *testing.T) {
	poolSize := 1

	newInstance := func() (poolTestStruct, error) {
		return poolTestStruct{ID: 42}, nil
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Acquire item
	item := pool.Acquire()

	// Pool should be empty
	if len(pool.pool) != 0 {
		t.Errorf("expected empty pool after acquire, got length %d", len(pool.pool))
	}

	// Release item back
	pool.Release(item)

	// Pool should have the item back
	if len(pool.pool) != 1 {
		t.Errorf("expected pool length 1 after release, got %d", len(pool.pool))
	}

	// Verify we can acquire the same item again
	returnedItem := pool.Acquire()
	if returnedItem.ID != item.ID {
		t.Errorf("expected same item returned, got ID %d, want %d", returnedItem.ID, item.ID)
	}
}

func TestExporterStructPoolAcquireReleaseMultiple(t *testing.T) {
	poolSize := 3
	counter := 0

	newInstance := func() (poolTestStruct, error) {
		counter++
		return poolTestStruct{ID: counter}, nil
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Acquire all items
	items := make([]poolTestStruct, poolSize)
	for i := 0; i < poolSize; i++ {
		items[i] = pool.Acquire()
	}

	// Release all items
	for _, item := range items {
		pool.Release(item)
	}

	// Pool should be full again
	if len(pool.pool) != poolSize {
		t.Errorf("expected pool length %d after releasing all items, got %d", poolSize, len(pool.pool))
	}
}

func TestExporterStructPoolDestroy(t *testing.T) {
	poolSize := 2

	newInstance := func() (poolTestStruct, error) {
		return poolTestStruct{ID: 1}, nil
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Destroy the pool
	pool.Destroy()

	// Attempting to acquire after destroy should block/panic in real usage
	// We can't easily test this without goroutines, but we can verify the channel is closed
	select {
	case _, ok := <-pool.pool:
		if ok {
			t.Error("expected channel to be closed after destroy")
		}
	default:
		// Channel is closed and empty
	}
}

func TestExporterStructPoolEmptyPool(t *testing.T) {
	poolSize := 0

	newInstance := func() (poolTestStruct, error) {
		return poolTestStruct{ID: 1}, nil
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)
	if err != nil {
		t.Fatalf("failed to create empty pool: %v", err)
	}

	if len(pool.pool) != 0 {
		t.Errorf("expected empty pool length 0, got %d", len(pool.pool))
	}

	if cap(pool.pool) != 0 {
		t.Errorf("expected empty pool capacity 0, got %d", cap(pool.pool))
	}
}

func BenchmarkPoolAcquireRelease(b *testing.B) {
	poolSize := 10

	newInstance := func() (poolTestStruct, error) {
		return poolTestStruct{ID: 1}, nil
	}

	pool, err := NewExporterStructPool(poolSize, newInstance)
	if err != nil {
		b.Fatalf("failed to create pool: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		item := pool.Acquire()
		pool.Release(item)
	}
}
