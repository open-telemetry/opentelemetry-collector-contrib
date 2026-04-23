// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"
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

func TestLinesToString_SortingBranches(t *testing.T) {
	tests := []struct {
		name     string
		input    []profile.Line
		expected string
	}{
		{
			name: "line/column/function sorting with nil functions",
			input: []profile.Line{
				{Line: 10, Column: 1},
				{Line: 10, Column: 1},
				{Function: &profile.Function{ID: 2, Name: "func2"}, Line: 10, Column: 1},
				{Function: &profile.Function{ID: 1, Name: "func1"}, Line: 10, Column: 1},
				{Line: 10, Column: 0},
				{Line: 9, Column: 9},
			},
			expected: "-1:9:9;-1:10:0;-1:10:1;-1:10:1;1:10:1;2:10:1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lts := initLookupTables()
			result := lts.linesToString(test.input)
			require.Equal(t, test.expected, result)
		})
	}
}

func TestGetValueHash(t *testing.T) {
	var intBytes [8]byte
	binary.LittleEndian.PutUint64(intBytes[:], uint64(42))

	var floatBytes [8]byte
	binary.LittleEndian.PutUint64(floatBytes[:], math.Float64bits(3.1415))

	intSliceHasher := xxh3.New()
	for _, v := range []int64{1, 2, 3} {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], uint64(v))
		_, _ = intSliceHasher.Write(b[:])
	}

	floatSliceHasher := xxh3.New()
	for _, v := range []float64{1.5, 2.5} {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
		_, _ = floatSliceHasher.Write(b[:])
	}

	tests := []struct {
		name     string
		value    any
		expected uint64
	}{
		{name: "string", value: "otel", expected: xxh3.HashString("otel")},
		{name: "int64", value: int64(42), expected: xxh3.Hash(intBytes[:])},
		{name: "float64", value: 3.1415, expected: xxh3.Hash(floatBytes[:])},
		{name: "bool true", value: true, expected: 1},
		{name: "bool false", value: false, expected: 0},
		{name: "int64 slice", value: []int64{1, 2, 3}, expected: intSliceHasher.Sum64()},
		{name: "float64 slice", value: []float64{1.5, 2.5}, expected: floatSliceHasher.Sum64()},
		{name: "byte slice", value: []byte("abc"), expected: xxh3.Hash([]byte("abc"))},
		{name: "default nil", value: nil, expected: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, getValueHash(test.value))
		})
	}

	require.NotEqual(t, getValueHash("stable"), getValueHash("changed"))
}

func TestGetIdxForMMAttributes(t *testing.T) {
	keyForAttrID := func(t *testing.T, lts *lookupTables, id int32) string {
		t.Helper()

		var keyIdx int32
		found := false
		for key, attrID := range lts.attributeTable {
			if attrID == id {
				keyIdx = key.keyStrIdx
				found = true
				break
			}
		}
		require.True(t, found)

		for s, idx := range lts.stringTable {
			if idx == keyIdx {
				return s
			}
		}

		t.Fatalf("string index %d not found", keyIdx)
		return ""
	}

	tests := []struct {
		name         string
		mapping      *profile.Mapping
		expectedKeys []string
	}{
		{
			name: "build id only",
			mapping: &profile.Mapping{
				BuildID: "build-1",
			},
			expectedKeys: []string{
				"process.executable.build_id.gnu",
			},
		},
		{
			name: "all has flags",
			mapping: &profile.Mapping{
				BuildID:         "build-2",
				HasFunctions:    true,
				HasFilenames:    true,
				HasLineNumbers:  true,
				HasInlineFrames: true,
			},
			expectedKeys: []string{
				"process.executable.build_id.gnu",
				"pprof.mapping.has_filenames",
				"pprof.mapping.has_functions",
				"pprof.mapping.has_inline_frames",
				"pprof.mapping.has_line_numbers",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lts := initLookupTables()
			ids := lts.getIdxForMMAttributes(test.mapping)
			require.Len(t, ids, len(test.expectedKeys))

			actualKeys := make([]string, 0, len(ids))
			for _, id := range ids {
				actualKeys = append(actualKeys, keyForAttrID(t, &lts, id))
			}

			slices.Sort(actualKeys)
			expectedKeys := slices.Clone(test.expectedKeys)
			slices.Sort(expectedKeys)
			require.Equal(t, expectedKeys, actualKeys)

			buildIDHash := getValueHash(test.mapping.BuildID)
			require.Equal(t, test.mapping.BuildID, lts.attributeHashToValue[buildIDHash])
		})
	}
}

func TestGetIdxForLocation(t *testing.T) {
	lts := initLookupTables()

	fn := &profile.Function{ID: 1, Name: "f", SystemName: "sys", Filename: "f.go", StartLine: 1}
	mapping := &profile.Mapping{
		Start:           10,
		Limit:           20,
		Offset:          1,
		File:            "bin",
		BuildID:         "build-id",
		HasFunctions:    true,
		HasFilenames:    true,
		HasLineNumbers:  true,
		HasInlineFrames: true,
	}
	location := &profile.Location{
		Address:  0x1000,
		IsFolded: true,
		Mapping:  mapping,
		Line: []profile.Line{
			{Function: fn, Line: 42, Column: 7},
		},
	}

	idx1 := lts.getIdxForLocation(location)
	idx2 := lts.getIdxForLocation(location)

	require.Equal(t, idx1, idx2)
	require.NotEqual(t, int32(0), idx1)
	require.Greater(t, len(lts.mappingTable), 1)
	require.Greater(t, len(lts.attributeTable), 1)
}

func TestConvertPprofToProfiles_LabelValidationErrors(t *testing.T) {
	baseFunction := &profile.Function{ID: 1, Name: "main", SystemName: "main", Filename: "main.go", StartLine: 1}
	baseMapping := &profile.Mapping{ID: 1, Start: 1, Limit: 2, Offset: 0, File: "bin", BuildID: "abc"}
	baseLocation := &profile.Location{
		ID:      1,
		Address: 0x1000,
		Mapping: baseMapping,
		Line:    []profile.Line{{Function: baseFunction, Line: 10, Column: 2}},
	}

	makeProfile := func(sample *profile.Sample) *profile.Profile {
		return &profile.Profile{
			SampleType: []*profile.ValueType{{Type: "cpu", Unit: "ns"}},
			Sample:     []*profile.Sample{sample},
			Function:   []*profile.Function{baseFunction},
			Location:   []*profile.Location{baseLocation},
			Mapping:    []*profile.Mapping{baseMapping},
			PeriodType: &profile.ValueType{Type: "cpu", Unit: "ns"},
			Period:     1,
		}
	}

	t.Run("string label with multiple values", func(t *testing.T) {
		sample := &profile.Sample{
			Location: []*profile.Location{baseLocation},
			Value:    []int64{10},
			Label: map[string][]string{
				"k": {"v1", "v2"},
			},
		}

		_, err := ConvertPprofToProfiles(makeProfile(sample))
		require.Error(t, err)
		require.ErrorIs(t, err, errPprofInvalid)
	})

	t.Run("numeric label with multiple values", func(t *testing.T) {
		sample := &profile.Sample{
			Location: []*profile.Location{baseLocation},
			Value:    []int64{10},
			NumLabel: map[string][]int64{
				"n": {1, 2},
			},
		}

		_, err := ConvertPprofToProfiles(makeProfile(sample))
		require.Error(t, err)
		require.ErrorIs(t, err, errPprofInvalid)
	})
}

func TestConvertPprofToProfiles_LabelAndNumUnitPaths(t *testing.T) {
	fn := &profile.Function{ID: 1, Name: "main", SystemName: "main", Filename: "main.go", StartLine: 1}
	mapping := &profile.Mapping{
		ID:             1,
		Start:          1,
		Limit:          2,
		Offset:         0,
		File:           "bin",
		BuildID:        "abc",
		HasFunctions:   true,
		HasFilenames:   true,
		HasLineNumbers: true,
	}
	location := &profile.Location{
		ID:      1,
		Address: 0x1000,
		Mapping: mapping,
		Line: []profile.Line{
			{Function: fn, Line: 10, Column: 2},
		},
	}

	prof := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: "cpu", Unit: "ns"}},
		Sample: []*profile.Sample{{
			Location: []*profile.Location{location},
			Value:    []int64{10},
			Label: map[string][]string{
				"label.with.unit": {"v"},
				"label.no.unit":   {"x"},
			},
			NumLabel: map[string][]int64{
				"num.with.unit": {7},
				"num.no.unit":   {8},
			},
			NumUnit: map[string][]string{
				"label.with.unit": {"items"},
				"num.with.unit":   {"ms"},
			},
		}},
		Function:   []*profile.Function{fn},
		Location:   []*profile.Location{location},
		Mapping:    []*profile.Mapping{mapping},
		PeriodType: &profile.ValueType{Type: "cpu", Unit: "ns"},
		Period:     1,
		DropFrames: "drop",
		KeepFrames: "keep",
		Comments:   []string{"comment"},
		DocURL:     "https://example.com",
	}

	out, err := ConvertPprofToProfiles(prof)
	require.NoError(t, err)
	require.NotNil(t, out)

	p := out.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)
	require.Equal(t, 1, p.Samples().Len())
	require.GreaterOrEqual(t, p.Samples().At(0).AttributeIndices().Len(), 4)
	require.GreaterOrEqual(t, p.AttributeIndices().Len(), 3)
}

func TestDumpLookupTables_Errors(t *testing.T) {
	t.Run("invalid mapping attribute indices", func(t *testing.T) {
		lts := initLookupTables()
		lts.mappingTable[mm{attrIdxs: "2;1"}] = 1

		dic := pprofile.NewProfiles().Dictionary()
		err := lts.dumpLookupTables(dic)
		require.Error(t, err)
		require.ErrorIs(t, err, errIdxFormatInvalid)
	})

	t.Run("invalid location lines format", func(t *testing.T) {
		lts := initLookupTables()
		lts.locationTable[loc{lines: "invalid"}] = 1

		dic := pprofile.NewProfiles().Dictionary()
		err := lts.dumpLookupTables(dic)
		require.Error(t, err)
		require.ErrorIs(t, err, errIdxFormatInvalid)
	})

	t.Run("missing attribute value for non-zero attribute", func(t *testing.T) {
		lts := initLookupTables()
		keyIdx := lts.getIdxForString("custom.key")
		lts.attributeTable[attr{keyStrIdx: keyIdx, valueHash: 12345, unitStrIdx: noAttrUnit}] = 1

		dic := pprofile.NewProfiles().Dictionary()
		err := lts.dumpLookupTables(dic)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid attribute")
	})
}

func TestAllElementsSame(t *testing.T) {
	require.False(t, allElementsSame([]int{}))
	require.True(t, allElementsSame([]int{3, 3, 3}))
	require.False(t, allElementsSame([]int{1, 2, 1}))
}

func TestGetAttributeString(t *testing.T) {
	dic := pprofile.NewProfiles().Dictionary()
	dic.StringTable().Append("")
	dic.StringTable().Append("key.a")

	attr := dic.AttributeTable().AppendEmpty()
	attr.SetKeyStrindex(1)
	attr.Value().SetStr("value-a")

	v, err := getAttributeString(dic, "key.a")
	require.NoError(t, err)
	require.Equal(t, "value-a", v)

	_, err = getAttributeString(dic, "missing")
	require.Error(t, err)
	require.ErrorIs(t, err, errNotFound)
}

func TestGetAttributeStringWithPrefix(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		dic := pprofile.NewProfiles().Dictionary()
		dic.StringTable().Append("")
		dic.StringTable().Append("pprof.profile.comment" + ".1")
		dic.StringTable().Append("pprof.profile.comment" + ".0")

		attr1 := dic.AttributeTable().AppendEmpty()
		attr1.SetKeyStrindex(1)
		attr1.Value().SetStr("comment-1")

		attr0 := dic.AttributeTable().AppendEmpty()
		attr0.SetKeyStrindex(2)
		attr0.Value().SetStr("comment-0")

		got, err := getAttributeStringWithPrefix(dic)
		require.NoError(t, err)
		require.Equal(t, []string{"comment-0", "comment-1"}, got)
	})

	t.Run("invalid index", func(t *testing.T) {
		dic := pprofile.NewProfiles().Dictionary()
		dic.StringTable().Append("")
		dic.StringTable().Append("pprof.profile.comment" + ".x")

		attr := dic.AttributeTable().AppendEmpty()
		attr.SetKeyStrindex(1)
		attr.Value().SetStr("comment")

		_, err := getAttributeStringWithPrefix(dic)
		require.Error(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		dic := pprofile.NewProfiles().Dictionary()
		dic.StringTable().Append("")

		_, err := getAttributeStringWithPrefix(dic)
		require.Error(t, err)
		require.ErrorIs(t, err, errNotFound)
	})
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

func generatePprofTestdata(t *testing.T, typ string) *profile.Profile {
	t.Helper()

	pprofile := pprof.Lookup(typ)
	if pprofile == nil {
		t.Fatalf("%s is not a valid pprof profile", typ)
	}

	// Generate some "load" for pprof
	prev, current := 0, 1
	for i := 2; i <= 42; i++ {
		next := prev + current
		prev = current
		current = next
	}

	t.Logf("Fibonacci for 42 is %d", current)

	tmpFile, err := os.CreateTemp(t.TempDir(), typ)
	require.NoError(t, err)

	t.Cleanup(func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	})

	err = pprofile.WriteTo(tmpFile, 0)
	require.NoError(t, err)

	_, err = tmpFile.Seek(0, 0)
	require.NoError(t, err)

	output, err := profile.Parse(tmpFile)
	require.NoError(t, err)

	return output
}

func TestConvertPprofToProfiles_AllTypes(t *testing.T) {
	// cpu profiles are covered with TestConvertPprofToPprofile and
	// the original testdata from pprof.
	for _, typ := range []string{"goroutine", "allocs", "heap", "threadcreate", "block", "mutex"} {
		t.Run(typ, func(t *testing.T) {
			pprofile := generatePprofTestdata(t, typ)
			pprofiles, err := ConvertPprofToProfiles(pprofile)
			if err != nil {
				t.Fatal(err)
			}
			_ = pprofiles
		})
	}
}
