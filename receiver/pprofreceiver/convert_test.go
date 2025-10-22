// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func TestConvertPprofToPprofile(t *testing.T) {
	tests := map[string]struct {
		expectedError error
	}{
		"cppbench.cpu": {},
		"gobench.cpu":  {},
		"java.cpu":     {},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inbytes, err := os.ReadFile(filepath.Join("internal/testdata/", name))
			if err != nil {
				t.Fatal(err)
			}
			p, err := profile.Parse(bytes.NewBuffer(inbytes))
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			pprofile, err := convertPprofToPprofile(p)
			switch {
			case errors.Is(err, tc.expectedError):
				// The expected error equals the returned error,
				// so we can just continue.
			default:
				t.Fatalf("expected error '%s' but got '%s'", tc.expectedError, err)
			}
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}
			_ = pprofile
		})
	}
}

func TestAttrIdxToString(t *testing.T) {
	for _, tc := range []struct {
		input    []int32
		expected string
	}{
		{input: []int32{}, expected: ""},
		{input: []int32{42}, expected: "42"},
		{input: []int32{2, 3, 5, 7}, expected: "2;3;5;7"},
		{input: []int32{97, 73, 89, 79, 83}, expected: "73;79;83;89;97"},
	} {
		t.Run(tc.expected, func(t *testing.T) {
			output := attrIdxToString(tc.input)
			if output != tc.expected {
				t.Fatalf("Expected '%s' but got '%s'", tc.expected, output)
			}
		})
	}
}

func TestStringToAttrIdx(t *testing.T) {
	for _, tc := range []struct {
		input         string
		output        []int32
		expectedError error
	}{
		{input: "", output: []int32{}},
		{input: "73", output: []int32{73}},
		{input: "3;5;7", output: []int32{3, 5, 7}},
		{input: "7;5;3", expectedError: errInalIdxFomrat},
		{input: "invalid", expectedError: strconv.ErrSyntax},
	} {
		t.Run(tc.input, func(t *testing.T) {
			output, err := stringToAttrIdx(tc.input)
			if !errors.Is(err, tc.expectedError) {
				t.Fatalf("Expected '%v' but got '%v'", tc.expectedError, err)
			}
			if !slices.Equal(tc.output, output) {
				t.Fatalf("Expected '%v' but got '%v'", tc.output, output)
			}
		})
	}
}

func TestStringToLine(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expected      []pprofile.Line
		expectedError error
	}{
		{
			name:  "valid multiple lines",
			input: "1:10:5;2:20:10",
			expected: []pprofile.Line{
				func() pprofile.Line {
					l := pprofile.NewLine()
					l.SetFunctionIndex(1)
					l.SetLine(10)
					l.SetColumn(5)
					return l
				}(),
				func() pprofile.Line {
					l := pprofile.NewLine()
					l.SetFunctionIndex(2)
					l.SetLine(20)
					l.SetColumn(10)
					return l
				}(),
			},
			expectedError: nil,
		},
		{
			name:          "empty string",
			input:         "",
			expected:      []pprofile.Line{},
			expectedError: nil,
		},
		{
			name:          "invalid format",
			input:         "invalid",
			expected:      nil,
			expectedError: errInalIdxFomrat,
		},
		{
			name:          "incomplete format",
			input:         "1:10:5;2:20",
			expected:      nil,
			expectedError: errInalIdxFomrat,
		},
		{
			name:          "invalid function ID",
			input:         "abc:10:5",
			expected:      nil,
			expectedError: strconv.ErrSyntax,
		},
		{
			name:          "invalid line number",
			input:         "1:abc:5",
			expected:      nil,
			expectedError: strconv.ErrSyntax,
		},
		{
			name:          "invalid column",
			input:         "1:10:abc",
			expected:      nil,
			expectedError: strconv.ErrSyntax,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := stringToLine(test.input)
			if !errors.Is(err, test.expectedError) {
				t.Errorf("stringToLine(%q) error = %v, wantErr %v", test.input, err, test.expectedError)
				return
			}
			if len(result) != len(test.expected) {
				t.Errorf("stringToLine(%q) length = %d, want %d", test.input, len(result), len(test.expected))
				return
			}
			for i := range result {
				if result[i].FunctionIndex() != test.expected[i].FunctionIndex() ||
					result[i].Line() != test.expected[i].Line() ||
					result[i].Column() != test.expected[i].Column() {
					t.Errorf("stringToLine(%q)[%d] = {FunctionIndex: %d, Line: %d, Column: %d}, want {FunctionIndex: %d, Line: %d, Column: %d}",
						test.input, i,
						result[i].FunctionIndex(), result[i].Line(), result[i].Column(),
						test.expected[i].FunctionIndex(), test.expected[i].Line(), test.expected[i].Column())
				}
			}
		})
	}
}

func TestLinesToString(t *testing.T) {
	tests := []struct {
		name     string
		input    []profile.Line
		expected string
	}{
		{
			name: "single line with function",
			input: []profile.Line{
				{Function: &profile.Function{Name: "func1"}, Line: 10, Column: 5},
			},
			expected: "1:10:5",
		},
		{
			name:     "empty lines",
			input:    []profile.Line{},
			expected: "",
		},
		{
			name: "line without function",
			input: []profile.Line{
				{Line: 20, Column: 3},
			},
			expected: "-1:20:3",
		},
		{
			name: "multiple lines",
			input: []profile.Line{
				{Function: &profile.Function{Name: "func1"}, Line: 10, Column: 5},
				{Function: &profile.Function{Name: "func2"}, Line: 20, Column: 10},
			},
			expected: "1:10:5;2:20:10",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lts := initLookupTables()
			result := lts.linesToString(test.input)
			if result != test.expected {
				t.Errorf("linesToString(%v) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}
