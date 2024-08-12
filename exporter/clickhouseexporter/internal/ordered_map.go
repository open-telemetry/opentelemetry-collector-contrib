package internal

import (
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

func (om *OrderedMap) Put(key any, value any) {
	om.Keys = append(om.Keys, key)
	om.Values = append(om.Values, value)
}

func (om *OrderedMap) Iterator() column.MapIterator {
	return NewOrderedMapIterator(om)
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

func ConvertOArraySetToMap(list clickhouse.ArraySet) map[string]string {
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
