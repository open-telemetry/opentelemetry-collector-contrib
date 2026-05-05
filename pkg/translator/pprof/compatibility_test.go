// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

// TestRoundTripConversion tests that pprof -> OTel Profiles -> pprof
// produces a profile that matches the original.
func TestRoundTripConversion(t *testing.T) {
	t.Run("manual profile with multiple sample types", func(t *testing.T) {
		// Create a manually constructed pprof profile with three sample types
		prof := createManualProfile()

		// Convert to OTel Profiles
		pprofiles, err := ConvertPprofToProfiles(prof)
		require.NoError(t, err)

		// Convert back to pprof
		roundTrip, err := ConvertPprofileToPprof(pprofiles)
		require.NoError(t, err)

		// Compare the profiles
		if diff := cmp.Diff(prof.String(), roundTrip.String()); diff != "" {
			t.Errorf("round-trip profile mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("gobench.cpu test data", func(t *testing.T) {
		// Load gobench.cpu test data
		inbytes, err := os.ReadFile(filepath.Join("internal/testdata/", "gobench.cpu"))
		require.NoError(t, err)

		prof, err := profile.Parse(bytes.NewBuffer(inbytes))
		require.NoError(t, err)

		// Convert to OTel Profiles
		pprofiles, err := ConvertPprofToProfiles(prof)
		require.NoError(t, err)

		// Convert back to pprof
		roundTrip, err := ConvertPprofileToPprof(pprofiles)
		require.NoError(t, err)

		// Compare the profiles
		if diff := cmp.Diff(prof.String(), roundTrip.String()); diff != "" {
			t.Errorf("round-trip profile mismatch (-want +got):\n%s", diff)
		}
	})
}

// createManualProfile creates a  pprof profile with at least three different sample types.
func createManualProfile() *profile.Profile {
	// Create functions
	fn1 := &profile.Function{
		ID:         1,
		Name:       "main",
		SystemName: "main",
		Filename:   "main.go",
		StartLine:  1,
	}
	fn2 := &profile.Function{
		ID:         2,
		Name:       "helper",
		SystemName: "helper",
		Filename:   "helper.go",
		StartLine:  10,
	}
	fn3 := &profile.Function{
		ID:         3,
		Name:       "worker",
		SystemName: "worker",
		Filename:   "worker.go",
		StartLine:  5,
	}

	// Create mappings with BuildID and flags to test round-trip compatibility
	mapping := &profile.Mapping{
		ID:              1,
		Start:           0x1000,
		Limit:           0x5000,
		Offset:          0,
		File:            "testbinary",
		BuildID:         "test-build-id-123",
		HasFunctions:    true,
		HasFilenames:    true,
		HasLineNumbers:  true,
		HasInlineFrames: false,
	}

	// Create locations with IsFolded to test round-trip compatibility
	loc1 := &profile.Location{
		ID:       1,
		Address:  0x1000,
		Mapping:  mapping,
		IsFolded: true,
		Line: []profile.Line{
			{Function: fn1, Line: 42, Column: 5},
		},
	}
	loc2 := &profile.Location{
		ID:       2,
		Address:  0x2000,
		Mapping:  mapping,
		IsFolded: false,
		Line: []profile.Line{
			{Function: fn2, Line: 20, Column: 10},
		},
	}
	loc3 := &profile.Location{
		ID:       3,
		Address:  0x3000,
		Mapping:  mapping,
		IsFolded: true,
		Line: []profile.Line{
			{Function: fn3, Line: 15, Column: 7},
		},
	}

	// Create samples with three different sample types:
	// 1. CPU time (nanoseconds)
	// 2. Allocated memory (bytes)
	// 3. Number of allocations (count)
	samples := []*profile.Sample{
		{
			Location: []*profile.Location{loc1, loc2},
			Value:    []int64{1000000, 512000, 10}, // cpu time, alloc bytes, alloc count
		},
		{
			Location: []*profile.Location{loc2, loc3},
			Value:    []int64{2000000, 1024000, 20},
		},
		{
			Location: []*profile.Location{loc1, loc3},
			Value:    []int64{1500000, 768000, 15},
		},
	}

	// Create the profile with three sample types and profile-level attributes
	return &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "cpu", Unit: "nanoseconds"},
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "alloc_objects", Unit: "count"},
		},
		Sample:     samples,
		Function:   []*profile.Function{fn1, fn2, fn3},
		Location:   []*profile.Location{loc1, loc2, loc3},
		Mapping:    []*profile.Mapping{mapping},
		PeriodType: &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:     10000000, // 10ms
		DropFrames: "drop_func",
		KeepFrames: "keep_func",
		Comments:   []string{"Test comment 1", "Test comment 2"},
		DocURL:     "https://example.com/profile",
	}
}
