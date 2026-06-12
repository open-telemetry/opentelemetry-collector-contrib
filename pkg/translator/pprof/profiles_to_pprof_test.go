// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"

import (
	"testing"

	"github.com/open-telemetry/sig-profiling/profcheck"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
	otlpprofiles "go.opentelemetry.io/proto/otlp/profiles/v1development"
	"google.golang.org/protobuf/proto"
)

// buildMinimalProfiles constructs the smallest valid ProfilesData, verifies it
// with profcheck.ConformanceChecker, and converts it to pprofile.Profiles for
// use with ConvertPprofileToPprof.
//
// The dictionary has one zero-value sentinel at index 0 and one real entry at
// index 1 for each table. The caller controls the sample's Values and
// TimestampsUnixNano. When timestamps are provided, DurationNano is set to
// max(timestamps)+1 so the checker's timestamp-range check passes.
func buildMinimalProfiles(t *testing.T, values []int64, timestamps []uint64) *pprofile.Profiles {
	t.Helper()

	dict := &otlpprofiles.ProfilesDictionary{
		// String table: 0 = "", 1 = "cpu", 2 = "nanoseconds"
		StringTable:    []string{"", "cpu", "nanoseconds"},
		MappingTable:   []*otlpprofiles.Mapping{{}},         // 0: zero-value sentinel
		AttributeTable: []*otlpprofiles.KeyValueAndUnit{{}}, // 0: zero-value sentinel
		LinkTable:      []*otlpprofiles.Link{{}},            // 0: zero-value sentinel
		FunctionTable: []*otlpprofiles.Function{
			{}, // 0: zero-value sentinel
			{NameStrindex: 1, SystemNameStrindex: 1, FilenameStrindex: 1}, // 1: "cpu"
		},
		LocationTable: []*otlpprofiles.Location{
			{}, // 0: zero-value sentinel
			{Lines: []*otlpprofiles.Line{{FunctionIndex: 1}}}, // 1: references function 1
		},
		StackTable: []*otlpprofiles.Stack{
			{},                            // 0: zero-value sentinel
			{LocationIndices: []int32{1}}, // 1: references location 1
		},
	}

	var durationNano uint64
	for _, ts := range timestamps {
		if ts+1 > durationNano {
			durationNano = ts + 1
		}
	}

	data := &otlpprofiles.ProfilesData{
		Dictionary: dict,
		ResourceProfiles: []*otlpprofiles.ResourceProfiles{{
			ScopeProfiles: []*otlpprofiles.ScopeProfiles{{
				Profiles: []*otlpprofiles.Profile{{
					SampleType:   &otlpprofiles.ValueType{TypeStrindex: 1, UnitStrindex: 2},
					PeriodType:   &otlpprofiles.ValueType{TypeStrindex: 1, UnitStrindex: 2},
					Period:       1,
					DurationNano: durationNano,
					Samples: []*otlpprofiles.Sample{{
						StackIndex:         1,
						Values:             values,
						TimestampsUnixNano: timestamps,
					}},
				}},
			}},
		}},
	}

	checker := profcheck.ConformanceChecker{
		CheckSampleTimestampShape: true,
		CheckDictionaryDuplicates: true,
	}
	require.NoError(t, checker.Check(data))

	b, err := proto.Marshal(data)
	require.NoError(t, err)
	p, err := (&pprofile.ProtoUnmarshaler{}).UnmarshalProfiles(b)
	require.NoError(t, err)
	return &p
}

// TestSampleValueTimestampShapes exercises the three shapes that the OTel
// profiles proto spec permits for Sample.values / Sample.timestamps_unix_nano
//
//   - Shape 1: timestamps only – values is empty; the count of timestamps is emitted as a single aggregated value.
//   - Shape 2: single aggregate value – one entry in values, timestamps is empty.
//   - Shape 3: per-observation – values and timestamps have the same non-zero length;
//     values[i] and timestamps[i] describe the same event.
func TestSampleValueTimestampShapes(t *testing.T) {
	t.Run("shape 1: timestamps only", func(t *testing.T) {
		profiles := buildMinimalProfiles(t, nil, []uint64{1_000_000_000, 2_000_000_000, 3_000_000_000})

		result, err := ConvertPprofileToPprof(profiles)

		require.NoError(t, err)
		require.NoError(t, result.CheckValid())
		require.Len(t, result.Sample, 1)
		require.Equal(t, []int64{3}, result.Sample[0].Value)
	})

	t.Run("shape 2: single aggregate value", func(t *testing.T) {
		profiles := buildMinimalProfiles(t, []int64{42}, nil)

		result, err := ConvertPprofileToPprof(profiles)

		require.NoError(t, err)
		require.NoError(t, result.CheckValid())
		require.Len(t, result.Sample, 1)
		require.Equal(t, []int64{42}, result.Sample[0].Value)
	})

	t.Run("shape 3: per-observation values and timestamps", func(t *testing.T) {
		profiles := buildMinimalProfiles(t,
			[]int64{10, 20, 30},
			[]uint64{1_000_000_000, 2_000_000_000, 3_000_000_000},
		)

		result, err := ConvertPprofileToPprof(profiles)

		require.NoError(t, err)
		require.NoError(t, result.CheckValid())
		require.Len(t, result.Sample, 3)
		require.Equal(t, []int64{10}, result.Sample[0].Value)
		require.Equal(t, []int64{20}, result.Sample[1].Value)
		require.Equal(t, []int64{30}, result.Sample[2].Value)
	})

	t.Run("invalid: multiple values without timestamps", func(t *testing.T) {
		// nValues > 1 with nTimestamps == 0 does not match any valid shape and
		// must be rejected. Build the proto directly to bypass the conformance
		// checker, which would otherwise prevent this invalid state from being
		// constructed.
		dict := &otlpprofiles.ProfilesDictionary{
			StringTable:  []string{"", "cpu", "nanoseconds"},
			MappingTable: []*otlpprofiles.Mapping{{}},
			FunctionTable: []*otlpprofiles.Function{
				{},
				{NameStrindex: 1, SystemNameStrindex: 1, FilenameStrindex: 1},
			},
			LocationTable: []*otlpprofiles.Location{
				{},
				{Lines: []*otlpprofiles.Line{{FunctionIndex: 1}}},
			},
			StackTable: []*otlpprofiles.Stack{
				{},
				{LocationIndices: []int32{1}},
			},
		}
		data := &otlpprofiles.ProfilesData{
			Dictionary: dict,
			ResourceProfiles: []*otlpprofiles.ResourceProfiles{{
				ScopeProfiles: []*otlpprofiles.ScopeProfiles{{
					Profiles: []*otlpprofiles.Profile{{
						SampleType: &otlpprofiles.ValueType{TypeStrindex: 1, UnitStrindex: 2},
						PeriodType: &otlpprofiles.ValueType{TypeStrindex: 1, UnitStrindex: 2},
						Period:     1,
						Samples: []*otlpprofiles.Sample{{
							StackIndex: 1,
							Values:     []int64{1, 2}, // two values, no timestamps: invalid
						}},
					}},
				}},
			}},
		}
		b, err := proto.Marshal(data)
		require.NoError(t, err)
		p, err := (&pprofile.ProtoUnmarshaler{}).UnmarshalProfiles(b)
		require.NoError(t, err)

		_, err = ConvertPprofileToPprof(&p)

		require.ErrorContains(t, err, "invalid sample")
	})
}
