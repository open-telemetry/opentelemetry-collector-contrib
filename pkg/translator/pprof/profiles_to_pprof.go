// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprof // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/pprof/profile"
	"github.com/zeebo/xxh3"
	"go.opentelemetry.io/collector/pdata/pprofile"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// errNotFound is returned if something requested is not available
var errNotFound = errors.New("not found")

func convertPprofileToPprof(src *pprofile.Profiles) (*profile.Profile, error) {
	dst := &profile.Profile{}

	rp := src.ResourceProfiles()
	if rp.Len() != 1 {
		return nil, errors.New("multiple ResourceProfiles can not be embedded into a single pprof profile")
	}

	sp := rp.At(0).ScopeProfiles()
	if sp.Len() != 1 {
		return nil, errors.New("multiple ScopeProfiles can not be embedded into a single pprof profile")
	}

	pprofiles := sp.At(0).Profiles()
	numProfiles := pprofiles.Len()

	// Basic check that all profiles hold the same number of samples.
	numSamples := []int{}
	for _, p := range pprofiles.All() {
		numSamples = append(numSamples, p.Samples().Len())
	}

	if !allElementsSame(numSamples) {
		return nil, errors.New("profiles hold varying number of samples")
	}

	// Helper maps to avoid duplicates.
	functionMap := make(map[uint64]*profile.Function)
	mappingMap := make(map[uint64]*profile.Mapping)
	locationMap := make(map[uint64]*profile.Location)

	// Pre-populate all mappings from the mapping table to preserve all of them
	// (not just those referenced by locations) and maintain their original order.
	for i, m := range src.Dictionary().MappingTable().All() {
		if i == 0 {
			// Skip the zero-value mapping at index 0.
			continue
		}
		populateMapping(dst, mappingMap,
			m.MemoryStart(),
			m.MemoryLimit(),
			m.FileOffset(),
			getStringFromIdx(src.Dictionary(), int(m.FilenameStrindex())),
		)
	}

	// As all profiles hold the same number of samples,
	// assume they only differ on the sample type and therefore can
	// be merged into a pprof profile.
	p := sp.At(0).Profiles().At(0)
	var attrErr error

	// Convert profiles samples into pprof samples.
	for sampleIdx, s := range p.Samples().All() {
		pprofSample := profile.Sample{}
		si := s.StackIndex()
		stack := src.Dictionary().StackTable().At(int(si))

		for i := range numProfiles {
			if sp.At(0).Profiles().At(i).Samples().At(sampleIdx).Values().Len() != 1 {
				return nil, errors.New("invalid number of values per sample")
			}
			pprofSample.Value = append(pprofSample.Value, pprofiles.At(i).Samples().At(sampleIdx).Values().At(0))
		}

		for _, li := range stack.LocationIndices().All() {
			loc := src.Dictionary().LocationTable().At(int(li))
			var locMapping *profile.Mapping

			if mi := loc.MappingIndex(); mi != 0 {
				m := src.Dictionary().MappingTable().At(int(mi))
				locMapping = populateMapping(dst, mappingMap,
					m.MemoryStart(),
					m.MemoryLimit(),
					m.FileOffset(),
					getStringFromIdx(src.Dictionary(), int(m.FilenameStrindex())),
				)
			}

			var lines []profile.Line
			for _, l := range loc.Lines().All() {
				pprofLine := profile.Line{
					Line:   l.Line(),
					Column: l.Column(),
				}
				fn := src.Dictionary().FunctionTable().At(int(l.FunctionIndex()))

				pprofLine.Function = populateFunction(dst, functionMap,
					getStringFromIdx(src.Dictionary(), int(fn.NameStrindex())),
					getStringFromIdx(src.Dictionary(), int(fn.SystemNameStrindex())),
					getStringFromIdx(src.Dictionary(), int(fn.FilenameStrindex())),
					fn.StartLine())

				lines = append(lines, pprofLine)
			}

			pprofSample.Location = append(pprofSample.Location,
				populateLocation(dst, locationMap, locMapping, loc.Address(), lines))
		}

		// pprof.Sample.label is skipped for the moment.
		dst.Sample = append(dst.Sample, &pprofSample)
	}

	// Set pprof values that should be common across all profiles.
	for i := range numProfiles {
		dst.SampleType = append(dst.SampleType, &profile.ValueType{
			Type: getStringFromIdx(src.Dictionary(), int(pprofiles.At(i).SampleType().TypeStrindex())),
			Unit: getStringFromIdx(src.Dictionary(), int(pprofiles.At(i).SampleType().UnitStrindex())),
		})
	}

	var commentStrs []string
	commentStrs, attrErr = getAttributeStringWithPrefix(src.Dictionary(),
		string(semconv.PprofProfileCommentKey))
	if attrErr != nil && !errors.Is(attrErr, errNotFound) {
		return nil, attrErr
	}
	if len(commentStrs) != 0 {
		dst.Comments = append(dst.Comments, commentStrs...)
	}

	dst.KeepFrames, attrErr = getAttributeString(src.Dictionary(), "pprof.profile.keep_frames")
	if attrErr != nil && !errors.Is(attrErr, errNotFound) {
		return nil, attrErr
	}
	dst.DropFrames, attrErr = getAttributeString(src.Dictionary(), "pprof.profile.drop_frames")
	if attrErr != nil && !errors.Is(attrErr, errNotFound) {
		return nil, attrErr
	}
	dst.DocURL, attrErr = getAttributeString(src.Dictionary(), "pprof.profile.doc_url")
	if attrErr != nil && !errors.Is(attrErr, errNotFound) {
		return nil, attrErr
	}
	dst.TimeNanos = int64(p.Time().AsTime().Nanosecond())
	dst.DurationNanos = int64(p.DurationNano())
	dst.Period = p.Period()

	pt := p.PeriodType()
	dst.PeriodType = &profile.ValueType{
		Type: getStringFromIdx(src.Dictionary(), int(pt.TypeStrindex())),
		Unit: getStringFromIdx(src.Dictionary(), int(pt.UnitStrindex())),
	}

	if err := dst.CheckValid(); err != nil {
		return nil, err
	}

	return dst, nil
}

// allElementsSame checks if all elements in the provided slice are equal.
// It returns true if the slice contains at least one element and all elements are equal.
// It returns false otherwise.
func allElementsSame[T cmp.Ordered](in []T) bool {
	if len(in) == 0 {
		return false
	}
	slices.Sort(in)

	return in[0] == in[len(in)-1]
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

// getAttributeStringWithPrefix walks the attribute_table and returns a string array
// for all keys that start with the given prefix.
// It returns errNotFound if the key can not be found in attribute_table.
func getAttributeStringWithPrefix(dic pprofile.ProfilesDictionary, keyprefix string) ([]string, error) {
	tmp := make(map[int]string)

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

	result := make([]string, len(tmp))
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
	start, limit, offset uint64, fileName string,
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
		h.Write(uint64Tobytes(uint64(l.Column))) //nolint:errcheck
		h.Write(uint64Tobytes(uint64(l.Line)))   //nolint:errcheck
		fHash := getFunctionHash(l.Function.Name, l.Function.SystemName,
			l.Function.Filename, l.Function.StartLine)
		h.Write(uint64Tobytes(fHash)) //nolint:errcheck
	}
	return h.Sum64()
}

// populateLocation helps to populate deduplicated locations.
func populateLocation(dst *profile.Profile, locationMap map[uint64]*profile.Location,
	m *profile.Mapping, addr uint64, lines []profile.Line,
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

	dst.Location = append(dst.Location, pprofL)
	locationMap[lHash] = pprofL

	return pprofL
}
