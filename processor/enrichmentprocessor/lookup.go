// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"errors"
	"strconv"
	"sync"
)

// lookup handles data lookup operations
type lookup struct {
	data              [][]string
	lookupColumnIndex map[string]map[string]int
	headerIndex       map[string]int
	mutex             sync.RWMutex
}

// newLookup creates a new lookup instance
func newLookup() *lookup {
	return &lookup{
		data:              make([][]string, 0),
		lookupColumnIndex: make(map[string]map[string]int),
		headerIndex:       make(map[string]int),
	}
}

// SetAll replaces all data in the lookup
func (l *lookup) SetAll(data [][]string, headerIndex map[string]int, indexFields []string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.data = data
	l.headerIndex = headerIndex
	l.lookupColumnIndex = make(map[string]map[string]int)

	// Iterate through all index fields
	for _, lookupField := range indexFields {
		if lookupField == "" {
			continue
		}

		lookupFieldColumnIndex, exists := headerIndex[lookupField]
		if !exists {
			continue
		}

		if l.lookupColumnIndex[lookupField] == nil {
			l.lookupColumnIndex[lookupField] = make(map[string]int)
		}

		for i, row := range data {
			if lookupFieldColumnIndex < len(row) {
				// WARNING: if column has same field, only last one will remain
				fieldValue := row[lookupFieldColumnIndex]
				l.lookupColumnIndex[lookupField][fieldValue] = i
			}
		}
	}
}

// Lookup performs a lookup operation
func (l *lookup) Lookup(headerName, value string) (enrichmentRow []string, headerIndex map[string]int, err error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Check if the header name exists in lookupColumnIndex
	headerMap, headerExists := l.lookupColumnIndex[headerName]
	if !headerExists {
		return nil, nil, errors.New("enrichment field '" + headerName + "' is not indexed")
	}

	// Check if the value exists in the header's map
	rowIndex, valueExists := headerMap[value]
	if !valueExists {
		return nil, nil, errors.New("enrichment data not found for field '" + headerName + "' with value '" + value + "'")
	}

	if rowIndex >= len(l.data) {
		return nil, nil, errors.New("enrichment data index out of range for field '" + headerName + "' with value '" + value + "' (index: " + strconv.Itoa(rowIndex) + ", data length: " + strconv.Itoa(len(l.data)) + ")")
	}

	return l.data[rowIndex], l.headerIndex, nil
}
