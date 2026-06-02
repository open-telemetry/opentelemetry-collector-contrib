// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"

// this file is a slightly different version of wh,
// setting additional Splunk-specific labels on each sample.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/pprof/profile"
	"github.com/zeebo/xxh3"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// errNotFound is returned if something requested is not available
var errNotFound = errors.New("not found")

func ConvertPprofileToPprof(dict pprofile.ProfilesDictionary, scope pcommon.InstrumentationScope, p pprofile.Profile) (*profile.Profile, error) {
	dst := &profile.Profile{}

	// Helper maps to avoid duplicates.
	functionMap := make(map[uint64]*profile.Function)
	mappingMap := make(map[uint64]*profile.Mapping)
	locationMap := make(map[uint64]*profile.Location)

	// Pre-populate all mappings from the mapping table to preserve all of them
	// (not just those referenced by locations) and maintain their original order.
	for i, m := range dict.MappingTable().All() {
		if i == 0 {
			// Skip the zero-value mapping at index 0.
			continue
		}
		populateMapping(dst, mappingMap,
			m.MemoryStart(),
			m.MemoryLimit(),
			m.FileOffset(),
			getStringFromIdx(dict, int(m.FilenameStrindex())),
			dict,
			m,
		)
	}

	sampleType := dict.StringTable().At(int(p.SampleType().TypeStrindex()))

	// As all profiles hold the same number of samples,
	// assume they only differ on the sample type and therefore can
	// be merged into a pprof profile.
	var attrErr error

	// Compute profileTime once for all samples; fall back to now only when the profile has no timestamp.
	profileTime := p.Time().AsTime()
	if p.Time().AsTime().UnixNano() == 0 {
		profileTime = time.Now()
	}

	// Convert profiles samples into pprof samples.
	for _, s := range p.Samples().All() {
		pprofSample := profile.Sample{
			Label: map[string][]string{
				"source.event.name": {scope.Name()},
			},
			NumLabel: map[string][]int64{
				"source.event.time": {profileTime.UnixMilli()},
			},
			Value: s.Values().AsRaw(),
		}
		if li := s.LinkIndex(); li > 0 && int(li) < dict.LinkTable().Len() {
			link := dict.LinkTable().At(int(li))
			pprofSample.Label["span_id"] = []string{link.SpanID().String()}
			pprofSample.Label["trace_id"] = []string{link.TraceID().String()}
		}
		si := s.StackIndex()
		stack := dict.StackTable().At(int(si))

		for _, li := range stack.LocationIndices().All() {
			loc := dict.LocationTable().At(int(li))
			var locMapping *profile.Mapping

			if mi := loc.MappingIndex(); mi != 0 {
				m := dict.MappingTable().At(int(mi))
				locMapping = populateMapping(dst, mappingMap,
					m.MemoryStart(),
					m.MemoryLimit(),
					m.FileOffset(),
					getStringFromIdx(dict, int(m.FilenameStrindex())),
					dict,
					m,
				)
			}

			var lines []profile.Line
			for _, l := range loc.Lines().All() {
				pprofLine := profile.Line{
					Line: l.Line(),
				}
				fn := dict.FunctionTable().At(int(l.FunctionIndex()))

				pprofLine.Function = populateFunction(dst, functionMap,
					getStringFromIdx(dict, int(fn.NameStrindex())),
					getStringFromIdx(dict, int(fn.SystemNameStrindex())),
					getStringFromIdx(dict, int(fn.FilenameStrindex())),
					fn.StartLine())

				lines = append(lines, pprofLine)
			}
			// label source.event.name of type string OPTIONALLY can contain the name of the event that triggered the sampling
			// label source.event.time of type int64 MUST be set to the unix time in millis when the sample was taken
			// label trace_id of type string MUST be set when sample was taken within a span scope
			// label span_id of type string MUST be set when sample was taken within a span scope
			// label thread.id of type int64 OPTIONALLY can be set to the thread identifier used by the runtime environment
			// label thread.name of type string OPTIONALLY can be set to the thread name used by the runtime environment
			// label thread.os.id of type int64 OPTIONALLY can be set to the thread identifier used by the operating system
			// label thread.stack.truncated of type string and with value true MUST be set when this sample does not contain the full stack trace

			pprofSample.Location = append(pprofSample.Location,
				populateLocation(dst, locationMap, locMapping, loc.Address(), lines, dict, loc))
		}

		if sampleType == "cpu" {
			pprofSample.Label["source.event.period"] = []string{strconv.Itoa(int(p.Period()))}
		}

		dst.Sample = append(dst.Sample, &pprofSample)
	}

	sampleUnit := dict.StringTable().At(int(p.SampleType().UnitStrindex()))
	nValues := 1
	if p.Samples().Len() > 0 {
		nValues = max(1, p.Samples().At(0).Values().Len())
	}
	for range nValues {
		dst.SampleType = append(dst.SampleType, &profile.ValueType{Type: sampleType, Unit: sampleUnit})
	}

	var commentStrs []string
	commentStrs, attrErr = getAttributeStringWithPrefix(dict)
	if attrErr != nil && !errors.Is(attrErr, errNotFound) {
		return nil, attrErr
	}
	if len(commentStrs) != 0 {
		dst.Comments = append(dst.Comments, commentStrs...)
	}

	dst.KeepFrames, attrErr = getAttributeString(dict, string(semconv.PprofProfileKeepFramesKey))
	if attrErr != nil && !errors.Is(attrErr, errNotFound) {
		return nil, attrErr
	}
	dst.DropFrames, attrErr = getAttributeString(dict, string(semconv.PprofProfileDropFramesKey))
	if attrErr != nil && !errors.Is(attrErr, errNotFound) {
		return nil, attrErr
	}
	dst.TimeNanos = p.Time().AsTime().UnixNano()
	dst.DurationNanos = int64(p.DurationNano())
	dst.Period = p.Period()

	pt := p.PeriodType()
	dst.PeriodType = &profile.ValueType{
		Type: getStringFromIdx(dict, int(pt.TypeStrindex())),
		Unit: getStringFromIdx(dict, int(pt.UnitStrindex())),
	}

	if err := checkValid(dst); err != nil {
		return nil, err
	}

	return dst, nil
}

// checkValid tests whether the profile is valid. Checks include, but are
// not limited to:
//   - len(Profile.Sample[n].value) == len(Profile.value_unit)
//   - Sample.id has a corresponding Profile.Location
func checkValid(p *profile.Profile) error {
	// Check that sample values are consistent
	sampleLen := len(p.SampleType)
	if sampleLen == 0 && len(p.Sample) != 0 {
		return errors.New("missing sample type information")
	}
	for _, s := range p.Sample {
		if s == nil {
			return errors.New("profile has nil sample")
		}
		if len(s.Value) != sampleLen {
			return fmt.Errorf("mismatch: sample has %d values vs. %d types", len(s.Value), len(p.SampleType))
		}
		for _, l := range s.Location {
			if l == nil {
				return errors.New("sample has nil location")
			}
		}
	}

	// Check that all mappings/locations/functions are in the tables
	// Check that there are no duplicate ids
	mappings := make(map[uint64]*profile.Mapping, len(p.Mapping))
	for _, m := range p.Mapping {
		if m == nil {
			return errors.New("profile has nil mapping")
		}
		if m.ID == 0 {
			return errors.New("found mapping with reserved ID=0")
		}
		if mappings[m.ID] != nil {
			return fmt.Errorf("multiple mappings with same id: %d", m.ID)
		}
		mappings[m.ID] = m
	}
	functions := make(map[uint64]*profile.Function, len(p.Function))
	for _, f := range p.Function {
		if f == nil {
			return errors.New("profile has nil function")
		}
		if f.ID == 0 {
			return errors.New("found function with reserved ID=0")
		}
		if functions[f.ID] != nil {
			return fmt.Errorf("multiple functions with same id: %d", f.ID)
		}
		functions[f.ID] = f
	}
	locations := make(map[uint64]*profile.Location, len(p.Location))
	for _, l := range p.Location {
		if l == nil {
			return errors.New("profile has nil location")
		}
		if l.ID == 0 {
			return errors.New("found location with reserved id=0")
		}
		if locations[l.ID] != nil {
			return fmt.Errorf("multiple locations with same id: %d", l.ID)
		}
		locations[l.ID] = l
		if m := l.Mapping; m != nil {
			if m.ID == 0 || mappings[m.ID] != m {
				return fmt.Errorf("inconsistent mapping %p: %d", m, m.ID)
			}
		}
		for _, ln := range l.Line {
			f := ln.Function
			if f == nil {
				return fmt.Errorf("location id: %d has a line with nil function", l.ID)
			}
			if f.ID == 0 || functions[f.ID] != f {
				return fmt.Errorf("inconsistent function %p: %d", f, f.ID)
			}
		}
	}
	return nil
}

// getAttributeString walks the attribute_table and returns the string value for
// the given key.
// It returns errNotFound if the key can not be found in attribute_table.
func getAttributeString(dic pprofile.ProfilesDictionary, key string) (string, error) {
	for _, attr := range dic.AttributeTable().All() {
		attrKey := getStringFromIdx(dic, int(attr.KeyStrindex()))
		if attrKey == key {
			return attr.Value().AsString(), nil
		}
	}
	return "", fmt.Errorf("attribute with key '%s': %w", key, errNotFound)
}

// getAttributeBool walks the attribute_table for the given indices and returns the bool value
// for the given key.
// It returns errNotFound if the key can not be found in attribute_table.
func getAttributeBool(dic pprofile.ProfilesDictionary, attrIndices []int32, key string) (bool, error) {
	for _, idx := range attrIndices {
		if idx == 0 || int(idx) >= dic.AttributeTable().Len() {
			continue
		}
		attr := dic.AttributeTable().At(int(idx))
		attrKey := getStringFromIdx(dic, int(attr.KeyStrindex()))
		if attrKey == key {
			return attr.Value().Bool(), nil
		}
	}
	return false, fmt.Errorf("attribute with key '%s': %w", key, errNotFound)
}

// getAttributeStringWithPrefix walks the attribute_table and returns a string array
// for all keys that start with the prefix pprof.profile.comment.
// It returns errNotFound if the key can not be found in attribute_table.
func getAttributeStringWithPrefix(dic pprofile.ProfilesDictionary) ([]string, error) {
	tmp := make(map[int]string)
	keyprefix := string(semconv.PprofProfileCommentKey)

	for _, attr := range dic.AttributeTable().All() {
		attrKey := getStringFromIdx(dic, int(attr.KeyStrindex()))
		attrIdxStr, found := strings.CutPrefix(attrKey, keyprefix+".")
		if !found {
			continue
		}
		attrIdx, err := strconv.Atoi(attrIdxStr)
		if err != nil {
			return []string{}, err
		}
		tmp[attrIdx] = attr.Value().AsString()
	}

	if len(tmp) == 0 {
		return []string{}, fmt.Errorf("attribute with prefix '%s': %w", keyprefix, errNotFound)
	}

	maxIdx := 0
	for k := range tmp {
		if k > maxIdx {
			maxIdx = k
		}
	}
	result := make([]string, maxIdx+1)
	for k, v := range tmp {
		result[k] = v
	}

	return result, nil
}

// getStringFromIdx returns the string on idx.
func getStringFromIdx(dic pprofile.ProfilesDictionary, idx int) string {
	return dic.StringTable().At(idx)
}

// uint64Tobytes helps to convert uint64 to []byte.
func uint64Tobytes(v uint64) []byte {
	return []byte{
		byte(v >> 56),
		byte(v >> 48),
		byte(v >> 40),
		byte(v >> 32),
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	}
}

// getFunctionHash returns a non-cryptographic hash as identifier for a Function.
func getFunctionHash(name, systemName, fileName string, startLine int64) uint64 {
	h := xxh3.New()
	h.WriteString(name)                       //nolint:errcheck
	h.WriteString(systemName)                 //nolint:errcheck
	h.WriteString(fileName)                   //nolint:errcheck
	h.Write(uint64Tobytes(uint64(startLine))) //nolint:errcheck
	return h.Sum64()
}

// populateFunction helps to populate deduplicated functions.
func populateFunction(dst *profile.Profile, functionMap map[uint64]*profile.Function,
	name, systemName, fileName string, startLine int64,
) *profile.Function {
	fnHash := getFunctionHash(name, systemName, fileName, startLine)
	if existingFn, ok := functionMap[fnHash]; ok {
		return existingFn
	}

	pprofFn := &profile.Function{
		ID:         uint64(len(dst.Function) + 1),
		Name:       name,
		SystemName: systemName,
		Filename:   fileName,
		StartLine:  startLine,
	}

	dst.Function = append(dst.Function, pprofFn)
	functionMap[fnHash] = pprofFn

	return pprofFn
}

// getMappingHash returns a non-cryptographic hash as identifier for a Mapping.
func getMappingHash(start, limit, offset uint64, file string) uint64 {
	h := xxh3.New()
	h.Write(uint64Tobytes(start))  //nolint:errcheck
	h.Write(uint64Tobytes(limit))  //nolint:errcheck
	h.Write(uint64Tobytes(offset)) //nolint:errcheck
	h.WriteString(file)            //nolint:errcheck
	return h.Sum64()
}

// populateMapping helps to populate deduplicated mappings.
func populateMapping(dst *profile.Profile, mappingMap map[uint64]*profile.Mapping,
	start, limit, offset uint64, fileName string, dic pprofile.ProfilesDictionary, m pprofile.Mapping,
) *profile.Mapping {
	mHash := getMappingHash(start, limit, offset, fileName)
	if existingMapping, ok := mappingMap[mHash]; ok {
		return existingMapping
	}

	pprofM := &profile.Mapping{
		ID:     uint64(len(dst.Mapping) + 1),
		Start:  start,
		Limit:  limit,
		Offset: offset,
		File:   fileName,
	}

	// Extract mapping attributes from the OTel profile
	var attrIndices []int32
	for _, idx := range m.AttributeIndices().All() {
		attrIndices = append(attrIndices, idx)
	}

	// Extract BuildID from attributes
	for _, idx := range attrIndices {
		if idx == 0 || int(idx) >= dic.AttributeTable().Len() {
			continue
		}
		attr := dic.AttributeTable().At(int(idx))
		attrKey := getStringFromIdx(dic, int(attr.KeyStrindex()))
		if attrKey == string(semconv.ProcessExecutableBuildIDGNUKey) {
			pprofM.BuildID = attr.Value().AsString()
			break
		}
	}

	// Extract boolean mapping flags
	hasFunctions, err := getAttributeBool(dic, attrIndices, string(semconv.PprofMappingHasFunctionsKey))
	if err == nil {
		pprofM.HasFunctions = hasFunctions
	}
	hasFilenames, err := getAttributeBool(dic, attrIndices, string(semconv.PprofMappingHasFilenamesKey))
	if err == nil {
		pprofM.HasFilenames = hasFilenames
	}
	hasLineNumbers, err := getAttributeBool(dic, attrIndices, string(semconv.PprofMappingHasLineNumbersKey))
	if err == nil {
		pprofM.HasLineNumbers = hasLineNumbers
	}
	hasInlineFrames, err := getAttributeBool(dic, attrIndices, string(semconv.PprofMappingHasInlineFramesKey))
	if err == nil {
		pprofM.HasInlineFrames = hasInlineFrames
	}

	dst.Mapping = append(dst.Mapping, pprofM)
	mappingMap[mHash] = pprofM

	return pprofM
}

// getLocationHash returns a non-cryptographic hash as identifier for a Location.
func getLocationHash(m *profile.Mapping, addr uint64, lines []profile.Line) uint64 {
	h := xxh3.New()
	if m != nil {
		mHash := getMappingHash(m.Start, m.Limit, m.Offset, m.File)
		h.Write(uint64Tobytes(mHash)) //nolint:errcheck
	}
	h.Write(uint64Tobytes(addr)) //nolint:errcheck
	for _, l := range lines {
		h.Write(uint64Tobytes(uint64(l.Line))) //nolint:errcheck
		fHash := getFunctionHash(l.Function.Name, l.Function.SystemName,
			l.Function.Filename, l.Function.StartLine)
		h.Write(uint64Tobytes(fHash)) //nolint:errcheck
	}
	return h.Sum64()
}

// populateLocation helps to populate deduplicated locations.
func populateLocation(dst *profile.Profile, locationMap map[uint64]*profile.Location,
	m *profile.Mapping, addr uint64, lines []profile.Line, dic pprofile.ProfilesDictionary, loc pprofile.Location,
) *profile.Location {
	lHash := getLocationHash(m, addr, lines)
	if existingLocation, ok := locationMap[lHash]; ok {
		return existingLocation
	}

	pprofL := &profile.Location{
		ID:      uint64(len(dst.Location) + 1),
		Mapping: m,
		Address: addr,
		Line:    lines,
	}

	// Extract location attributes from the OTel profile
	var attrIndices []int32
	for _, idx := range loc.AttributeIndices().All() {
		attrIndices = append(attrIndices, idx)
	}

	// Extract IsFolded from attributes
	isFolded, err := getAttributeBool(dic, attrIndices, string(semconv.PprofLocationIsFoldedKey))
	if err == nil {
		pprofL.IsFolded = isFolded
	}

	dst.Location = append(dst.Location, pprofL)
	locationMap[lHash] = pprofL

	return pprofL
}
