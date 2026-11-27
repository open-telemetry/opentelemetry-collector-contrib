// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"

import (
	"slices"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func TestConvertPprofToProfilesPreservesInlineOrder(t *testing.T) {
	p := profileWithInlineFrames([]profile.Line{
		{Function: &profile.Function{ID: 1, Name: "leaf"}, Line: 900},
		{Function: &profile.Function{ID: 2, Name: "caller"}, Line: 100},
	})
	expectedLines := slices.Clone(p.Location[0].Line)
	require.NoError(t, p.CheckValid())

	converted, err := ConvertPprofToProfiles(p)
	require.NoError(t, err)
	requireInlineFrames(t, *converted, 0, expectedLines)
	require.Equal(t, expectedLines, p.Location[0].Line)
}

func TestConvertPprofToProfilesKeepsDistinctInlineChains(t *testing.T) {
	p := profileWithInlineFrames([]profile.Line{
		{Function: &profile.Function{ID: 1, Name: "leaf"}, Line: 900},
		{Function: &profile.Function{ID: 2, Name: "caller"}, Line: 100},
	})
	first := p.Location[0]
	second := &profile.Location{
		ID: 2, Mapping: first.Mapping, Address: first.Address,
		Line: []profile.Line{first.Line[1], first.Line[0]},
	}
	duplicate := &profile.Location{
		ID: 3, Mapping: first.Mapping, Address: first.Address,
		Line: slices.Clone(first.Line),
	}
	p.Location = append(p.Location, second, duplicate)
	p.Sample = append(p.Sample,
		&profile.Sample{Value: []int64{7}, Location: []*profile.Location{second}},
		&profile.Sample{Value: []int64{11}, Location: []*profile.Location{duplicate}},
	)
	expectedFirst, expectedSecond := slices.Clone(first.Line), slices.Clone(second.Line)
	require.NoError(t, p.CheckValid())

	converted, err := ConvertPprofToProfiles(p)
	require.NoError(t, err)
	samples := converted.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).Samples()
	require.NotEqual(t, samples.At(0).StackIndex(), samples.At(1).StackIndex())
	require.Equal(t, samples.At(0).StackIndex(), samples.At(2).StackIndex())
	requireInlineFrames(t, *converted, 0, expectedFirst)
	requireInlineFrames(t, *converted, 1, expectedSecond)
}

func profileWithInlineFrames(lines []profile.Line) *profile.Profile {
	mapping := &profile.Mapping{ID: 1, Start: 0x1000, Limit: 0x2000, File: "program"}
	location := &profile.Location{ID: 1, Mapping: mapping, Address: 0x1100, Line: lines}
	p := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		PeriodType: &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:     1,
		Mapping:    []*profile.Mapping{mapping},
		Location:   []*profile.Location{location},
		Sample:     []*profile.Sample{{Value: []int64{3}, Location: []*profile.Location{location}}},
	}
	for _, line := range lines {
		p.Function = append(p.Function, line.Function)
	}
	return p
}

func requireInlineFrames(t *testing.T, converted pprofile.Profiles, sampleIndex int, expected []profile.Line) {
	t.Helper()
	dictionary := converted.Dictionary()
	sample := converted.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).Samples().At(sampleIndex)
	stack := dictionary.StackTable().At(int(sample.StackIndex()))
	require.Equal(t, 1, stack.LocationIndices().Len())
	location := dictionary.LocationTable().At(int(stack.LocationIndices().At(0)))
	require.Equal(t, len(expected), location.Lines().Len())
	for i, expectedLine := range expected {
		line := location.Lines().At(i)
		function := dictionary.FunctionTable().At(int(line.FunctionIndex()))
		require.Equal(t, expectedLine.Function.Name, dictionary.StringTable().At(int(function.NameStrindex())))
		require.Equal(t, expectedLine.Line, line.Line())
		require.Equal(t, expectedLine.Column, line.Column())
	}
}
