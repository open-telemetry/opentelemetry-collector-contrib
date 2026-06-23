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
	assert.Equal(t, []string{"100"}, pprofProfile.Sample[0].Label[sourceEventPeriodLabel])
	assert.Equal(t, []string{spanID.String()}, pprofProfile.Sample[0].Label[spanIDFieldKey])
	assert.Equal(t, []string{traceID.String()}, pprofProfile.Sample[0].Label[traceIDFieldKey])

	assert.Equal(t, []string{"runtime/profiler"}, pprofProfile.Sample[1].Label[sourceEventNameLabel])
	assert.Equal(t, []int64{ts.UnixMilli()}, pprofProfile.Sample[1].NumLabel[sourceEventTimeLabel])
	_, hasSpanID := pprofProfile.Sample[1].Label[spanIDFieldKey]
	assert.False(t, hasSpanID)
	_, hasTraceID := pprofProfile.Sample[1].Label[traceIDFieldKey]
	assert.False(t, hasTraceID)
}
