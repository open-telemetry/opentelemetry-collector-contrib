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
