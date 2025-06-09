// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"runtime/pprof"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func TestMapPprofToPdata(t *testing.T) {
	p := pprof.Lookup("goroutine")
	pprofData := new(bytes.Buffer)
	err := p.WriteTo(pprofData, 0)
	require.NoError(t, err)
	parsed, err := profile.Parse(pprofData)
	require.NoError(t, err)
	converted := pprofToPprofile(parsed, pprofData.Bytes())
	require.NotNil(t, converted)
	pprofBack := pprofileToPprof(converted)
	require.Equal(t, parsed, pprofBack[0])
}

func pprofileToPprof(pdataProfiles pprofile.Profiles) []*profile.Profile {
	result := []*profile.Profile{}
	functions := make([]*profile.Function, pdataProfiles.ProfilesDictionary().FunctionTable().Len())
	for i, f := range pdataProfiles.ProfilesDictionary().FunctionTable().All() {
		functions[i] = &profile.Function{
			ID:         uint64(i + 1), // this is not preserved, so we just make it up.
			Name:       pdataProfiles.ProfilesDictionary().StringTable().At(int(f.NameStrindex())),
			SystemName: pdataProfiles.ProfilesDictionary().StringTable().At(int(f.SystemNameStrindex())),
			Filename:   pdataProfiles.ProfilesDictionary().StringTable().At(int(f.FilenameStrindex())),
			StartLine:  f.StartLine(),
		}
	}

	mappings := make([]*profile.Mapping, pdataProfiles.ProfilesDictionary().MappingTable().Len())
	for i, m := range pdataProfiles.ProfilesDictionary().MappingTable().All() {
		mappings[i] = &profile.Mapping{
			ID:                     uint64(i + 1), // this is not preserved, so we just make it up.
			Start:                  m.MemoryStart(),
			Limit:                  m.MemoryLimit(),
			Offset:                 m.FileOffset(),
			File:                   pdataProfiles.ProfilesDictionary().StringTable().At(int(m.FilenameStrindex())),
			BuildID:                "", // TODO no mapping
			HasFunctions:           m.HasFunctions(),
			HasFilenames:           m.HasFilenames(),
			HasLineNumbers:         m.HasLineNumbers(),
			HasInlineFrames:        m.HasInlineFrames(),
			KernelRelocationSymbol: "", // TODO no mapping
		}
	}
	for _, rp := range pdataProfiles.ResourceProfiles().All() {
		for _, sp := range rp.ScopeProfiles().All() {
			for _, p := range sp.Profiles().All() {
				var comments []string
				if p.CommentStrindices().Len() > 0 {
					comments = make([]string, p.CommentStrindices().Len())
					for i, index := range p.CommentStrindices().All() {
						comments[i] = pdataProfiles.ProfilesDictionary().StringTable().At(int(index))
					}
				}
				sampleTypes := make([]*profile.ValueType, p.SampleType().Len())
				for i, st := range p.SampleType().All() {
					sampleTypes[i] = &profile.ValueType{
						Type: pdataProfiles.ProfilesDictionary().StringTable().At(int(st.TypeStrindex())),
						Unit: pdataProfiles.ProfilesDictionary().StringTable().At(int(st.UnitStrindex())),
					}
				}

				locations := make([]*profile.Location, p.LocationIndices().Len())
				for i, l := range p.LocationIndices().All() {
					pdataLocation := pdataProfiles.ProfilesDictionary().LocationTable().At(int(l))
					lines := make([]profile.Line, pdataLocation.Line().Len())
					for j, k := range pdataLocation.Line().All() {
						lines[j] = profile.Line{
							Function: functions[k.FunctionIndex()],
							Line:     k.Line(),
							Column:   k.Column(),
						}
					}
					locations[i] = &profile.Location{
						ID:       uint64(l + 1), // TODO we don't map this over.
						Mapping:  mappings[pdataLocation.MappingIndex()],
						Address:  pdataLocation.Address(),
						Line:     lines,
						IsFolded: pdataLocation.IsFolded(),
					}
				}

				samples := make([]*profile.Sample, p.Sample().Len())
				for i, s := range p.Sample().All() {
					sampleLocations := make([]*profile.Location, s.LocationsLength())
					for li := 0; li < int(s.LocationsLength()); li++ {
						pdataLocation := pdataProfiles.ProfilesDictionary().LocationTable().At(i + int(s.LocationsStartIndex()))
						lines := make([]profile.Line, pdataLocation.Line().Len())
						for j, k := range pdataLocation.Line().All() {
							lines[j] = profile.Line{
								Function: functions[k.FunctionIndex()],
								Line:     k.Line(),
								Column:   k.Column(),
							}
						}
						sampleLocations[li] = &profile.Location{
							ID:       uint64(li + 1), // TODO we don't map this over.
							Mapping:  mappings[pdataLocation.MappingIndex()],
							Address:  pdataLocation.Address(),
							Line:     lines,
							IsFolded: pdataLocation.IsFolded(),
						}
					}
					samples[i] = &profile.Sample{
						Location: sampleLocations,
						Value:    s.Value().AsRaw(),
						Label:    nil, // TODO no mapping
						NumLabel: nil, // TODO no mapping
						NumUnit:  nil, // TODO no mapping
					}
				}

				result = append(result, &profile.Profile{
					SampleType:        sampleTypes,
					DefaultSampleType: pdataProfiles.ProfilesDictionary().StringTable().At(int(p.DefaultSampleTypeIndex())),
					Sample:            samples,
					Mapping:           mappings,
					Location:          locations,
					Function:          functions,
					Comments:          comments,
					DocURL:            "", // TODO no mapping
					DropFrames:        "", // TODO no mapping
					KeepFrames:        "", // TODO no mapping
					TimeNanos:         p.Time().AsTime().UnixNano(),
					DurationNanos:     p.Duration().AsTime().UnixNano(),
					PeriodType:        nil,
					Period:            p.Period(),
				})
			}
		}
	}
	return result
}
