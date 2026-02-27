// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, err)
			p, err := profile.Parse(bytes.NewBuffer(inbytes))
			require.NoError(t, err)

			pprofile, err := ConvertPprofToProfiles(p)
			switch {
			case errors.Is(err, tc.expectedError):
				// The expected error equals the returned error,
				// so we can just continue.
			default:
				t.Fatalf("expected error '%s' but got '%s'", tc.expectedError, err)
			}
			roundTrip, err := convertPprofileToPprof(pprofile)
			require.NoError(t, err)
			if diff := cmp.Diff(p.String(), roundTrip.String()); diff != "" {
				t.Fatalf("round-trip profile mismatch (-want +got):\n%s", diff)
			}
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
		{input: "7;5;3", expectedError: errIdxFormatInvalid},
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
			expectedError: errIdxFormatInvalid,
		},
		{
			name:          "incomplete format",
			input:         "1:10:5;2:20",
			expected:      nil,
			expectedError: errIdxFormatInvalid,
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

func TestConvertMultipleSampleTypes(t *testing.T) {
	// Create a pprof profile with multiple sample types
	prof := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "inuse_objects", Unit: "count"},
			{Type: "inuse_space", Unit: "bytes"},
		},
		TimeNanos:     1000000000,
		DurationNanos: 5000000000,
		PeriodType:    &profile.ValueType{Type: "space", Unit: "bytes"},
		Period:        524288,
	}

	// Create functions
	fn1 := &profile.Function{
		ID:       1,
		Name:     "main",
		Filename: "main.go",
	}
	fn2 := &profile.Function{
		ID:       2,
		Name:     "foo",
		Filename: "foo.go",
	}

	// Create locations
	loc1 := &profile.Location{
		ID:      1,
		Address: 0x1000,
		Line: []profile.Line{
			{Function: fn1, Line: 10},
		},
	}
	loc2 := &profile.Location{
		ID:      2,
		Address: 0x2000,
		Line: []profile.Line{
			{Function: fn2, Line: 20},
		},
	}

	prof.Function = []*profile.Function{fn1, fn2}
	prof.Location = []*profile.Location{loc1, loc2}

	// Create samples with values for all four sample types
	prof.Sample = []*profile.Sample{
		{
			Location: []*profile.Location{loc1},
			Value:    []int64{5, 512, 3, 256},
		},
		{
			Location: []*profile.Location{loc2},
			Value:    []int64{10, 1024, 7, 768},
		},
		{
			Location: []*profile.Location{loc1, loc2},
			Value:    []int64{15, 2048, 12, 1536},
		},
	}

	// Convert pprof to OTel Profiles
	pprofiles, err := ConvertPprofToProfiles(prof)
	require.NoError(t, err)
	require.NotNil(t, pprofiles)

	// Verify structure
	require.Equal(t, 1, pprofiles.ResourceProfiles().Len())
	rp := pprofiles.ResourceProfiles().At(0)
	require.Equal(t, 1, rp.ScopeProfiles().Len())
	sp := rp.ScopeProfiles().At(0)

	// Verify that we have one profile per sample type
	require.Equal(t, 4, sp.Profiles().Len(), "Should have 4 profiles (one per sample type)")

	// Verify each profile
	// Note: First and last are swapped between pprof and OTel Profiles
	// (pprof's last sample type becomes OTel's first profile, and vice versa)
	for i := range 4 {
		p := sp.Profiles().At(i)

		// Verify sample type
		sampleTypeStr := pprofiles.Dictionary().StringTable().At(int(p.SampleType().TypeStrindex()))
		sampleUnitStr := pprofiles.Dictionary().StringTable().At(int(p.SampleType().UnitStrindex()))

		// Map OTel profile index to pprof sample type index (swap first and last)
		var pprofIdx int
		switch i {
		case 0:
			pprofIdx = 3 // first OTel profile is last pprof sample type
		case 3:
			pprofIdx = 0 // last OTel profile is first pprof sample type
		default:
			pprofIdx = i // middle indices stay the same
		}

		switch pprofIdx {
		case 0:
			require.Equal(t, "alloc_objects", sampleTypeStr)
			require.Equal(t, "count", sampleUnitStr)
		case 1:
			require.Equal(t, "alloc_space", sampleTypeStr)
			require.Equal(t, "bytes", sampleUnitStr)
		case 2:
			require.Equal(t, "inuse_objects", sampleTypeStr)
			require.Equal(t, "count", sampleUnitStr)
		case 3:
			require.Equal(t, "inuse_space", sampleTypeStr)
			require.Equal(t, "bytes", sampleUnitStr)
		}

		// Verify all profiles have the same number of samples
		require.Equal(t, 3, p.Samples().Len(), "Each profile should have 3 samples")

		// Verify sample values correspond to the correct sample type index
		for sampleIdx := 0; sampleIdx < p.Samples().Len(); sampleIdx++ {
			sample := p.Samples().At(sampleIdx)
			require.Equal(t, 1, sample.Values().Len(), "Each sample should have one value")
			expectedValue := prof.Sample[sampleIdx].Value[pprofIdx]
			require.Equal(t, expectedValue, sample.Values().At(0), "Sample value should match original pprof value for sample type index %d", pprofIdx)
		}

		// Verify profile metadata
		require.Equal(t, int64(1000000000), p.Time().AsTime().UnixNano())
		require.Equal(t, uint64(5000000000), p.DurationNano())
		require.Equal(t, int64(524288), p.Period())
	}

	// Verify round-trip conversion
	roundTrip, err := convertPprofileToPprof(pprofiles)
	require.NoError(t, err)
	require.NotNil(t, roundTrip)

	// Verify that round-trip profile matches the original
	require.Len(t, prof.SampleType, len(roundTrip.SampleType))
	for i, st := range prof.SampleType {
		require.Equal(t, st.Type, roundTrip.SampleType[i].Type)
		require.Equal(t, st.Unit, roundTrip.SampleType[i].Unit)
	}

	// Verify samples are correctly reconstructed
	require.Len(t, prof.Sample, len(roundTrip.Sample))
	for i, sample := range prof.Sample {
		require.Len(t, sample.Value, len(roundTrip.Sample[i].Value))
		for j, val := range sample.Value {
			require.Equal(t, val, roundTrip.Sample[i].Value[j])
		}
	}
}

func TestDefaultSampleTypeConvention(t *testing.T) {
	// Create a pprof profile with multiple sample types.
	// By convention, the last sample type (inuse_space) is the default in pprof.
	prof := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "inuse_objects", Unit: "count"},
			{Type: "inuse_space", Unit: "bytes"}, // default (last)
		},
		TimeNanos:     1000000000,
		DurationNanos: 5000000000,
		PeriodType:    &profile.ValueType{Type: "space", Unit: "bytes"},
		Period:        524288,
	}

	fn := &profile.Function{
		ID:       1,
		Name:     "testFunc",
		Filename: "test.go",
	}
	loc := &profile.Location{
		ID:      1,
		Address: 0x1000,
		Line:    []profile.Line{{Function: fn, Line: 42}},
	}

	prof.Function = []*profile.Function{fn}
	prof.Location = []*profile.Location{loc}
	prof.Sample = []*profile.Sample{
		{
			Location: []*profile.Location{loc},
			Value:    []int64{100, 200, 300, 400}, // values for each sample type
		},
	}

	// Convert pprof to OTel Profiles
	pprofiles, err := ConvertPprofToProfiles(prof)
	require.NoError(t, err)
	require.NotNil(t, pprofiles)

	sp := pprofiles.ResourceProfiles().At(0).ScopeProfiles().At(0)
	require.Equal(t, 4, sp.Profiles().Len())

	// Verify that the first OTel profile corresponds to pprof's last (default) sample type
	firstProfile := sp.Profiles().At(0)
	firstSampleTypeStr := pprofiles.Dictionary().StringTable().At(int(firstProfile.SampleType().TypeStrindex()))
	firstSampleUnitStr := pprofiles.Dictionary().StringTable().At(int(firstProfile.SampleType().UnitStrindex()))
	require.Equal(t, "inuse_space", firstSampleTypeStr, "First OTel profile should be pprof's default (last) sample type")
	require.Equal(t, "bytes", firstSampleUnitStr)
	require.Equal(t, int64(400), firstProfile.Samples().At(0).Values().At(0), "First profile should have value from last sample type")

	// Verify that the last OTel profile corresponds to pprof's first sample type
	lastProfile := sp.Profiles().At(3)
	lastSampleTypeStr := pprofiles.Dictionary().StringTable().At(int(lastProfile.SampleType().TypeStrindex()))
	lastSampleUnitStr := pprofiles.Dictionary().StringTable().At(int(lastProfile.SampleType().UnitStrindex()))
	require.Equal(t, "alloc_objects", lastSampleTypeStr, "Last OTel profile should be pprof's first sample type")
	require.Equal(t, "count", lastSampleUnitStr)
	require.Equal(t, int64(100), lastProfile.Samples().At(0).Values().At(0), "Last profile should have value from first sample type")

	// Verify middle profiles
	secondProfile := sp.Profiles().At(1)
	secondSampleTypeStr := pprofiles.Dictionary().StringTable().At(int(secondProfile.SampleType().TypeStrindex()))
	require.Equal(t, "alloc_space", secondSampleTypeStr)
	require.Equal(t, int64(200), secondProfile.Samples().At(0).Values().At(0))

	thirdProfile := sp.Profiles().At(2)
	thirdSampleTypeStr := pprofiles.Dictionary().StringTable().At(int(thirdProfile.SampleType().TypeStrindex()))
	require.Equal(t, "inuse_objects", thirdSampleTypeStr)
	require.Equal(t, int64(300), thirdProfile.Samples().At(0).Values().At(0))

	// Verify round-trip conversion preserves order
	roundTrip, err := convertPprofileToPprof(pprofiles)
	require.NoError(t, err)
	require.NotNil(t, roundTrip)

	// Original pprof order should be restored
	require.Len(t, prof.SampleType, len(roundTrip.SampleType))
	for i, st := range prof.SampleType {
		require.Equal(t, st.Type, roundTrip.SampleType[i].Type, "Round-trip should preserve original pprof sample type order")
		require.Equal(t, st.Unit, roundTrip.SampleType[i].Unit)
	}

	// Verify sample values are in original order
	require.Len(t, prof.Sample, len(roundTrip.Sample))
	for i, sample := range prof.Sample {
		require.Len(t, sample.Value, len(roundTrip.Sample[i].Value))
		for j, val := range sample.Value {
			require.Equal(t, val, roundTrip.Sample[i].Value[j], "Round-trip should preserve original sample values order")
		}
	}
}
