// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"

import (
	"crypto/md5" //nolint:gosec
	"time"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func pprofToPprofile(parsed *profile.Profile, originalPayload []byte) pprofile.Profiles {
	result := pprofile.NewProfiles()
	// create a new profile record
	resultProfile := result.ResourceProfiles().AppendEmpty().ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()

	var hash [16]byte
	// set the original payload on the record
	// per https://github.com/open-telemetry/semantic-conventions/blob/main/model/profile/registry.yaml
	resultProfile.SetOriginalPayloadFormat("go")
	resultProfile.OriginalPayload().FromRaw(originalPayload)
	hash = md5.Sum(originalPayload) //nolint:gosec

	// set period
	resultProfile.SetPeriod(parsed.Period)
	// TODO set period type?
	// TODO set doc url?

	// set timestamps - both start and duration
	resultProfile.SetStartTime(pcommon.NewTimestampFromTime(time.Unix(0, parsed.TimeNanos)))
	resultProfile.SetDuration(pcommon.NewTimestampFromTime(time.Unix(0, parsed.DurationNanos)))

	// set the default simple type. Given this is a string, it gets cached in the string table and referred to via its index.
	result.ProfilesDictionary().StringTable().Append(parsed.DefaultSampleType)
	resultProfile.SetDefaultSampleTypeIndex(int32(result.ProfilesDictionary().StringTable().Len() - 1))

	// set the profile ID.
	resultProfile.SetProfileID(hash)

	// add all mappings to the global dictionary
	for _, m := range parsed.Mapping {
		// create a mapping in the mapping table
		resultM := result.ProfilesDictionary().MappingTable().AppendEmpty()
		resultM.SetFileOffset(m.Offset)
		resultM.SetMemoryStart(m.Start)
		resultM.SetMemoryLimit(m.Limit)
		resultM.SetHasFilenames(m.HasFilenames)
		resultM.SetHasFunctions(m.HasFunctions)
		resultM.SetHasInlineFrames(m.HasInlineFrames)
		resultM.SetHasLineNumbers(m.HasLineNumbers)
		result.ProfilesDictionary().StringTable().Append(m.File)
		resultM.SetFilenameStrindex(int32(result.ProfilesDictionary().StringTable().Len() - 1))
	}

	// now iterate over each sample of the pprof data
	for _, s := range parsed.Sample {
		// for each sample create a corresponding sample record in our pprofile record
		resultS := resultProfile.Sample().AppendEmpty()
		// set the value of the sample.
		resultS.Value().FromRaw(s.Value)
		// now iterate over each location.

		// set the start index at which we will start to read locations.
		resultS.SetLocationsStartIndex(int32(result.ProfilesDictionary().LocationTable().Len()))

		// read each location and map it over to pprofile
		for _, l := range parsed.Location {
			// add to the location table
			resultL := result.ProfilesDictionary().LocationTable().AppendEmpty()
			resultL.SetAddress(l.Address)
			resultL.SetIsFolded(l.IsFolded)
			for i, m := range result.ProfilesDictionary().MappingTable().All() {
				if m.FileOffset() == l.Mapping.Offset && result.ProfilesDictionary().StringTable().At(int(m.FilenameStrindex())) == l.Mapping.File {
					resultL.SetMappingIndex(int32(i))
					break
				}
			}
		}
		resultS.SetLocationsLength(int32(len(parsed.Location)))
	}

	for _, f := range parsed.Function {
		resultF := result.ProfilesDictionary().FunctionTable().AppendEmpty()
		result.ProfilesDictionary().StringTable().Append(f.Filename)
		resultF.SetFilenameStrindex(int32(result.ProfilesDictionary().StringTable().Len() - 1))
		result.ProfilesDictionary().StringTable().Append(f.Name)
		resultF.SetNameStrindex(int32(result.ProfilesDictionary().StringTable().Len() - 1))
		resultF.SetStartLine(f.StartLine)
		result.ProfilesDictionary().StringTable().Append(f.SystemName)
		resultF.SetSystemNameStrindex(int32(result.ProfilesDictionary().StringTable().Len() - 1))
	}

	for _, s := range parsed.SampleType {
		resultSampleType := resultProfile.SampleType().AppendEmpty()
		result.ProfilesDictionary().StringTable().Append(s.Unit)
		resultSampleType.SetUnitStrindex(int32(result.ProfilesDictionary().StringTable().Len() - 1))
		result.ProfilesDictionary().StringTable().Append(s.Type)
		resultSampleType.SetTypeStrindex(int32(result.ProfilesDictionary().StringTable().Len() - 1))
		// TODO set aggregation temporality
	}

	commentsIndices := make([]int32, len(parsed.Comments))
	start := result.ProfilesDictionary().StringTable().Len() - 1
	result.ProfilesDictionary().StringTable().Append(parsed.Comments...)
	for i := range parsed.Comments {
		commentsIndices[i] = int32(start + i)
	}
	resultProfile.CommentStrindices().FromRaw(commentsIndices)

	return result
}
