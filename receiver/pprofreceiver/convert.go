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
	// noAttrUnit is an internal helper to indicate that no
	// unit is associated to this Attribute.
	noAttrUnit = int32(-1)
)

// attr is a helper struct to build pprofile.ProfilesDictionary.attribute_table.
type attr struct {
	keyStrIdx  int32
	value      any
	unitStrIdx int32
}

// fn is a helper struct to build pprofile.ProfilesDictionary.function_table.
type fn struct {
	name       string
	systemName string
	fileName   string
	startLine  int64
}

// lookupTables is a helper struct around pprofile.ProfilesDictionary.
type lookupTables struct {
	stringTable        map[string]int32
	lastStringTableIdx int32

	attributeTable        map[attr]int32
	lastAttributeTableIdx int32

	functionTable        map[fn]int32
	lastFunctionTableIdx int32
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

	// stack_table[0] must always be zero value (Stack{}) and present.
	dst.Dictionary().StackTable().AppendEmpty()
	lastStackTableIdx := int32(0)

	// Initialize remaining lookup tables of pprofile.ProfilesDictionary in initLookupTables.
	lts := initLookupTables()

	// Add envelope messages
	rp := dst.ResourceProfiles().AppendEmpty()
	rp.SetSchemaUrl(semconv.SchemaURL)

	sp := rp.ScopeProfiles().AppendEmpty()
	sp.SetSchemaUrl(semconv.SchemaURL)

	// Use a dedicated pprofile.Profile for each sample type.
	for stIdx, st := range src.SampleType {
		p := sp.Profiles().AppendEmpty()

		// pprof.Profile.sample_type
		p.SampleType().SetTypeStrindex(lts.getIdxForString(st.Type))
		p.SampleType().SetUnitStrindex(lts.getIdxForString(st.Unit))

		// pprof.Profile.sample
		for _, sample := range src.Sample {
			s := p.Sample().AppendEmpty()

			// pprof.Sample.location_id
			var locTableIDs []int32
			for _, loc := range sample.Location {
				l := dst.Dictionary().LocationTable().AppendEmpty()
				lastLocationTableIdx++

				locTableIDs = append(locTableIDs, lastLocationTableIdx)

				// pprof.Location.mapping_id
				if loc.Mapping != nil {
					// Insert the backing mapping information first.
					m := dst.Dictionary().MappingTable().AppendEmpty()
					lastMappingTableIdx++

					// pprof.Mapping.memory_start
					m.SetMemoryStart(loc.Mapping.Start)

					// pprof.Mapping.memory_limit
					m.SetMemoryLimit(loc.Mapping.Limit)

					// pprof.Mapping.file_offset
					m.SetFileOffset(loc.Mapping.Offset)

					// pprof.Mapping.filename
					m.SetFilenameStrindex(lts.getIdxForString(loc.Mapping.File))

					// pprof.Mapping.build_id
					// Assume all build_ids are GNU build IDs
					buildIDIdx := lts.getIdxForAttribute(
						string(semconv.ProcessExecutableBuildIDGNUKey), loc.Mapping.BuildID)
					m.AttributeIndices().Append(buildIDIdx)

					// pprof.Mapping.has_*
					// The current Go release of SemConv does not yet include the changes from
					// https://github.com/open-telemetry/semantic-conventions/pull/2522
					// Therefore hardcode the values in the meantime.
					if loc.Mapping.HasFunctions {
						idx := lts.getIdxForAttribute("pprof.mapping.has_functions", true)
						m.AttributeIndices().Append(idx)
					}

					if loc.Mapping.HasFilenames {
						idx := lts.getIdxForAttribute("pprof.mapping.has_filenames", true)
						m.AttributeIndices().Append(idx)
					}

					if loc.Mapping.HasLineNumbers {
						idx := lts.getIdxForAttribute("pprof.mapping.has_line_numbers", true)
						m.AttributeIndices().Append(idx)
					}

					if loc.Mapping.HasInlineFrames {
						idx := lts.getIdxForAttribute("pprof.mapping.has_inline_frames", true)
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
						ln.SetFunctionIndex(getIdxForFunction(lts,
							line.Function.Name,
							line.Function.SystemName,
							line.Function.Filename,
							line.Function.StartLine))
					}

					// pprof.Line.line
					ln.SetLine(line.Line)

					// pprof.Line.column
					ln.SetColumn(line.Column)
				}

				// pprof.Location.is_folded
				if loc.IsFolded {
					idx := lts.getIdxForAttribute("pprof.location.is_folded", true)
					l.AttributeIndices().Append(idx)
				}
			}

			st := dst.Dictionary().StackTable().AppendEmpty()
			lastStackTableIdx++
			st.LocationIndices().FromRaw(locTableIDs)

			// pprof.Sample.value
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
					idx = lts.getIdxForAttribute(lk, lv)
				} else {
					idx = lts.getIdxForAttributeWithUnit(lk, lu[0], lv)
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
					idx = lts.getIdxForAttribute(lk, lv)
				} else {
					idx = lts.getIdxForAttributeWithUnit(lk, lu[0], lv)
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
		dropFramesIdx := lts.getIdxForAttribute("drop_frames", src.DropFrames)
		p.AttributeIndices().Append(dropFramesIdx)

		// pprof.Profile.keep_frames
		keepFramesIdx := lts.getIdxForAttribute("keep_frames", src.KeepFrames)
		p.AttributeIndices().Append(keepFramesIdx)

		// pprof.Profile.time_nanos
		p.SetTime(pcommon.Timestamp(src.TimeNanos))

		// pprof.Profile.duration_nanos
		p.SetDuration(pcommon.Timestamp(src.DurationNanos))

		// pprof.Profile.period_type
		p.PeriodType().SetTypeStrindex(lts.getIdxForString(src.PeriodType.Type))
		p.PeriodType().SetUnitStrindex(lts.getIdxForString(src.PeriodType.Unit))

		// pprof.Profile.period
		p.SetPeriod(src.Period)

		// pprof.Profile.comment
		for _, c := range src.Comments {
			idx := lts.getIdxForString(c)
			p.CommentStrindices().Append(idx)
		}

		// pprof.Profile.default_sample_type
		// As OTel pprofile uses a single Sample Type, it is implicit its default type.

		// pprof.Profile.doc_url
		docURLIdx := lts.getIdxForAttribute("doc_url", src.DocURL)
		p.AttributeIndices().Append(docURLIdx)
	}

	if err := lts.dumpLookupTables(dst.Dictionary()); err != nil {
		return nil, err
	}
	return &dst, nil
}

// getIdxForFunction returns the corresponding index for the function.
// If the function does not yet exist in the cache, it will be added.
func getIdxForFunction(lts lookupTables, name, systemName, fileName string, startLine int64) int32 {
	key := fn{
		name:       name,
		systemName: systemName,
		fileName:   fileName,
		startLine:  startLine,
	}
	if idx, exists := lts.functionTable[key]; exists {
		return idx
	}
	lts.lastFunctionTableIdx++
	lts.functionTable[key] = lts.lastFunctionTableIdx
	return lts.lastFunctionTableIdx
}

// getIdxForString returns the corresponding index for the string.
// If the string does not yet exist in the cache, it will be added.
func (lts *lookupTables) getIdxForString(s string) int32 {
	if idx, exists := lts.stringTable[s]; exists {
		return idx
	}
	lts.lastStringTableIdx++
	lts.stringTable[s] = lts.lastStringTableIdx
	return lts.lastStringTableIdx
}

// getIdxForAttribute returns the corresponding index for the attribute.
// If the attribute does not yet exist in the cache, it will be added.
func (lts *lookupTables) getIdxForAttribute(key string, value any) int32 {
	return lts.getIdxForAttributeWithUnit(key, "", value)
}

// getIdxForAttributeWithUnit returns the corresponding index for the attribute.
// If the attribute does not yet exist in the cache, it will be added.
func (lts *lookupTables) getIdxForAttributeWithUnit(key, unit string, value any) int32 {
	keyStrIdx := lts.getIdxForString(key)

	unitStrIdx := noAttrUnit
	if unit != "" {
		unitStrIdx = lts.getIdxForString(unit)
	}

	for attr, idx := range lts.attributeTable {
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

	lts.lastAttributeTableIdx++
	lts.attributeTable[attr{
		keyStrIdx:  keyStrIdx,
		value:      value,
		unitStrIdx: unitStrIdx,
	}] = lts.lastAttributeTableIdx
	return lts.lastAttributeTableIdx
}

func isEqualValue(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// initLookupTables returns a supporting elements to construct pprofile.ProfilesDictionary.
func initLookupTables() lookupTables {
	lts := lookupTables{
		stringTable:    make(map[string]int32),
		attributeTable: make(map[attr]int32),
		functionTable:  make(map[fn]int32),
	}

	// string_table[0] must always be "" and present.
	lts.lastStringTableIdx = 0
	lts.stringTable[""] = lts.lastStringTableIdx

	// attribute_table[0] must always be zero value (KeyValueAndUnit{}) and present.
	lts.lastAttributeTableIdx = 0
	lts.attributeTable[attr{}] = lts.lastAttributeTableIdx

	// function_table[0] must always be zero value (Function{}) and present.
	lts.lastFunctionTableIdx = 0
	lts.functionTable[fn{}] = lts.lastFunctionTableIdx
	return lts
}

// dumpLookupTables fills pprofile.ProfilesDictionary with the content of
// the supporting lookup tables.
func (lts *lookupTables) dumpLookupTables(dic pprofile.ProfilesDictionary) error {
	for i := 0; i < len(lts.functionTable); i++ {
		dic.FunctionTable().AppendEmpty()
	}
	for fn, id := range lts.functionTable {
		dic.FunctionTable().At(int(id)).SetNameStrindex(lts.getIdxForString(fn.name))
		dic.FunctionTable().At(int(id)).SetSystemNameStrindex(lts.getIdxForString(fn.systemName))
		dic.FunctionTable().At(int(id)).SetFilenameStrindex(lts.getIdxForString(fn.fileName))
		dic.FunctionTable().At(int(id)).SetStartLine(fn.startLine)
	}

	for i := 0; i < len(lts.stringTable); i++ {
		dic.StringTable().Append("")
	}
	for s, id := range lts.stringTable {
		dic.StringTable().SetAt(int(id), s)
	}

	for i := 0; i < len(lts.attributeTable); i++ {
		dic.AttributeTable().AppendEmpty()
	}
	for a, id := range lts.attributeTable {
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
