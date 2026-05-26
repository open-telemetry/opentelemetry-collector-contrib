// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

// TestConvertPprofileToPprof_SampleTypeUnit verifies that SampleType entries carry the
// unit string from p.SampleType().UnitStrindex(), not just the type.
func TestConvertPprofileToPprof_SampleTypeUnit(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()
	dict.StackTable().AppendEmpty()
	// string index 0 = "cpu" (type), index 1 = "nanoseconds" (unit)
	dict.StringTable().Append("cpu")
	dict.StringTable().Append("nanoseconds")

	p := profiles.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	p.SampleType().SetTypeStrindex(0)
	p.SampleType().SetUnitStrindex(1)
	p.Samples().AppendEmpty().Values().Append(100)

	sp := profiles.ResourceProfiles().At(0).ScopeProfiles().At(0)
	result, err := ConvertPprofileToPprof(dict, sp.Scope(), p)
	require.NoError(t, err)
	require.Len(t, result.SampleType, 1)
	assert.Equal(t, "cpu", result.SampleType[0].Type)
	assert.Equal(t, "nanoseconds", result.SampleType[0].Unit, "SampleType.Unit must be populated from UnitStrindex")
}

// TestConvertPprofileToPprof_LinkIndexZeroNotAttached verifies that a sample whose
// LinkIndex is 0 (the sentinel "not set" value) does not get span/trace labels, even
// when the link table is non-empty.
func TestConvertPprofileToPprof_LinkIndexZeroNotAttached(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()
	dict.StackTable().AppendEmpty()
	dict.StringTable().Append("cpu")

	// Add a real link at index 0 in the link table (index 0 is sentinel, so it
	// should never be used).
	link := dict.LinkTable().AppendEmpty()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	link.SetTraceID(traceID)
	link.SetSpanID(spanID)

	p := profiles.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	s := p.Samples().AppendEmpty()
	s.Values().Append(1)
	// LinkIndex defaults to 0 — do not call SetLinkIndex

	sp := profiles.ResourceProfiles().At(0).ScopeProfiles().At(0)
	result, err := ConvertPprofileToPprof(dict, sp.Scope(), p)
	require.NoError(t, err)
	require.Len(t, result.Sample, 1)
	_, hasSpanID := result.Sample[0].Label["span_id"]
	_, hasTraceID := result.Sample[0].Label["trace_id"]
	assert.False(t, hasSpanID, "span_id must not be set when LinkIndex == 0 (sentinel)")
	assert.False(t, hasTraceID, "trace_id must not be set when LinkIndex == 0 (sentinel)")
}

// TestConvertPprofileToPprof_LinkIndexNonZeroAttached verifies that a sample with a
// non-zero LinkIndex correctly picks up trace/span labels.
func TestConvertPprofileToPprof_LinkIndexNonZeroAttached(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()
	dict.StackTable().AppendEmpty()
	dict.StringTable().Append("cpu")

	// Index 0 is sentinel; add a real link at index 1.
	dict.LinkTable().AppendEmpty()         // index 0 — sentinel placeholder
	link := dict.LinkTable().AppendEmpty() // index 1
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	link.SetTraceID(traceID)
	link.SetSpanID(spanID)

	p := profiles.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	s := p.Samples().AppendEmpty()
	s.Values().Append(1)
	s.SetLinkIndex(1) // point to the real link

	sp := profiles.ResourceProfiles().At(0).ScopeProfiles().At(0)
	result, err := ConvertPprofileToPprof(dict, sp.Scope(), p)
	require.NoError(t, err)
	require.Len(t, result.Sample, 1)
	assert.Equal(t, []string{spanID.String()}, result.Sample[0].Label["span_id"])
	assert.Equal(t, []string{traceID.String()}, result.Sample[0].Label["trace_id"])
}

// TestConvertPprofileToPprof_ZeroTimeConsistentAcrossSamples verifies that when the
// profile timestamp is zero, all samples receive the same fallback time (time.Now is
// called once, not once per sample).
func TestConvertPprofileToPprof_ZeroTimeConsistentAcrossSamples(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()
	dict.StackTable().AppendEmpty()
	dict.StringTable().Append("cpu")

	p := profiles.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	for range 3 {
		s := p.Samples().AppendEmpty()
		s.Values().Append(1)
	}
	// Leave timestamp at zero to trigger the time.Now() fallback.

	sp := profiles.ResourceProfiles().At(0).ScopeProfiles().At(0)
	result, err := ConvertPprofileToPprof(dict, sp.Scope(), p)
	require.NoError(t, err)
	require.Len(t, result.Sample, 3)

	t0 := result.Sample[0].NumLabel["source.event.time"]
	for i, s := range result.Sample {
		assert.Equal(t, t0, s.NumLabel["source.event.time"],
			"sample %d has different source.event.time — time.Now() was called per sample instead of once", i)
	}
}

// TestGetAttributeStringWithPrefix_NonContiguousKeys verifies that non-contiguous or
// high-numbered comment indices do not cause an out-of-bounds panic.
func TestGetAttributeStringWithPrefix_NonContiguousKeys(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()
	// String table: index 0 = key for comment.3, index 1 = "hello", index 2 = key for comment.7, index 3 = "world"
	dict.StringTable().Append("pprof.profile.comment.3")
	dict.StringTable().Append("hello")
	dict.StringTable().Append("pprof.profile.comment.7")
	dict.StringTable().Append("world")

	attr0 := dict.AttributeTable().AppendEmpty()
	attr0.SetKeyStrindex(0) // key = "pprof.profile.comment.3"
	attr0.Value().SetStr("hello")

	attr1 := dict.AttributeTable().AppendEmpty()
	attr1.SetKeyStrindex(2) // key = "pprof.profile.comment.7"
	attr1.Value().SetStr("world")

	result, err := getAttributeStringWithPrefix(dict)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(result), 8, "result slice must accommodate index 7")
	assert.Equal(t, "hello", result[3])
	assert.Equal(t, "world", result[7])
}

func buildMinimalProfiles() (pprofile.ProfilesDictionary, pcommon.InstrumentationScope, pprofile.Profile) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()

	dict.StackTable().AppendEmpty()
	dict.StringTable().Append("cpu")

	rp := profiles.ResourceProfiles().AppendEmpty()
	sp := rp.ScopeProfiles().AppendEmpty()
	p := sp.Profiles().AppendEmpty()
	s := p.Samples().AppendEmpty()
	s.Values().Append(1)
	return dict, sp.Scope(), p
}

func TestConvertPprofileToPprof_TimeNanos(t *testing.T) {
	dict, scope, p := buildMinimalProfiles()

	ts := time.Date(2024, 6, 15, 12, 0, 0, 123456789, time.UTC)
	p.SetTime(pcommon.NewTimestampFromTime(ts))

	result, err := ConvertPprofileToPprof(dict, scope, p)
	require.NoError(t, err)

	assert.Equal(t, ts.UnixNano(), result.TimeNanos,
		"TimeNanos must be a full Unix nanosecond timestamp, not just the sub-second nanosecond component")
}

func TestConvertPprofileToPprof_ZeroTime(t *testing.T) {
	dict, scope, p := buildMinimalProfiles()

	before := time.Now().UnixNano()
	result, err := ConvertPprofileToPprof(dict, scope, p)
	after := time.Now().UnixNano()
	require.NoError(t, err)

	assert.Zero(t, result.TimeNanos)
	_ = before
	_ = after
}

func TestConvertPprofileToPprof_DurationNanos(t *testing.T) {
	dict, scope, p := buildMinimalProfiles()
	p.SetDurationNano(5_000_000_000)

	result, err := ConvertPprofileToPprof(dict, scope, p)
	require.NoError(t, err)

	assert.Equal(t, int64(5_000_000_000), result.DurationNanos)
}

// TestConvertPprofileToPprof_SampleTypeCount verifies that SampleType has exactly
// nValues entries regardless of how many samples the profile contains.
// SampleType describes the value column schema for the whole profile, not per sample.
func TestConvertPprofileToPprof_SampleTypeCount(t *testing.T) {
	tests := []struct {
		name     string
		nSamples int
		nValues  int
	}{
		{"one sample one value", 1, 1},
		{"one sample two values", 1, 2},
		{"three samples one value", 3, 1},
		{"three samples two values", 3, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profiles := pprofile.NewProfiles()
			dict := profiles.Dictionary()
			dict.StackTable().AppendEmpty()
			dict.StringTable().Append("cpu")

			p := profiles.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
			for range tc.nSamples {
				s := p.Samples().AppendEmpty()
				for range tc.nValues {
					s.Values().Append(1)
				}
			}

			sp := profiles.ResourceProfiles().At(0).ScopeProfiles().At(0)
			result, err := ConvertPprofileToPprof(dict, sp.Scope(), p)
			require.NoError(t, err)

			assert.Len(t, result.SampleType, tc.nValues,
				"SampleType length must equal the number of value columns, not nSamples × nValues")
			assert.Len(t, result.Sample, tc.nSamples)
		})
	}
}
