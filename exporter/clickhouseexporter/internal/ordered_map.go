package internal

import (
	"sort"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
)

// OrderedMap is a simple (non thread safe) ordered map
type OrderedMap struct {
	Keys   []any
	Values []any
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{}
}

// NewOrderedMapFromMap converts a map to an ordered map
// Provided to limit the changes to the existing code so OrderedMap can introduced
// without large changes to the types that are used
//
// Parameters:
//   - mapIn: The map that will be converted into a OrderedMap.
//
// Returns:
//
//	A OrderedMap containing the same key-value pairs as the mapIn.
func NewOrderedMapFromMap(mapIn map[string]string) *OrderedMap {
	om := NewOrderedMap()
	for k, v := range mapIn {
		om.Put(k, v)
	}
	om.Sort()
	return om
}

func (om *OrderedMap) Put(key any, value any) {
	om.Keys = append(om.Keys, key)
	om.Values = append(om.Values, value)
}

func (om *OrderedMap) Iterator() column.MapIterator {
	return NewOrderedMapIterator(om)
}

// Sort sorts the OrderedMap by its keys.
func (om *OrderedMap) Sort() {
	// Create a slice of indices and sort it based on the keys
	indices := make([]int, len(om.Keys))
	for i := range indices {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		return om.Keys[indices[i]].(string) < om.Keys[indices[j]].(string)
	})

	// Create new slices for sorted keys and values
	sortedKeys := make([]any, len(om.Keys))
	sortedValues := make([]any, len(om.Values))

	for i, idx := range indices {
		sortedKeys[i] = om.Keys[idx]
		sortedValues[i] = om.Values[idx]
	}

	// Update the OrderedMap with sorted keys and values
	om.Keys = sortedKeys
	om.Values = sortedValues
}

type OrderedMapIter struct {
	om        *OrderedMap
	iterIndex int
}

func NewOrderedMapIterator(om *OrderedMap) column.MapIterator {
	return &OrderedMapIter{om: om, iterIndex: -1}
}

func (i *OrderedMapIter) Next() bool {
	i.iterIndex++
	return i.iterIndex < len(i.om.Keys)
}

func (i *OrderedMapIter) Key() any {
	return i.om.Keys[i.iterIndex]
}

func (i *OrderedMapIter) Value() any {
	return i.om.Values[i.iterIndex]
}

func ConvertOrderedMapToMap(orderedMap *OrderedMap) map[string]string {
	result := make(map[string]string)
	for i, key := range orderedMap.Keys {
		strKey, ok := key.(string)
		if !ok {
			continue // Skip if the key is not a string
		}
		strValue, ok := orderedMap.Values[i].(string)
		if !ok {
			continue // Skip if the value is not a string
		}
		result[strKey] = strValue
	}
	return result
}

// convertArraySetOrderedMapToMap converts a clickhouse.ArraySet of ordered maps to a map
// This is used for testing purposes to compare the expected and actual values
func convertArraySetOrderedMapToMap(list clickhouse.ArraySet) map[string]string {
	result := make(map[string]string)
	for _, orderedMap := range list {
		mapValue, ok := orderedMap.(column.IterableOrderedMap)
		if !ok {
			continue // Skip if the key is not a string
		}

		for iter := mapValue.Iterator(); iter.Next(); {
			strKey, ok := iter.Key().(string)
			if !ok {
				continue // Skip if the key is not a string
			}

			strValue, ok := iter.Value().(string)
			if !ok {
				continue // Skip if the value is not a string
			}
			result[strKey] = strValue
		}
	}
	return result
}
