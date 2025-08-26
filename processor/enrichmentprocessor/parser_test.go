// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONParser_Parse(t *testing.T) {
	parser := NewJSONParser()

	testCases := []struct {
		name          string
		jsonData      string
		expectError   bool
		expectedRows  int
		expectedCols  int
		expectedIndex map[string]int
	}{
		{
			name: "valid_json_array",
			jsonData: `[
				{"service_name": "user-service", "team": "platform", "environment": "prod"},
				{"service_name": "payment-service", "team": "payments", "environment": "stage"}
			]`,
			expectError:   false,
			expectedRows:  2,
			expectedCols:  3,
			expectedIndex: map[string]int{"service_name": 0, "team": 1, "environment": 2},
		},
		{
			name: "single_object_array",
			jsonData: `[
				{"id": "123", "name": "test"}
			]`,
			expectError:   false,
			expectedRows:  1,
			expectedCols:  2,
			expectedIndex: map[string]int{"id": 0, "name": 1},
		},
		{
			name: "objects_with_missing_fields",
			jsonData: `[
				{"service_name": "user-service", "team": "platform"},
				{"service_name": "payment-service", "environment": "stage"}
			]`,
			expectError:   false,
			expectedRows:  2,
			expectedCols:  3,
			expectedIndex: map[string]int{"service_name": 0, "team": 1, "environment": 2},
		},
		{
			name: "objects_with_various_types",
			jsonData: `[
				{"name": "service1", "port": 8080, "enabled": true, "rate": 99.5},
				{"name": "service2", "port": 9090, "enabled": false, "rate": 85.2}
			]`,
			expectError:   false,
			expectedRows:  2,
			expectedCols:  4,
			expectedIndex: map[string]int{"name": 0, "port": 1, "enabled": 2, "rate": 3},
		},
		{
			name:        "empty_json_array",
			jsonData:    `[]`,
			expectError: true,
		},
		{
			name:        "invalid_json",
			jsonData:    `{invalid json}`,
			expectError: true,
		},
		{
			name:        "json_object_not_array",
			jsonData:    `{"service_name": "user-service"}`,
			expectError: true,
		},
		{
			name:        "array_of_non_objects",
			jsonData:    `["string1", "string2"]`,
			expectError: true,
		},
		{
			name:        "mixed_array_types",
			jsonData:    `[{"name": "service1"}, "string"]`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, index, err := parser.Parse([]byte(tc.jsonData))

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, data)
				assert.Nil(t, index)
			} else {
				require.NoError(t, err)
				assert.Len(t, data, tc.expectedRows)
				assert.Len(t, index, tc.expectedCols)

				// Check that all rows have correct length
				for i, row := range data {
					assert.Len(t, row, tc.expectedCols, "row %d should have %d columns", i, tc.expectedCols)
				}

				// Check index mapping contains expected keys
				for expectedKey := range tc.expectedIndex {
					_, exists := index[expectedKey]
					assert.True(t, exists, "index should contain key %s", expectedKey)
				}
			}
		})
	}
}

func TestCSVParser_Parse(t *testing.T) {
	parser := NewCSVParser()

	testCases := []struct {
		name          string
		csvData       string
		expectError   bool
		expectedRows  int
		expectedCols  int
		expectedIndex map[string]int
	}{
		{
			name: "valid_csv_with_headers",
			csvData: `service_name,team,environment
user-service,platform,prod
payment-service,payments,stage`,
			expectError:   false,
			expectedRows:  2,
			expectedCols:  3,
			expectedIndex: map[string]int{"service_name": 0, "team": 1, "environment": 2},
		},
		{
			name: "csv_with_quotes",
			csvData: `name,description
"Service A","A service with, commas"
"Service B","Another ""quoted"" service"`,
			expectError:   false,
			expectedRows:  2,
			expectedCols:  2,
			expectedIndex: map[string]int{"name": 0, "description": 1},
		},
		{
			name: "csv_with_empty_values",
			csvData: `id,name,value
1,test,
2,,empty
3,final,data`,
			expectError:   false,
			expectedRows:  3,
			expectedCols:  3,
			expectedIndex: map[string]int{"id": 0, "name": 1, "value": 2},
		},
		{
			name: "single_row_csv",
			csvData: `id,name
1,test`,
			expectError:   false,
			expectedRows:  1,
			expectedCols:  2,
			expectedIndex: map[string]int{"id": 0, "name": 1},
		},
		{
			name:        "csv_with_only_headers",
			csvData:     `id,name,value`,
			expectError: true,
		},
		{
			name:        "empty_csv",
			csvData:     ``,
			expectError: true,
		},
		{
			name:        "invalid_csv_format",
			csvData:     `id,name\nunclosed "quote field`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, index, err := parser.Parse([]byte(tc.csvData))

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, data)
				assert.Nil(t, index)
			} else {
				require.NoError(t, err)
				assert.Len(t, data, tc.expectedRows)
				assert.Len(t, index, tc.expectedCols)

				// Check that all rows have correct length
				for i, row := range data {
					assert.Len(t, row, tc.expectedCols, "row %d should have %d columns", i, tc.expectedCols)
				}

				// Check index mapping
				for expectedKey, expectedIndex := range tc.expectedIndex {
					actualIndex, exists := index[expectedKey]
					assert.True(t, exists, "index should contain key %s", expectedKey)
					assert.Equal(t, expectedIndex, actualIndex, "index for key %s should be %d", expectedKey, expectedIndex)
				}
			}
		})
	}
}

func TestGetParser(t *testing.T) {
	testCases := []struct {
		name        string
		format      string
		expectError bool
		expectType  string
	}{
		{
			name:        "json_parser",
			format:      "json",
			expectError: false,
			expectType:  "*enrichmentprocessor.JSONParser",
		},
		{
			name:        "csv_parser",
			format:      "csv",
			expectError: false,
			expectType:  "*enrichmentprocessor.CSVParser",
		},
		{
			name:        "json_uppercase",
			format:      "JSON",
			expectError: false,
			expectType:  "*enrichmentprocessor.JSONParser",
		},
		{
			name:        "csv_uppercase",
			format:      "CSV",
			expectError: false,
			expectType:  "*enrichmentprocessor.CSVParser",
		},
		{
			name:        "unsupported_format",
			format:      "xml",
			expectError: true,
		},
		{
			name:        "empty_format",
			format:      "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser, err := GetParser(tc.format)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, parser)
				assert.Contains(t, err.Error(), "unsupported format")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, parser)
				// Check type using interface assertion
				switch tc.expectType {
				case "*enrichmentprocessor.JSONParser":
					_, ok := parser.(*JSONParser)
					assert.True(t, ok, "parser should be JSONParser")
				case "*enrichmentprocessor.CSVParser":
					_, ok := parser.(*CSVParser)
					assert.True(t, ok, "parser should be CSVParser")
				}
			}
		})
	}
}

func TestParseData(t *testing.T) {
	testCases := []struct {
		name        string
		data        string
		format      string
		expectError bool
		expectedLen int
	}{
		{
			name: "parse_json_data",
			data: `[
				{"service_name": "user-service", "team": "platform"},
				{"service_name": "payment-service", "team": "payments"}
			]`,
			format:      "json",
			expectError: false,
			expectedLen: 2,
		},
		{
			name: "parse_csv_data",
			data: `service_name,team
user-service,platform
payment-service,payments`,
			format:      "csv",
			expectError: false,
			expectedLen: 2,
		},
		{
			name:        "parse_unsupported_format",
			data:        `<xml>data</xml>`,
			format:      "xml",
			expectError: true,
		},
		{
			name:        "parse_invalid_json",
			data:        `{invalid json}`,
			format:      "json",
			expectError: true,
		},
		{
			name:        "parse_invalid_csv",
			data:        `unclosed "quote`,
			format:      "csv",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, index, err := ParseData([]byte(tc.data), tc.format)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, data)
				assert.Nil(t, index)
			} else {
				require.NoError(t, err)
				assert.Len(t, data, tc.expectedLen)
				assert.NotNil(t, index)
			}
		})
	}
}

func TestJSONParser_ParseDataConsistency(t *testing.T) {
	parser := NewJSONParser()

	// Test that missing fields are handled consistently
	jsonData := `[
		{"service_name": "service1", "team": "team1", "environment": "prod"},
		{"service_name": "service2", "team": "team2"},
		{"service_name": "service3", "environment": "dev"}
	]`

	data, index, err := parser.Parse([]byte(jsonData))
	require.NoError(t, err)
	require.Len(t, data, 3)
	require.Len(t, index, 3) // service_name, team, environment

	// Check that all rows have the same length
	for _, row := range data {
		assert.Len(t, row, 3)
	}

	// Check that missing values are empty strings
	serviceNameIndex := index["service_name"]
	teamIndex := index["team"]
	envIndex := index["environment"]

	// Row 0: all fields present
	assert.Equal(t, "service1", data[0][serviceNameIndex])
	assert.Equal(t, "team1", data[0][teamIndex])
	assert.Equal(t, "prod", data[0][envIndex])

	// Row 1: environment missing
	assert.Equal(t, "service2", data[1][serviceNameIndex])
	assert.Equal(t, "team2", data[1][teamIndex])
	assert.Equal(t, "", data[1][envIndex]) // Missing field should be empty string

	// Row 2: team missing
	assert.Equal(t, "service3", data[2][serviceNameIndex])
	assert.Equal(t, "", data[2][teamIndex]) // Missing field should be empty string
	assert.Equal(t, "dev", data[2][envIndex])
}

func TestCSVParser_ParseSpecialCharacters(t *testing.T) {
	parser := NewCSVParser()

	// Test CSV with various special characters and edge cases
	csvData := `name,description,value
"Service, with comma","Description with ""quotes""",123
"Service with newline","Line 1
Line 2",456
Simple Service,Simple Description,789`

	data, index, err := parser.Parse([]byte(csvData))
	require.NoError(t, err)
	require.Len(t, data, 3)
	require.Len(t, index, 3)

	nameIndex := index["name"]
	descIndex := index["description"]
	valueIndex := index["value"]

	// Check first row with comma in field
	assert.Equal(t, "Service, with comma", data[0][nameIndex])
	assert.Equal(t, `Description with "quotes"`, data[0][descIndex])
	assert.Equal(t, "123", data[0][valueIndex])

	// Check second row with newline in field
	assert.Equal(t, "Service with newline", data[1][nameIndex])
	assert.Contains(t, data[1][descIndex], "Line 1")
	assert.Contains(t, data[1][descIndex], "Line 2")
	assert.Equal(t, "456", data[1][valueIndex])

	// Check third row (simple case)
	assert.Equal(t, "Simple Service", data[2][nameIndex])
	assert.Equal(t, "Simple Description", data[2][descIndex])
	assert.Equal(t, "789", data[2][valueIndex])
}
