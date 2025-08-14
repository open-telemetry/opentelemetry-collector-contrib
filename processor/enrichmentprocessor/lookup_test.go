// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLookup(t *testing.T) {
	lookup := NewLookup()

	assert.NotNil(t, lookup)
	assert.NotNil(t, lookup.data)
	assert.NotNil(t, lookup.LookupColumnIndex)
	assert.NotNil(t, lookup.headerIndex)
}

func TestLookup_SetAll(t *testing.T) {
	lookup := NewLookup()

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

	lookup.SetAll(data, headerIndex, indexFields)

	// Verify basic fields are set
	assert.Equal(t, data, lookup.data)
	assert.Equal(t, headerIndex, lookup.headerIndex)
	assert.Equal(t, 2, len(lookup.LookupColumnIndex))

	// Verify indexing works correctly
	assert.Equal(t, 0, lookup.LookupColumnIndex["field1"]["value1"])
	assert.Equal(t, 1, lookup.LookupColumnIndex["field1"]["value2"])
	assert.Equal(t, 2, lookup.LookupColumnIndex["field1"]["value3"])
	assert.Equal(t, 0, lookup.LookupColumnIndex["field2"]["data1"])
	assert.Equal(t, 1, lookup.LookupColumnIndex["field2"]["data2"])
	assert.Equal(t, 2, lookup.LookupColumnIndex["field2"]["data3"])
}

func TestLookup_SetAll_EdgeCases(t *testing.T) {
	lookup := NewLookup()

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

	lookup.SetAll(data, headerIndex, indexFields)

	// Only valid fields should be indexed
	assert.Equal(t, 2, len(lookup.LookupColumnIndex))
	assert.Contains(t, lookup.LookupColumnIndex, "field1")
	assert.Contains(t, lookup.LookupColumnIndex, "field2")

	// field1 should have all values
	assert.Equal(t, 3, len(lookup.LookupColumnIndex["field1"]))
	// field2 should skip the short row
	assert.Equal(t, 2, len(lookup.LookupColumnIndex["field2"]))
}

func TestLookup_Lookup(t *testing.T) {
	lookup := NewLookup()

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

	lookup.SetAll(data, headerIndex, indexFields)

	ctx := context.Background()

	// Test successful lookup
	row, index, err := lookup.Lookup(ctx, "field1", "key2")
	require.NoError(t, err)
	assert.Equal(t, []string{"key2", "data2", "info2"}, row)
	assert.Equal(t, headerIndex, index)

	// Test lookup with non-existent value
	row, index, err = lookup.Lookup(ctx, "field1", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "enrichment data not found for field 'field1' with value 'nonexistent'")
	assert.Nil(t, row)
	assert.Nil(t, index)

	// Test lookup with non-indexed field
	row, index, err = lookup.Lookup(ctx, "field3", "info1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "enrichment field 'field3' is not indexed")
	assert.Nil(t, row)
	assert.Nil(t, index)
}

func BenchmarkLookup_SetAll(b *testing.B) {
	lookup := NewLookup()

	// Create dataset
	data := make([][]string, 1000)
	for i := 0; i < 1000; i++ {
		data[i] = []string{
			fmt.Sprintf("key%d", i),
			fmt.Sprintf("data%d", i),
		}
	}

	headerIndex := map[string]int{
		"field1": 0,
		"field2": 1,
	}
	indexFields := []string{"field1", "field2"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lookup.SetAll(data, headerIndex, indexFields)
	}
}

func BenchmarkLookup_Lookup(b *testing.B) {
	lookup := NewLookup()

	// Create dataset
	data := make([][]string, 1000)
	for i := 0; i < 1000; i++ {
		data[i] = []string{
			fmt.Sprintf("key%d", i),
			fmt.Sprintf("data%d", i),
		}
	}

	headerIndex := map[string]int{
		"field1": 0,
		"field2": 1,
	}
	indexFields := []string{"field1"}

	lookup.SetAll(data, headerIndex, indexFields)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = lookup.Lookup(ctx, "field1", fmt.Sprintf("key%d", i%1000))
	}
}
