// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func TestAddProfilingPprofSampleLabels(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()
	dict.StringTable().Append("cpu")
	dict.LinkTable().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	link := dict.LinkTable().AppendEmpty()
	link.SetTraceID(traceID)
	link.SetSpanID(spanID)

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("runtime/profiler")

	otelProfile := pprofile.NewProfile()
	otelProfile.SampleType().SetTypeStrindex(0)
	otelProfile.SetPeriod(100)
	ts := time.Date(2024, 6, 15, 12, 0, 0, 123456789, time.UTC)
	otelProfile.SetTime(pcommon.NewTimestampFromTime(ts))
	sampleWithLink := otelProfile.Samples().AppendEmpty()
	sampleWithLink.SetLinkIndex(1)
	otelProfile.Samples().AppendEmpty()

	pprofProfile := &profile.Profile{
		Sample: []*profile.Sample{
			{},
			{},
		},
	}

	AddProfilingPprofSampleLabels(pprofProfile, dict, scope, otelProfile)

	require.Len(t, pprofProfile.Sample, 2)
	assert.Equal(t, []string{"runtime/profiler"}, pprofProfile.Sample[0].Label[sourceEventNameLabel])
	assert.Equal(t, []int64{ts.UnixMilli()}, pprofProfile.Sample[0].NumLabel[sourceEventTimeLabel])
	assert.NotContains(t, pprofProfile.Sample[0].Label, sourceEventPeriodLabel)
	assert.Equal(t, []string{spanID.String()}, pprofProfile.Sample[0].Label[spanIDFieldKey])
	assert.Equal(t, []string{traceID.String()}, pprofProfile.Sample[0].Label[traceIDFieldKey])

	assert.Equal(t, []string{"runtime/profiler"}, pprofProfile.Sample[1].Label[sourceEventNameLabel])
	assert.Equal(t, []int64{ts.UnixMilli()}, pprofProfile.Sample[1].NumLabel[sourceEventTimeLabel])
	_, hasSpanID := pprofProfile.Sample[1].Label[spanIDFieldKey]
	assert.False(t, hasSpanID)
	_, hasTraceID := pprofProfile.Sample[1].Label[traceIDFieldKey]
	assert.False(t, hasTraceID)
}

func TestAddProfilingPprofSampleLabelsCPUPeriod(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dict := profiles.Dictionary()
	dict.StringTable().Append("samples")
	dict.StringTable().Append("count")
	dict.StringTable().Append("cpu")
	dict.StringTable().Append("nanoseconds")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("ebpf-profiler")
	otelProfile := pprofile.NewProfile()
	otelProfile.SampleType().SetTypeStrindex(0)
	otelProfile.SampleType().SetUnitStrindex(1)
	otelProfile.PeriodType().SetTypeStrindex(2)
	otelProfile.PeriodType().SetUnitStrindex(3)
	otelProfile.SetPeriod(50_000_000)
	otelProfile.Samples().AppendEmpty()

	pprofProfile := &profile.Profile{Sample: []*profile.Sample{{}}}
	AddProfilingPprofSampleLabels(pprofProfile, dict, scope, otelProfile)

	require.Equal(t, []string{"50"}, pprofProfile.Sample[0].Label[sourceEventPeriodLabel])
}

func TestAddProfilingPprofSampleLabelsCPUPeriodUnknownUnit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		periodUnit string
		unitIndex  int
	}{
		{name: "unknown unit", periodUnit: "count", unitIndex: 1},
		{name: "missing unit", unitIndex: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profiles := pprofile.NewProfiles()
			dict := profiles.Dictionary()
			dict.StringTable().Append("cpu")
			if tc.periodUnit != "" {
				dict.StringTable().Append(tc.periodUnit)
			}

			scope := pcommon.NewInstrumentationScope()
			otelProfile := pprofile.NewProfile()
			// The CPU sample type does not make an unknown or missing period unit convertible.
			otelProfile.SampleType().SetTypeStrindex(0)
			otelProfile.SampleType().SetUnitStrindex(0)
			otelProfile.PeriodType().SetTypeStrindex(0)
			otelProfile.PeriodType().SetUnitStrindex(int32(tc.unitIndex))
			otelProfile.SetPeriod(50_000_000)
			otelProfile.Samples().AppendEmpty()

			pprofProfile := &profile.Profile{Sample: []*profile.Sample{{}}}
			AddProfilingPprofSampleLabels(pprofProfile, dict, scope, otelProfile)

			assert.NotContains(t, pprofProfile.Sample[0].Label, sourceEventPeriodLabel)
		})
	}
}

func TestPeriodInMilliseconds(t *testing.T) {
	tests := []struct {
		name      string
		period    int64
		unit      string
		want      float64
		wantValid bool
	}{
		{name: "nanoseconds", period: 50_000_000, unit: "nanoseconds", want: 50, wantValid: true},
		{name: "microseconds", period: 1_500, unit: "us", want: 1.5, wantValid: true},
		{name: "milliseconds", period: 2, unit: "ms", want: 2, wantValid: true},
		{name: "seconds", period: 1, unit: "s", want: 1000, wantValid: true},
		{name: "unknown unit", period: 1, unit: "count", wantValid: false},
		{name: "non-positive", period: 0, unit: "ms", wantValid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := periodInMilliseconds(tt.period, tt.unit)
			assert.Equal(t, tt.wantValid, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
