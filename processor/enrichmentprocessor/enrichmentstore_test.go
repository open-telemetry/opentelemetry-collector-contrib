// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnrichmentStore(t *testing.T) {
	store := newEnrichmentStore()

	assert.NotNil(t, store)
	assert.NotNil(t, store.data)
	assert.NotNil(t, store.fieldIndex)
	assert.NotNil(t, store.headerIndex)
	assert.Empty(t, store.data)
	assert.Empty(t, store.fieldIndex)
	assert.Empty(t, store.headerIndex)
}

func TestEnrichmentStore_SetAll(t *testing.T) {
	store := newEnrichmentStore()

	data := [][]string{
		{"value1", "data1", "info1"},
		{"value2", "data2", "info2"},
		{"value3", "data3", "info3"},
	}
	headerIndex := map[string]int{
		"field1": 0,
		"field2": 1,
		"field3": 2,
	}
	indexFields := []string{"field1", "field2"}

	store.SetAll(data, headerIndex, indexFields)

	// Verify basic fields are set
	assert.Equal(t, data, store.data)
	assert.Equal(t, headerIndex, store.headerIndex)
	assert.Len(t, store.fieldIndex, 2)

	// Verify indexing works correctly
	assert.Equal(t, 0, store.fieldIndex["field1"]["value1"])
	assert.Equal(t, 1, store.fieldIndex["field1"]["value2"])
	assert.Equal(t, 2, store.fieldIndex["field1"]["value3"])
	assert.Equal(t, 0, store.fieldIndex["field2"]["data1"])
	assert.Equal(t, 1, store.fieldIndex["field2"]["data2"])
	assert.Equal(t, 2, store.fieldIndex["field2"]["data3"])
}

func TestEnrichmentStore_SetAll_EdgeCases(t *testing.T) {
	store := newEnrichmentStore()

	data := [][]string{
		{"value1", "data1"},
		{"value2"}, // Short row
		{"value3", "data3"},
	}
	headerIndex := map[string]int{
		"field1": 0,
		"field2": 1,
	}
	indexFields := []string{"field1", "field2", "nonexistent", ""}

	store.SetAll(data, headerIndex, indexFields)

	// Only valid fields should be indexed
	assert.Len(t, store.fieldIndex, 2)
	assert.Contains(t, store.fieldIndex, "field1")
	assert.Contains(t, store.fieldIndex, "field2")

	// field1 should have all values
	assert.Len(t, store.fieldIndex["field1"], 3)
	// field2 should skip the short row
	assert.Len(t, store.fieldIndex["field2"], 2)
}

func TestEnrichmentStore_Get(t *testing.T) {
	store := newEnrichmentStore()

	data := [][]string{
		{"key1", "data1", "info1"},
		{"key2", "data2", "info2"},
	}
	headerIndex := map[string]int{
		"field1": 0,
		"field2": 1,
		"field3": 2,
	}
	indexFields := []string{"field1", "field2"}

	store.SetAll(data, headerIndex, indexFields)

	// Test successful lookup
	row, index, err := store.Get("field1", "key2")
	require.NoError(t, err)
	assert.Equal(t, []string{"key2", "data2", "info2"}, row)
	assert.Equal(t, headerIndex, index)

	// Test lookup with non-existent value
	row, index, err = store.Get("field1", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "enrichment data not found for field 'field1' with value 'nonexistent'")
	assert.Nil(t, row)
	assert.Nil(t, index)

	// Test lookup with non-indexed field
	row, index, err = store.Get("field3", "info1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "enrichment field 'field3' is not indexed")
	assert.Nil(t, row)
	assert.Nil(t, index)
}

func TestEnrichmentStore_GetStats(t *testing.T) {
	store := newEnrichmentStore()

	// Test empty store stats
	stats := store.GetStats()
	assert.Equal(t, 0, stats.TotalRows)
	assert.Equal(t, 0, stats.IndexedFields)
	assert.Equal(t, 0, stats.TotalColumns)
	assert.Empty(t, stats.FieldIndexSize)

	// Add data and test populated stats
	data := [][]string{
		{"key1", "data1", "info1"},
		{"key2", "data2", "info2"},
		{"key3", "data1", "info3"}, // Duplicate value in field2
	}
	headerIndex := map[string]int{
		"field1": 0,
		"field2": 1,
		"field3": 2,
	}
	indexFields := []string{"field1", "field2"}

	store.SetAll(data, headerIndex, indexFields)

	stats = store.GetStats()
	assert.Equal(t, 3, stats.TotalRows)
	assert.Equal(t, 2, stats.IndexedFields)
	assert.Equal(t, 3, stats.TotalColumns)
	assert.Equal(t, 3, stats.FieldIndexSize["field1"]) // 3 unique values
	assert.Equal(t, 2, stats.FieldIndexSize["field2"]) // 2 unique values (data1 appears twice, only last index kept)
}

func TestEnrichmentStore_ThreadSafety(t *testing.T) {
	store := newEnrichmentStore()

	data := [][]string{
		{"key1", "data1"},
		{"key2", "data2"},
	}
	headerIndex := map[string]int{
		"field1": 0,
		"field2": 1,
	}
	indexFields := []string{"field1"}

	store.SetAll(data, headerIndex, indexFields)

	// Test concurrent access
	done := make(chan bool, 10)

	// Multiple readers
	for i := 0; i < 5; i++ {
		go func() {
			row, index, err := store.Get("field1", "key1")
			assert.NoError(t, err)
			assert.Equal(t, []string{"key1", "data1"}, row)
			assert.Equal(t, headerIndex, index)
			done <- true
		}()
	}

	// Multiple stats readers
	for i := 0; i < 5; i++ {
		go func() {
			stats := store.GetStats()
			assert.Equal(t, 2, stats.TotalRows)
			assert.Equal(t, 1, stats.IndexedFields)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
