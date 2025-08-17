// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
	"fmt"
	"sync"
)

// Lookup handles data lookup operations
type Lookup struct {
	data              [][]string
	LookupColumnIndex map[string]map[string]int
	headerIndex       map[string]int
	mutex             sync.RWMutex
}

// NewLookup creates a new Lookup instance
func NewLookup() *Lookup {
	return &Lookup{
		data:              make([][]string, 0),
		LookupColumnIndex: make(map[string]map[string]int),
		headerIndex:       make(map[string]int),
	}
}

// SetAll replaces all data in the lookup
func (l *Lookup) SetAll(data [][]string, headerIndex map[string]int, indexFields []string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.data = data
	l.headerIndex = headerIndex
	l.LookupColumnIndex = make(map[string]map[string]int)

	// Iterate through all index fields
	for _, lookupField := range indexFields {
		if lookupField == "" {
			continue
		}

		lookupFieldColumnIndex, exists := headerIndex[lookupField]
		if !exists {
			continue
		}

		if l.LookupColumnIndex[lookupField] == nil {
			l.LookupColumnIndex[lookupField] = make(map[string]int)
		}

		for i, row := range data {
			if lookupFieldColumnIndex < len(row) {
				// WARNING: if column has same field, only last one will remain
				fieldValue := row[lookupFieldColumnIndex]
				l.LookupColumnIndex[lookupField][fieldValue] = i
			}
		}
	}
}

// Lookup performs a lookup for the given key and returns matched row and index
func (l *Lookup) Lookup(ctx context.Context, headerName, value string) (enrichmentRow []string, headerIndex map[string]int, err error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Check if the header name exists in LookupColumnIndex
	headerMap, headerExists := l.LookupColumnIndex[headerName]
	if !headerExists {
		return nil, nil, fmt.Errorf("enrichment field '%s' is not indexed", headerName)
	}

	// Check if the value exists in the header's map
	rowIndex, valueExists := headerMap[value]
	if !valueExists {
		return nil, nil, fmt.Errorf("enrichment data not found for field '%s' with value '%s'", headerName, value)
	}

	if rowIndex >= len(l.data) {
		return nil, nil, fmt.Errorf("enrichment data index out of range for field '%s' with value '%s' (index: %d, data length: %d)", headerName, value, rowIndex, len(l.data))
	}

	return l.data[rowIndex], l.headerIndex, nil
}
