// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DataParser defines the interface for parsing different data formats
type DataParser interface {
	// Parse parses the given data and returns rows and column index mapping
	Parse(data []byte) ([][]string, map[string]int, error)
}

// JSONParser handles JSON data parsing
type JSONParser struct{}

// NewJSONParser creates a new JSON parser
func NewJSONParser() DataParser {
	return &JSONParser{}
}

// Parse parses JSON data and creates a lookup structure
// Expected format: Array of objects where each object represents a row
func (p *JSONParser) Parse(data []byte) ([][]string, map[string]int, error) {
	var jsonData any
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}

	dataArray, ok := jsonData.([]any)
	if !ok {
		return nil, nil, errors.New("JSON must be an array of objects")
	}

	if len(dataArray) == 0 {
		return nil, nil, errors.New("JSON array cannot be empty")
	}

	// Collect all unique keys to build header mapping
	allKeys := make(map[string]bool)
	for _, item := range dataArray {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, nil, errors.New("all items in JSON array must be objects")
		}

		for key := range itemMap {
			allKeys[key] = true
		}
	}

	// Create header index mapping
	index := make(map[string]int)
	i := 0
	for key := range allKeys {
		index[key] = i
		i++
	}

	// Convert JSON objects to row format
	var rows [][]string
	for _, item := range dataArray {
		itemMap := item.(map[string]any) // Safe cast as validated above
		row := make([]string, len(index))

		for colName, colIndex := range index {
			if value, exists := itemMap[colName]; exists {
				row[colIndex] = fmt.Sprintf("%v", value)
			} else {
				row[colIndex] = "" // Empty string for missing values
			}
		}
		rows = append(rows, row)
	}

	return rows, index, nil
}

// CSVParser handles CSV data parsing
type CSVParser struct{}

// NewCSVParser creates a new CSV parser
func NewCSVParser() DataParser {
	return &CSVParser{}
}

// Parse parses CSV data and creates a lookup structure
// Requires header row and consistent column count
func (p *CSVParser) Parse(data []byte) ([][]string, map[string]int, error) {
	csvReader := csv.NewReader(strings.NewReader(string(data)))
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, nil, errors.New("CSV file must have at least 2 rows (header + data)")
	}

	headers := records[0]
	index := make(map[string]int)

	// Build header index mapping
	for i, header := range headers {
		index[header] = i
	}

	// Return data rows (excluding header)
	dataRows := records[1:]

	return dataRows, index, nil
}

// GetParser returns the appropriate parser for the given format
func GetParser(format string) (DataParser, error) {
	switch strings.ToLower(format) {
	case "json":
		return NewJSONParser(), nil
	case "csv":
		return NewCSVParser(), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// ParseData is a convenience function that parses data using the appropriate parser
func ParseData(data []byte, format string) ([][]string, map[string]int, error) {
	parser, err := GetParser(format)
	if err != nil {
		return nil, nil, err
	}
	return parser.Parse(data)
}
