// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// errInvalPprof is returned for invalid pprof data.
var errInvalPprof = errors.New("invalid pprof data")

const (
	// noAttrUnit is a helper to indicate that no
	// unit is associated to this Attribute.
	noAttrUnit = int32(-1)
)

// attr is a helper struct to build pprofile.ProfilesDictionary.attribute_table.
type attr struct {
	keyStrIdx  int32
	value      any
	unitStrIdx int32
}

// cache is a helper struct around pprofile.ProfilesDictionary.
type cache struct {
	stringTable        map[string]int32
	lastStringTableIdx int32

	attributeTable        map[attr]int32
	lastAttributeTableIdx int32
}

func convertPprofToPprofile(src *profile.Profile) (*pprofile.Profiles, error) {
	if err := src.CheckValid(); err != nil {
		return nil, fmt.Errorf("%w: %w", err, errInvalPprof)
	}
	dst := pprofile.NewProfiles()

	// mapping_table[0] must always be zero value (Mapping{}) and present.
	dst.Dictionary().MappingTable().AppendEmpty()
	lastMappingTableIdx := int32(0)

	// location_table[0] must always be zero value (Location{}) and present.
	dst.Dictionary().LocationTable().AppendEmpty()
	lastLocationTableIdx := int32(0)

	// function_table[0] must always be zero value (Function{}) and present.
	dst.Dictionary().FunctionTable().AppendEmpty()
	lastFunctionTableIdx := int32(0)

	// stack_table[0] must always be zero value (Stack{}) and present.
	dst.Dictionary().StackTable().AppendEmpty()
	lastStackTableIdx := int32(0)

	// Initialize remaining lookup tables of pprofile.ProfilesDictionary in initCache.
	transformCache := initCache()

	// Add envelope messages
	rp := dst.ResourceProfiles().AppendEmpty()
	rp.SetSchemaUrl(semconv.SchemaURL)

	sp := rp.ScopeProfiles().AppendEmpty()
	sp.SetSchemaUrl(semconv.SchemaURL)

	// Use a dedicated pprofile.Profile for each sample type.
	for stIdx, st := range src.SampleType {
		p := sp.Profiles().AppendEmpty()

		// pprof.Profile.sample_type
		p.SampleType().SetTypeStrindex(getIdxForString(transformCache, st.Type))
		p.SampleType().SetUnitStrindex(getIdxForString(transformCache, st.Unit))

		// pprof.Profile.sample
		for _, sample := range src.Sample {
			s := p.Sample().AppendEmpty()

			// pprof.Sample.location_id
			// TODO: deduplicate inserts to pprofile.ProfilesDictionary.
			var locTableIDs []int32
			for _, loc := range sample.Location {
				l := dst.Dictionary().LocationTable().AppendEmpty()
				lastLocationTableIdx++

				locTableIDs = append(locTableIDs, lastLocationTableIdx)

				// pprof.Location.id
				// TODO

				// pprof.Location.mapping_id
				if loc.Mapping != nil {
					// Insert the backing mapping information first.
					m := dst.Dictionary().MappingTable().AppendEmpty()
					lastMappingTableIdx++

					// pprof.Mapping.id
					// TODO

					// pprof.Mapping.memory_start
					m.SetMemoryStart(loc.Mapping.Start)

					// pprof.Mapping.memory_limit
					m.SetMemoryLimit(loc.Mapping.Limit)

					// pprof.Mapping.file_offset
					m.SetFileOffset(loc.Mapping.Offset)

					// pprof.Mapping.filename
					m.SetFilenameStrindex(getIdxForString(transformCache, loc.Mapping.File))

					// pprof.Mapping.build_id
					// Assume all build_ids are GNU build IDs
					buildIDIdx := getIdxForAttribute(transformCache,
						string(semconv.ProcessExecutableBuildIDGNUKey), loc.Mapping.BuildID)
					m.AttributeIndices().Append(buildIDIdx)

					// pprof.Mapping.has_*
					// The current Go release of SemConv does not yet include the changes from
					// https://github.com/open-telemetry/semantic-conventions/pull/2522
					// Therefore hardcode the values in the meantime.
					if loc.Mapping.HasFunctions {
						idx := getIdxForAttribute(transformCache, "pprof.mapping.has_functions", true)
						m.AttributeIndices().Append(idx)
					}

					if loc.Mapping.HasFilenames {
						idx := getIdxForAttribute(transformCache, "pprof.mapping.has_filenames", true)
						m.AttributeIndices().Append(idx)
					}

					if loc.Mapping.HasLineNumbers {
						idx := getIdxForAttribute(transformCache, "pprof.mapping.has_line_numbers", true)
						m.AttributeIndices().Append(idx)
					}

					if loc.Mapping.HasInlineFrames {
						idx := getIdxForAttribute(transformCache, "pprof.mapping.has_inline_frames", true)
						m.AttributeIndices().Append(idx)
					}
					l.SetMappingIndex(lastMappingTableIdx)
				}

				// pprof.Location.address
				l.SetAddress(loc.Address)

				// pprof.Location.line
				for _, line := range loc.Line {
					ln := l.Line().AppendEmpty()

					// pprof.Line.function_id
					if line.Function != nil {
						fn := dst.Dictionary().FunctionTable().AppendEmpty()
						lastFunctionTableIdx++

						// pprof.Function.id
						// TODO

						// pprof.Function.name
						fn.SetNameStrindex(getIdxForString(transformCache, line.Function.Name))

						// pprof.Function.system_name
						fn.SetSystemNameStrindex(getIdxForString(transformCache, line.Function.SystemName))

						// pprof.Function.filename
						fn.SetFilenameStrindex(getIdxForString(transformCache, line.Function.Filename))

						// pprof.Function.start_line
						fn.SetStartLine(line.Function.StartLine)

						ln.SetFunctionIndex(lastFunctionTableIdx)
					}

					// pprof.Line.line
					ln.SetLine(line.Line)

					// pprof.Line.column
					ln.SetColumn(line.Column)
				}

				// pprof.Location.is_folded
				if loc.IsFolded {
					idx := getIdxForAttribute(transformCache, "pprof.location.is_folded", true)
					l.AttributeIndices().Append(idx)
				}
			}

			st := dst.Dictionary().StackTable().AppendEmpty()
			lastStackTableIdx++
			st.LocationIndices().FromRaw(locTableIDs)

			// pprof.Sample.value
			if len(sample.Value) != len(src.SampleType) {
				return nil, fmt.Errorf("length of Sample.value (%d) is not equal to "+
					"length of Profile.sample_type (%d): %w",
					len(sample.Value), len(src.SampleType), errInvalPprof)
			}
			s.Values().Append(sample.Value[stIdx])

			// pprof.Sample.label - this field is split into string and numeric labels.
			for lk, lv := range sample.Label {
				if len(lv) != 1 {
					return nil, fmt.Errorf("invalid length of string label value %d: %w",
						len(lv), errInvalPprof)
				}
				var idx int32
				lu, exist := sample.NumUnit[lk]
				if !exist {
					idx = getIdxForAttribute(transformCache, lk, lv)
				} else {
					idx = getIdxForAttributeWithUnit(transformCache, lk, lu[0], lv)
				}
				s.AttributeIndices().Append(idx)
			}

			for lk, lv := range sample.NumLabel {
				if len(lv) != 1 {
					return nil, fmt.Errorf("invalid length of numeric label value %d: %w",
						len(lv), errInvalPprof)
				}
				var idx int32
				lu, exist := sample.NumUnit[lk]
				if !exist {
					idx = getIdxForAttribute(transformCache, lk, lv)
				} else {
					idx = getIdxForAttributeWithUnit(transformCache, lk, lu[0], lv)
				}
				s.AttributeIndices().Append(idx)
			}
		}

		// pprof.Profile.mapping
		// As OTel pprofile manages its own ProfilesDictionary.mapping_table, there
		// is no 1 to 1 mapping here.

		// pprof.Profile.location
		// As OTel pprofile manages its own ProfilesDictionary.location_table, there
		// is no 1 to 1 mapping here.

		// pprof.Profile.function
		// As OTel pprofile manages its own ProfilesDictionary.function_table, there
		// is no 1 to 1 mapping here.

		// pprof.Profile.string_table
		// As OTel pprofile manages its own ProfilesDictionary.string_table, there
		// is no 1 to 1 mapping here.

		// pprof.Profile.drop_frames
		dropFramesIdx := getIdxForAttribute(transformCache, "drop_frames", src.DropFrames)
		p.AttributeIndices().Append(dropFramesIdx)

		// pprof.Profile.keep_frames
		keepFramesIdx := getIdxForAttribute(transformCache, "keep_frames", src.KeepFrames)
		p.AttributeIndices().Append(keepFramesIdx)

		// pprof.Profile.time_nanos
		p.SetTime(pcommon.Timestamp(src.TimeNanos))

		// pprof.Profile.duration_nanos
		p.SetDuration(pcommon.Timestamp(src.DurationNanos))

		// pprof.Profile.period_type
		p.PeriodType().SetTypeStrindex(getIdxForString(transformCache, src.PeriodType.Type))
		p.PeriodType().SetUnitStrindex(getIdxForString(transformCache, src.PeriodType.Unit))

		// pprof.Profile.period
		p.SetPeriod(src.Period)

		// pprof.Profile.comment
		for _, c := range src.Comments {
			idx := getIdxForString(transformCache, c)
			p.CommentStrindices().Append(idx)
		}

		// pprof.Profile.default_sample_type
		// As OTel pprofile uses a single Sample Type, it is implicit its default type.

		// pprof.Profile.doc_url
		docURLIdx := getIdxForAttribute(transformCache, "doc_url", src.DocURL)
		p.AttributeIndices().Append(docURLIdx)
	}

	if err := dumpTransformCache(dst.Dictionary(), transformCache); err != nil {
		return nil, err
	}
	return &dst, nil
}

// getIdxForString returns the corresponding index for the string.
// If the string does not yet exist in the cache, it will be adedd.
func getIdxForString(c cache, s string) int32 {
	if idx, exists := c.stringTable[s]; exists {
		return idx
	}
	c.lastStringTableIdx++
	c.stringTable[s] = c.lastStringTableIdx
	return c.lastStringTableIdx
}

// getIdxForAttribute returns the corresponding index for the attribute.
// If the attribute does not yet exist in the cache, it will be added.
func getIdxForAttribute(c cache, key string, value any) int32 {
	return getIdxForAttributeWithUnit(c, key, "", value)
}

// getIdxForAttributeWithUnit returns the corresponding index for the attribute.
// If the attribute does not yet exist in the cache, it will be added.
func getIdxForAttributeWithUnit(c cache, key, unit string, value any) int32 {
	keyStrIdx := getIdxForString(c, key)

	unitStrIdx := noAttrUnit
	if unit != "" {
		unitStrIdx = getIdxForString(c, unit)
	}

	for attr, idx := range c.attributeTable {
		if attr.keyStrIdx != keyStrIdx {
			continue
		}
		if attr.unitStrIdx != unitStrIdx {
			continue
		}
		if !isEqualValue(attr.value, value) {
			continue
		}
		return idx
	}

	c.lastAttributeTableIdx++
	c.attributeTable[attr{
		keyStrIdx:  keyStrIdx,
		value:      value,
		unitStrIdx: unitStrIdx,
	}] = c.lastAttributeTableIdx
	return c.lastAttributeTableIdx
}

func isEqualValue(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// initCache returns a supporting elements to construct pprofile.ProfilesDictionary.
func initCache() cache {
	c := cache{
		stringTable:    make(map[string]int32),
		attributeTable: make(map[attr]int32),
	}

	// string_table[0] must always be "" and present.
	c.lastStringTableIdx = 0
	c.stringTable[""] = c.lastStringTableIdx

	// attribute_table[0] must always be zero value (KeyValueAndUnit{}) and present.
	c.lastAttributeTableIdx = 0
	c.attributeTable[attr{}] = c.lastAttributeTableIdx

	return c
}

// dumpTransformCache fills pprofile.ProfilesDictionary with the content of
// the supporting cache.
func dumpTransformCache(dic pprofile.ProfilesDictionary, c cache) error {
	for i := 0; i < len(c.stringTable); i++ {
		dic.StringTable().Append("")
	}
	for s, id := range c.stringTable {
		dic.StringTable().SetAt(int(id), s)
	}

	for i := 0; i < len(c.attributeTable); i++ {
		dic.AttributeTable().AppendEmpty()
	}
	for a, id := range c.attributeTable {
		dic.AttributeTable().At(int(id)).SetKeyStrindex(a.keyStrIdx)
		if err := dic.AttributeTable().At(int(id)).Value().FromRaw(a.value); err != nil {
			return err
		}
		if a.unitStrIdx != noAttrUnit {
			dic.AttributeTable().At(int(id)).SetUnitStrindex(a.unitStrIdx)
		}
	}
	return nil
}
