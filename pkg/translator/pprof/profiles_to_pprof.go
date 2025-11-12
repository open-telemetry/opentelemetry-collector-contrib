// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

var (
	// errInvalPprof is returned for invalid pprof data.
	errInvalPprof = errors.New("invalid pprof data")
	// errInalIdxFomrat is returned on invalid string formats for indices.
	errInalIdxFomrat = errors.New("invalid format of attribute indices")
)

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

// mm is a helper struct to build pprofile.ProfilesDictionary.mapping_table.
type mm struct {
	memoryStart    uint64
	memoryLimit    uint64
	fileOffset     uint64
	filenameStrIdx int32
	attrIdxs       string // List of consecutive increasing indices separated by a semicolon.
}

// loc is a helper struct to build pprofile.ProfilesDictionary.location_table.
type loc struct {
	mappingIdx int32
	address    uint64
	lines      string // String representation of a sorted list of lines.
	attrIdxs   string // List of consecutive increasing indices separated by a semicolon.
}

// lookupTables is a helper struct around pprofile.ProfilesDictionary.
type lookupTables struct {
	mappingTable        map[mm]int32
	lastMappingTableIdx int32

	locationTable        map[loc]int32
	lastLocationTableIdx int32

	functionTable        map[fn]int32
	lastFunctionTableIdx int32

	// The concept of profiles.Link does not exit in pprof.
	// Therefore this helper table is skipped.

	stringTable        map[string]int32
	lastStringTableIdx int32

	attributeTable        map[attr]int32
	lastAttributeTableIdx int32

	stackTable        map[string]int32
	lastStackTableIdx int32
}

func convertPprofToPprofile(src *profile.Profile) (*pprofile.Profiles, error) {
	if err := src.CheckValid(); err != nil {
		return nil, fmt.Errorf("%w: %w", err, errInvalPprof)
	}
	dst := pprofile.NewProfiles()

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
			s := p.Samples().AppendEmpty()

			// pprof.Sample.location_id
			stackIdx := lts.getIdxForStack(sample.Location)
			s.SetStackIndex(stackIdx)

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
		// TODO: refactor lookupTables to allow string[] to be held
		p.AttributeIndices().Append(lts.getIdxForAttributeWithUnit("pprof.profile.comment", "", strings.Join(src.Comments, ",")))

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
func (lts *lookupTables) getIdxForFunction(name, systemName, fileName string, startLine int64) int32 {
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

// getIdxForMapping returns the correspoinding index for the mapping.
// If the mapping does not yet exist in the cache, it will be added.
func (lts *lookupTables) getIdxForMapping(start, limit, offset uint64, fnStrIdx int32, attrIdx []int32) int32 {
	attrIdxs := attrIdxToString(attrIdx)
	key := mm{
		memoryStart:    start,
		memoryLimit:    limit,
		fileOffset:     offset,
		filenameStrIdx: fnStrIdx,
		attrIdxs:       attrIdxs,
	}
	if idx, exists := lts.mappingTable[key]; exists {
		return idx
	}
	lts.lastMappingTableIdx++
	lts.mappingTable[key] = lts.lastMappingTableIdx
	return lts.lastMappingTableIdx
}

// getIdxForMMAttributes returns a list of indices to attributes related
// to the mapping.
func (lts *lookupTables) getIdxForMMAttributes(m *profile.Mapping) []int32 {
	ids := []int32{}

	// pprof.Mapping.build_id
	// Assume all build_ids are GNU build IDs
	buildIDIdx := lts.getIdxForAttribute(
		string(semconv.ProcessExecutableBuildIDGNUKey), m.BuildID)
	ids = append(ids, buildIDIdx)

	// pprof.Mapping.has_*
	// The current Go release of SemConv does not yet include the changes from
	// https://github.com/open-telemetry/semantic-conventions/pull/2522
	// Therefore hardcode the values in the meantime.
	if m.HasFunctions {
		idx := lts.getIdxForAttribute("pprof.mapping.has_functions", true)
		ids = append(ids, idx)
	}

	if m.HasFilenames {
		idx := lts.getIdxForAttribute("pprof.mapping.has_filenames", true)
		ids = append(ids, idx)
	}

	if m.HasLineNumbers {
		idx := lts.getIdxForAttribute("pprof.mapping.has_line_numbers", true)
		ids = append(ids, idx)
	}

	if m.HasInlineFrames {
		idx := lts.getIdxForAttribute("pprof.mapping.has_inline_frames", true)
		ids = append(ids, idx)
	}

	return ids
}

// getIdxForStack returns the corresponding index for a stack and its location
// indices. If the mapping does not yet exist in the cache, it will be added.
func (lts *lookupTables) getIdxForStack(locs []*profile.Location) int32 {
	var locTableIDs []int32
	for _, loc := range locs {
		idx := lts.getIdxForLocation(loc)

		locTableIDs = append(locTableIDs, idx)
	}

	key := attrIdxToString(locTableIDs)
	if idx, exists := lts.stackTable[key]; exists {
		return idx
	}
	lts.lastStackTableIdx++
	lts.stackTable[key] = lts.lastStackTableIdx
	return lts.lastStackTableIdx
}

// getIdxForLocation returns the corresponding index for a location.
// If the location does not yet exist in the cache, it will be added.
func (lts *lookupTables) getIdxForLocation(l *profile.Location) int32 {
	var attrIdxs []int32
	// pprof.Location.is_folded
	if l.IsFolded {
		idx := lts.getIdxForAttribute("pprof.location.is_folded", true)
		attrIdxs = append(attrIdxs, idx)
	}

	key := loc{
		// pprof.Location.address
		address:  l.Address,
		attrIdxs: attrIdxToString(attrIdxs),
		// pprof.Location.line
		lines: lts.linesToString(l.Line),
	}

	// pprof.Location.mapping_id
	if l.Mapping != nil {
		mmIdx := lts.getIdxForMapping(
			l.Mapping.Start,
			l.Mapping.Limit,
			l.Mapping.Offset,
			lts.getIdxForString(l.Mapping.File),
			lts.getIdxForMMAttributes(l.Mapping),
		)
		key.mappingIdx = mmIdx
	}

	if idx, exists := lts.locationTable[key]; exists {
		return idx
	}
	lts.lastLocationTableIdx++
	lts.locationTable[key] = lts.lastLocationTableIdx
	return lts.lastLocationTableIdx
}

func isEqualValue(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// initLookupTables returns a supporting elements to construct pprofile.ProfilesDictionary.
func initLookupTables() lookupTables {
	lts := lookupTables{
		mappingTable:   make(map[mm]int32),
		locationTable:  make(map[loc]int32),
		functionTable:  make(map[fn]int32),
		stringTable:    make(map[string]int32),
		attributeTable: make(map[attr]int32),
		stackTable:     make(map[string]int32),
	}

	// mapping_table[0] must always be zero value (Mapping{}) and present.
	lts.lastMappingTableIdx = 0
	lts.mappingTable[mm{}] = lts.lastMappingTableIdx

	// location_table[0] must always be zero value (Location{}) and present.
	lts.lastLocationTableIdx = 0
	lts.locationTable[loc{}] = lts.lastLocationTableIdx

	// function_table[0] must always be zero value (Function{}) and present.
	lts.lastFunctionTableIdx = 0
	lts.functionTable[fn{}] = lts.lastFunctionTableIdx

	// string_table[0] must always be "" and present.
	lts.lastStringTableIdx = 0
	lts.stringTable[""] = lts.lastStringTableIdx

	// attribute_table[0] must always be zero value (KeyValueAndUnit{}) and present.
	lts.lastAttributeTableIdx = 0
	lts.attributeTable[attr{}] = lts.lastAttributeTableIdx

	// stack_table[0] must always be zero value (Stack{}) and present.
	lts.lastStackTableIdx = 0
	lts.stackTable[""] = lts.lastStackTableIdx
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

	for i := 0; i < len(lts.mappingTable); i++ {
		dic.MappingTable().AppendEmpty()
	}
	for m, id := range lts.mappingTable {
		dic.MappingTable().At(int(id)).SetMemoryStart(m.memoryStart)
		dic.MappingTable().At(int(id)).SetMemoryLimit(m.memoryLimit)
		dic.MappingTable().At(int(id)).SetFileOffset(m.fileOffset)
		dic.MappingTable().At(int(id)).SetFilenameStrindex(m.filenameStrIdx)
		attrIndices, err := stringToAttrIdx(m.attrIdxs)
		if err != nil {
			return err
		}
		dic.MappingTable().At(int(id)).AttributeIndices().Append(attrIndices...)
	}

	for i := 0; i < len(lts.locationTable); i++ {
		dic.LocationTable().AppendEmpty()
	}
	for l, id := range lts.locationTable {
		dic.LocationTable().At(int(id)).SetAddress(l.address)
		dic.LocationTable().At(int(id)).SetMappingIndex(l.mappingIdx)
		lines, err := stringToLine(l.lines)
		if err != nil {
			return err
		}
		for _, ln := range lines {
			newLine := dic.LocationTable().At(int(id)).Lines().AppendEmpty()
			newLine.SetLine(ln.Line())
			newLine.SetColumn(ln.Column())
			newLine.SetFunctionIndex(ln.FunctionIndex())
		}
		attrIdxs, err := stringToAttrIdx(l.attrIdxs)
		if err != nil {
			return err
		}
		dic.LocationTable().At(int(id)).AttributeIndices().Append(attrIdxs...)
	}

	for i := 0; i < len(lts.stackTable); i++ {
		dic.StackTable().AppendEmpty()
	}
	for s, id := range lts.stackTable {
		locIndices, err := stringToAttrIdx(s)
		if err != nil {
			return err
		}
		dic.StackTable().At(int(id)).LocationIndices().Append(locIndices...)
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

	// The concept of profiles.Link does not exist in pprof.
	// Therefore LinkTable only holds an empty value to be compliant.
	dic.LinkTable().AppendEmpty()

	return nil
}

// attrIdxToString is a helper function to convert a list of indices
// into a string.
func attrIdxToString(indices []int32) string {
	if len(indices) == 0 {
		return ""
	}
	slices.Sort(indices)

	stringNumbers := make([]string, len(indices))

	for i, n := range indices {
		stringNumbers[i] = strconv.FormatInt(int64(n), 10)
	}

	return strings.Join(stringNumbers, ";")
}

// stringToAttrIdx is a helper function to convert a string into
// a list of indices.
func stringToAttrIdx(indices string) ([]int32, error) {
	if indices == "" {
		return []int32{}, nil
	}
	parts := strings.Split(indices, ";")

	result := make([]int32, 0, len(parts))
	for _, s := range parts {
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse '%s' as int32. %w", s, err)
		}
		result = append(result, int32(n))
	}
	if !slices.IsSorted(result) {
		return nil, fmt.Errorf("invalid order of indices '%s': %w", indices, errInalIdxFomrat)
	}

	return result, nil
}

// linesToString is a helper function to convert a list of lines into a string.
func (lts *lookupTables) linesToString(lines []profile.Line) string {
	if len(lines) == 0 {
		return ""
	}

	slices.SortFunc(lines, func(a, b profile.Line) int {
		if a.Line != b.Line {
			return int(a.Line - b.Line)
		}
		if a.Column != b.Column {
			return int(a.Column - b.Column)
		}
		if a.Function == nil && b.Function == nil {
			return 0
		}
		if a.Function == nil {
			return -1
		}
		if b.Function == nil {
			return 1
		}
		return int(a.Function.ID - b.Function.ID)
	})

	var parts []string
	for _, line := range lines {
		funcID := int32(-1)
		if line.Function != nil {
			funcID = lts.getIdxForFunction(
				line.Function.Name,
				line.Function.SystemName,
				line.Function.Filename,
				line.Function.StartLine)
		}
		parts = append(parts, fmt.Sprintf("%d:%d:%d", funcID, line.Line, line.Column))
	}
	return strings.Join(parts, ";")
}

// stringToLine is a helper function to convert a string into a list of lines.
func stringToLine(lines string) ([]pprofile.Line, error) {
	if lines == "" {
		return []pprofile.Line{}, nil
	}

	parts := strings.Split(lines, ";")
	result := make([]pprofile.Line, 0, len(parts))

	for _, part := range parts {
		components := strings.Split(part, ":")
		if len(components) != 3 {
			return nil, fmt.Errorf("invalid line format '%s': %w", part, errInalIdxFomrat)
		}

		funcID, err := strconv.ParseInt(components[0], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse function ID '%s': %w", components[0], err)
		}

		lineNum, err := strconv.ParseInt(components[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse line number '%s': %w", components[1], err)
		}

		column, err := strconv.ParseInt(components[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse column '%s': %w", components[2], err)
		}

		line := pprofile.NewLine()
		line.SetFunctionIndex(int32(funcID))
		line.SetLine(lineNum)
		line.SetColumn(column)

		result = append(result, line)
	}
	return result, nil
}
