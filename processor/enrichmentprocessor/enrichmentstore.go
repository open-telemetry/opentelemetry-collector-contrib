// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"errors"
	"strconv"
	"sync"
)

// EnrichmentStore handles data storage and lookup operations for enrichment data.
// It provides thread-safe indexing and retrieval of enrichment data.
type EnrichmentStore struct {
	data        [][]string
	fieldIndex  map[string]map[string]int
	headerIndex map[string]int
	mutex       sync.RWMutex
}

// newEnrichmentStore creates a new enrichment store instance
func newEnrichmentStore() *EnrichmentStore {
	return &EnrichmentStore{
		data:        make([][]string, 0),
		fieldIndex:  make(map[string]map[string]int),
		headerIndex: make(map[string]int),
	}
}

// SetAll replaces all data in the enrichment store with thread-safe operations
func (es *EnrichmentStore) SetAll(data [][]string, headerIndex map[string]int, indexFields []string) {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	es.data = data
	es.headerIndex = headerIndex
	es.fieldIndex = make(map[string]map[string]int)

	// Build indexes for all specified fields
	for _, field := range indexFields {
		if field == "" {
			continue
		}

		fieldColumnIndex, exists := headerIndex[field]
		if !exists {
			continue
		}

		if es.fieldIndex[field] == nil {
			es.fieldIndex[field] = make(map[string]int)
		}

		for i, row := range data {
			if fieldColumnIndex < len(row) {
				// WARNING: if column has duplicate values, only the last one will remain
				fieldValue := row[fieldColumnIndex]
				es.fieldIndex[field][fieldValue] = i
			}
		}
	}
}

// Get performs a thread-safe lookup operation and returns enrichment data
func (es *EnrichmentStore) Get(field, value string) (enrichmentRow []string, headerIndex map[string]int, err error) {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	// Check if the field is indexed
	fieldMap, fieldExists := es.fieldIndex[field]
	if !fieldExists {
		return nil, nil, errors.New("enrichment field '" + field + "' is not indexed")
	}

	// Check if the value exists in the field's index
	rowIndex, valueExists := fieldMap[value]
	if !valueExists {
		return nil, nil, errors.New("enrichment data not found for field '" + field + "' with value '" + value + "'")
	}

	// Validate row index bounds
	if rowIndex >= len(es.data) {
		return nil, nil, errors.New("enrichment data index out of range for field '" + field + "' with value '" + value + "' (index: " + strconv.Itoa(rowIndex) + ", data length: " + strconv.Itoa(len(es.data)) + ")")
	}

	return es.data[rowIndex], es.headerIndex, nil
}

// GetStats returns statistics about the enrichment store for monitoring and debugging
func (es *EnrichmentStore) GetStats() EnrichmentStoreStats {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	stats := EnrichmentStoreStats{
		TotalRows:      len(es.data),
		IndexedFields:  len(es.fieldIndex),
		TotalColumns:   len(es.headerIndex),
		FieldIndexSize: make(map[string]int),
	}

	for field, fieldMap := range es.fieldIndex {
		stats.FieldIndexSize[field] = len(fieldMap)
	}

	return stats
}

// EnrichmentStoreStats provides statistics about the enrichment store
type EnrichmentStoreStats struct {
	TotalRows      int
	IndexedFields  int
	TotalColumns   int
	FieldIndexSize map[string]int
}
