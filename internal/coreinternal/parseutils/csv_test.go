// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parseutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ParseCSV(t *testing.T) {
	testCases := []struct {
		name        string
		row         string
		delimiter   rune
		lazyQuotes  bool
		expectedRow []string
		expectedErr string
	}{
		{
			name:        "Typical CSV row",
			row:         "field1,field2,field3",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1", "field2", "field3"},
		},
		{
			name:        "Quoted CSV row",
			row:         `field1,"field2,contains delimiter",field3`,
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1", "field2,contains delimiter", "field3"},
		},
		{
			name:        "Bare quote in field (strict)",
			row:         `field1,field"2,field3`,
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1"},
		},
		{
			name:        "Bare quote in field (lazy quotes)",
			row:         `field1,field"2,field3`,
			delimiter:   ',',
			lazyQuotes:  true,
			expectedRow: []string{"field1", `field"2`, "field3"},
		},
		{
			name:        "Empty row",
			row:         "",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedErr: "no csv lines found",
		},
		{
			name:        "Newlines in field",
			row:         "field1,fie\nld2,field3",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1", "fie\nld2", "field3"},
		},
		{
			name:        "Newlines prefix field",
			row:         "field1,\nfield2,field3",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1", "\nfield2", "field3"},
		},
		{
			name:        "Newlines suffix field",
			row:         "field1,field2\n,field3",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1", "field2\n", "field3"},
		},
		{
			name:        "Newlines prefix row",
			row:         "\nfield1,field2,field3",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1", "field2", "field3"},
		},
		{
			name:        "Newlines suffix row",
			row:         "field1,field2,field3\n",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"field1", "field2", "field3"},
		},
		{
			name:        "Newlines in first row",
			row:         "fiel\nd1,field2,field3",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"fiel\nd1", "field2", "field3"},
		},
		{
			name:        "Newlines in all rows",
			row:         "\nfiel\nd1,fie\nld2,fie\nld3\n",
			delimiter:   ',',
			lazyQuotes:  false,
			expectedRow: []string{"fiel\nd1", "fie\nld2", "fie\nld3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ReadCSVRow(tc.row, tc.delimiter, tc.lazyQuotes)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.Equal(t, tc.expectedRow, s)
			}
		})
	}
}

func Test_MapCSVHeaders(t *testing.T) {
	testCases := []struct {
		name        string
		headers     []string
		fields      []string
		expectedMap map[string]any
		expectedErr string
	}{
		{
			name:    "Map headers to fields",
			headers: []string{"Col1", "Col2", "Col3"},
			fields:  []string{"Val1", "Val2", "Val3"},
			expectedMap: map[string]any{
				"Col1": "Val1",
				"Col2": "Val2",
				"Col3": "Val3",
			},
		},
		{
			name:        "Missing field",
			headers:     []string{"Col1", "Col2", "Col3"},
			fields:      []string{"Val1", "Val2"},
			expectedErr: "wrong number of fields: expected 3, found 2",
		},
		{
			name:        "Too many fields",
			headers:     []string{"Col1", "Col2", "Col3"},
			fields:      []string{"Val1", "Val2", "Val3", "Val4"},
			expectedErr: "wrong number of fields: expected 3, found 4",
		},
		{
			name:    "Single field",
			headers: []string{"Col1"},
			fields:  []string{"Val1"},
			expectedMap: map[string]any{
				"Col1": "Val1",
			},
		},
		{
			name:        "No fields",
			headers:     []string{},
			fields:      []string{},
			expectedMap: map[string]any{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := MapCSVHeaders(tc.headers, tc.fields)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.Equal(t, tc.expectedMap, m)
			}
		})
	}
}
