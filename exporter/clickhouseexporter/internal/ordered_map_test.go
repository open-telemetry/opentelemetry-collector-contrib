// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertOrderedMapToMap(t *testing.T) {
	t.Run("Normal case with valid string keys and values", func(t *testing.T) {
		orderedMap := NewOrderedMap()
		orderedMap.Put("key1", "value1")
		orderedMap.Put("key2", "value2")

		result := ConvertOrderedMapToMap(orderedMap)

		expected := map[string]string{"key1": "value1", "key2": "value2"}
		assert.Equal(t, expected, result)
	})

	t.Run("Case with non-string keys", func(t *testing.T) {
		orderedMap := NewOrderedMap()
		orderedMap.Put(1, "value1")
		orderedMap.Put("key2", "value2")

		result := ConvertOrderedMapToMap(orderedMap)

		expected := map[string]string{"key2": "value2"}
		assert.Equal(t, expected, result)
	})

	t.Run("Case with non-string values", func(t *testing.T) {
		orderedMap := NewOrderedMap()
		orderedMap.Put("key1", 1)
		orderedMap.Put("key2", "value2")

		result := ConvertOrderedMapToMap(orderedMap)

		expected := map[string]string{"key2": "value2"}
		assert.Equal(t, expected, result)
	})

	t.Run("Empty OrderedMap", func(t *testing.T) {
		orderedMap := NewOrderedMap()

		result := ConvertOrderedMapToMap(orderedMap)

		expected := map[string]string{}
		assert.Equal(t, expected, result)
	})

	t.Run("Sort OrderedMap", func(t *testing.T) {
		orderedMap := NewOrderedMap()
		orderedMap.Put("key1", "value1")
		orderedMap.Put("key3", "value3")
		orderedMap.Put("key0", "value0")
		orderedMap.Put("key2", "value2")

		orderedMap.Sort()

		for i := range orderedMap.Keys {
			assert.Equal(t, fmt.Sprintf("key%v", i), orderedMap.Keys[i])
			assert.Equal(t, fmt.Sprintf("value%v", i), orderedMap.Values[i])
		}

	})
}
