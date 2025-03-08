// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"

import (
	"bytes"
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
)

// CompareProfiles compares each part of two given Profiles and returns
// an error if they don't match. The error describes what didn't match.
func CompareProfiles(expected, actual pprofile.Profiles, options ...CompareProfilesOption) error {
	exp, act := pprofile.NewProfiles(), pprofile.NewProfiles()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	for _, option := range options {
		option.applyOnProfiles(exp, act)
	}

	if exp.IsReadOnly() != act.IsReadOnly() {
		return fmt.Errorf("readOnly state differs from expected '%t'", exp.IsReadOnly())
	}

	if exp.SampleCount() != act.SampleCount() {
		return fmt.Errorf("sample count state differs from expected '%d', actual '%d'", exp.SampleCount(), act.SampleCount())
	}

	expectedProfiles, actualProfiles := exp.ResourceProfiles(), act.ResourceProfiles()
	if expectedProfiles.Len() != actualProfiles.Len() {
		return fmt.Errorf("number of resources doesn't match expected: %d, actual: %d",
			expectedProfiles.Len(), actualProfiles.Len())
	}

	numResources := expectedProfiles.Len()

	// Keep track of matching resources so that each can only be matched once
	matchingResources := make(map[pprofile.ResourceProfiles]pprofile.ResourceProfiles, numResources)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numResources; e++ {
		er := expectedProfiles.At(e)
		var foundMatch bool
		for a := 0; a < numResources; a++ {
			ar := actualProfiles.At(a)
			if _, ok := matchingResources[ar]; ok {
				continue
			}
			if reflect.DeepEqual(er.Resource().Attributes().AsRaw(), ar.Resource().Attributes().AsRaw()) {
				foundMatch = true
				matchingResources[ar] = er
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`resources are out of order: resource "%v" expected at index %d, found at index %d`,
							er.Resource().Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected resource: %v", er.Resource().Attributes().AsRaw()))
		}
	}

	for i := 0; i < numResources; i++ {
		if _, ok := matchingResources[actualProfiles.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected resource: %v", actualProfiles.At(i).Resource().Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for ar, er := range matchingResources {
		errPrefix := fmt.Sprintf(`resource "%v"`, er.Resource().Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareResourceProfiles(er, ar)))
	}

	return errs
}

// CompareResourceProfiles compares each part of two given ResourceProfiles and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceProfiles(expected, actual pprofile.ResourceProfiles) error {
	errs := multierr.Combine(
		internal.CompareResource(expected.Resource(), actual.Resource()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

	esls := expected.ScopeProfiles()
	asls := actual.ScopeProfiles()

	if esls.Len() != asls.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of scopes doesn't match expected: %d, actual: %d", esls.Len(),
			asls.Len()))
		return errs
	}

	numScopeProfiles := esls.Len()

	// Keep track of matching scope profiles so that each container can only be matched once
	matchingScopeProfiles := make(map[pprofile.ScopeProfiles]pprofile.ScopeProfiles, numScopeProfiles)

	var outOfOrderErrs error
	for e := 0; e < numScopeProfiles; e++ {
		esl := expected.ScopeProfiles().At(e)
		var foundMatch bool
		for a := 0; a < numScopeProfiles; a++ {
			asl := actual.ScopeProfiles().At(a)
			if _, ok := matchingScopeProfiles[asl]; ok {
				continue
			}
			if esl.Scope().Name() == asl.Scope().Name() {
				foundMatch = true
				matchingScopeProfiles[asl] = esl
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("scopes are out of order: scope %s expected at index %d, found at index %d",
							esl.Scope().Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected scope: %s", esl.Scope().Name()))
		}
	}

	for i := 0; i < numScopeProfiles; i++ {
		if _, ok := matchingScopeProfiles[actual.ScopeProfiles().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected scope: %s", actual.ScopeProfiles().At(i).Scope().Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for i := 0; i < esls.Len(); i++ {
		errPrefix := fmt.Sprintf(`scope "%s"`, esls.At(i).Scope().Name())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareScopeProfiles(esls.At(i), asls.At(i))))
	}

	return errs
}

// CompareScopeProfiles compares each part of two given ProfilesSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareScopeProfiles(expected, actual pprofile.ScopeProfiles) error {
	errs := multierr.Combine(
		internal.CompareInstrumentationScope(expected.Scope(), actual.Scope()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

	if expected.Profiles().Len() != actual.Profiles().Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of profiles doesn't match expected: %d, actual: %d",
			expected.Profiles().Len(), actual.Profiles().Len()))
		return errs
	}

	numProfiles := expected.Profiles().Len()

	// Keep track of matching containers so that each container can only be matched once
	matchingProfiles := make(map[pprofile.Profile]pprofile.Profile, numProfiles)

	var outOfOrderErrs error
	for e := 0; e < numProfiles; e++ {
		elr := expected.Profiles().At(e)
		errs = multierr.Append(errs, ValidateProfile(elr))
		em := profileAttributesToMap(elr)

		var foundMatch bool
		for a := 0; a < numProfiles; a++ {
			alr := actual.Profiles().At(a)
			errs = multierr.Append(errs, ValidateProfile(alr))
			if _, ok := matchingProfiles[alr]; ok {
				continue
			}
			am := profileAttributesToMap(alr)

			if reflect.DeepEqual(em, am) {
				foundMatch = true
				matchingProfiles[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`profiles are out of order: profile "%v" expected at index %d, found at index %d`,
							em, e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected profile: %v", em))
		}
	}

	for i := 0; i < numProfiles; i++ {
		if _, ok := matchingProfiles[actual.Profiles().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected profile: %v",
				profileAttributesToMap(actual.Profiles().At(i))))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingProfiles {
		errPrefix := fmt.Sprintf(`profile "%v"`, profileAttributesToMap(elr))
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfile(elr, alr)))
	}
	return errs
}

func compareAttributes(a, b pprofile.Profile) error {
	aa := profileAttributesToMap(a)
	ba := profileAttributesToMap(b)

	if !reflect.DeepEqual(aa, ba) {
		return fmt.Errorf("attributes don't match expected: %v, actual: %v", aa, ba)
	}

	return nil
}

func CompareProfile(expected, actual pprofile.Profile) error {
	errs := multierr.Combine(
		compareAttributes(expected, actual),
		internal.CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)

	if expected.ProfileID().String() != actual.ProfileID().String() {
		errs = multierr.Append(errs, fmt.Errorf("profileID does not match expected '%s', actual '%s'", expected.ProfileID().String(), actual.ProfileID().String()))
	}

	if expected.StartTime() != actual.StartTime() {
		errs = multierr.Append(errs, fmt.Errorf("start timestamp doesn't match expected: %d, "+"actual: %d", expected.StartTime(), actual.StartTime()))
	}

	if !reflect.DeepEqual(expected.LocationIndices(), actual.LocationIndices()) {
		errs = multierr.Append(errs, fmt.Errorf("locationIndicies do not match expected"))
	}

	if !reflect.DeepEqual(expected.CommentStrindices(), actual.CommentStrindices()) {
		errs = multierr.Append(errs, fmt.Errorf("comment does not match expected"))
	}

	if expected.Time() != actual.Time() {
		errs = multierr.Append(errs, fmt.Errorf("time doesn't match expected: %d, actual: %d", expected.Time(), actual.Time()))
	}

	if !reflect.DeepEqual(expected.StringTable(), actual.StringTable()) {
		errs = multierr.Append(errs, fmt.Errorf("stringTable '%v' does not match expected '%v'", actual.StringTable().AsRaw(), expected.StringTable().AsRaw()))
	}

	if expected.OriginalPayloadFormat() != actual.OriginalPayloadFormat() {
		errs = multierr.Append(errs, fmt.Errorf("originalPayloadFormat does not match expected '%s', actual '%s'", expected.OriginalPayloadFormat(), actual.OriginalPayloadFormat()))
	}

	if !bytes.Equal(expected.OriginalPayload().AsRaw(), actual.OriginalPayload().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("keepFrames does not match expected '%s', actual '%s'", expected.OriginalPayload().AsRaw(), actual.OriginalPayload().AsRaw()))
	}

	if expected.StartTime() != actual.StartTime() {
		errs = multierr.Append(errs, fmt.Errorf("startTime doesn't match expected: %d, actual: %d", expected.StartTime(), actual.StartTime()))
	}

	if expected.Duration() != actual.Duration() {
		errs = multierr.Append(errs, fmt.Errorf("duration doesn't match expected: %d, actual: %d", expected.Duration(), actual.Duration()))
	}

	if expected.Period() != actual.Period() {
		errs = multierr.Append(errs, fmt.Errorf("period does not match expected '%d', actual '%d'", expected.Period(), actual.Period()))
	}

	if expected.DefaultSampleTypeStrindex() != actual.DefaultSampleTypeStrindex() {
		errs = multierr.Append(errs, fmt.Errorf("defaultSampleType does not match expected '%d', actual '%d'", expected.DefaultSampleTypeStrindex(), actual.DefaultSampleTypeStrindex()))
	}

	if expected.PeriodType().TypeStrindex() != actual.PeriodType().TypeStrindex() ||
		expected.PeriodType().UnitStrindex() != actual.PeriodType().UnitStrindex() ||
		expected.PeriodType().AggregationTemporality() != actual.PeriodType().AggregationTemporality() {
		errs = multierr.Append(errs, fmt.Errorf("periodType does not match expected 'unit: %d, type: %d, aggregationTemporality: %d', actual 'unit: %d, type: %d,"+
			"aggregationTemporality: %d'", expected.PeriodType().UnitStrindex(), expected.PeriodType().TypeStrindex(), expected.PeriodType().AggregationTemporality(),
			actual.PeriodType().UnitStrindex(), actual.PeriodType().TypeStrindex(), actual.PeriodType().AggregationTemporality()))
	}

	errs = multierr.Append(errs, internal.AddErrPrefix("sampleType", CompareProfileValueTypeSlice(expected.SampleType(), actual.SampleType())))

	errs = multierr.Append(errs, internal.AddErrPrefix("sample", CompareProfileSampleSlice(expected.Sample(), actual.Sample())))

	errs = multierr.Append(errs, internal.AddErrPrefix("mapping", CompareProfileMappingSlice(expected.MappingTable(), actual.MappingTable())))

	errs = multierr.Append(errs, internal.AddErrPrefix("location", CompareProfileLocationSlice(expected.LocationTable(), actual.LocationTable())))

	errs = multierr.Append(errs, internal.AddErrPrefix("function", CompareProfileFunctionSlice(expected.FunctionTable(), actual.FunctionTable())))

	errs = multierr.Append(errs, internal.AddErrPrefix("attributeUnits", CompareProfileAttributeUnitSlice(expected.AttributeUnits(), actual.AttributeUnits())))

	errs = multierr.Append(errs, internal.AddErrPrefix("linkTable", CompareProfileLinkSlice(expected.LinkTable(), actual.LinkTable())))

	return errs
}

func CompareProfileValueTypeSlice(expected, actual pprofile.ValueTypeSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of valueTypes doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numValueTypes := expected.Len()

	matchingValueTypes := make(map[pprofile.ValueType]pprofile.ValueType, numValueTypes)

	var outOfOrderErrs error
	for e := 0; e < numValueTypes; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numValueTypes; a++ {
			alr := actual.At(a)
			if _, ok := matchingValueTypes[alr]; ok {
				continue
			}
			if elr.TypeStrindex() == alr.TypeStrindex() && elr.UnitStrindex() == alr.UnitStrindex() {
				foundMatch = true
				matchingValueTypes[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`valueTypes are out of order: valueType "unit: %d, type: %d, aggregationTemporality: %d" expected at index %d, found at index %d`,
							elr.UnitStrindex(), elr.TypeStrindex(), elr.AggregationTemporality(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected valueType "unit: %d, type: %d, aggregationTemporality: %d"`, elr.UnitStrindex(), elr.TypeStrindex(), elr.AggregationTemporality()))
		}
	}

	for i := 0; i < numValueTypes; i++ {
		if _, ok := matchingValueTypes[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected valueType "unit: %d, type: %d, aggregationTemporality: %d"`,
				actual.At(i).UnitStrindex(), actual.At(i).TypeStrindex(), actual.At(i).AggregationTemporality()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingValueTypes {
		if !isValueTypeEqual(elr, alr) {
			errs = multierr.Append(errs, fmt.Errorf(`expected valueType "unit: %d, type: %d, aggregationTemporality: %d",`+
				`got "unit: %d, type: %d, aggregationTemporality: %d"`, elr.UnitStrindex(), elr.TypeStrindex(), elr.AggregationTemporality(),
				alr.UnitStrindex(), alr.TypeStrindex(), alr.AggregationTemporality()))
		}
	}

	return errs
}

func isValueTypeEqual(expected, actual pprofile.ValueType) bool {
	return expected.TypeStrindex() == actual.TypeStrindex() &&
		expected.UnitStrindex() == actual.UnitStrindex() &&
		expected.AggregationTemporality() == actual.AggregationTemporality()
}

func CompareProfileSampleSlice(expected, actual pprofile.SampleSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of samples doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numSlice := expected.Len()

	matchingItems := make(map[pprofile.Sample]pprofile.Sample, numSlice)

	var outOfOrderErrs error
	for e := 0; e < numSlice; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numSlice; a++ {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if reflect.DeepEqual(elr.AttributeIndices().AsRaw(), alr.AttributeIndices().AsRaw()) {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`samples are out of order: sample "attributes: %v" expected at index %d, found at index %d`,
							elr.AttributeIndices().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected sample "attributes: %v"`, elr.AttributeIndices().AsRaw()))
		}
	}

	for i := 0; i < numSlice; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected sample "attributes: %v"`,
				actual.At(i).AttributeIndices().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingItems {
		errPrefix := fmt.Sprintf(`sample "attributes: %v"`, elr.AttributeIndices().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfileSample(elr, alr)))
	}

	return errs
}

func CompareProfileSample(expected, actual pprofile.Sample) error {
	var errs error
	if expected.LocationsStartIndex() != actual.LocationsStartIndex() {
		errs = multierr.Append(errs, fmt.Errorf("expected locationStartIndex '%d', got '%d'", expected.LocationsStartIndex(), actual.LocationsStartIndex()))
	}

	if expected.LocationsLength() != actual.LocationsLength() {
		errs = multierr.Append(errs, fmt.Errorf("expected locationLenght '%d', got '%d'", expected.LocationsLength(), actual.LocationsLength()))
	}

	if !reflect.DeepEqual(expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected timestampUnixNano '%v', got '%v'", expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()))
	}

	if !reflect.DeepEqual(expected.Value().AsRaw(), actual.Value().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected value '%v', got '%v'", expected.Value().AsRaw(), actual.Value().AsRaw()))
	}

	if !reflect.DeepEqual(expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected timestampUnixNano '%v', got '%v'", expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()))
	}

	if !reflect.DeepEqual(expected.AttributeIndices().AsRaw(), actual.AttributeIndices().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected attributes '%v', got '%v'", expected.AttributeIndices().AsRaw(), actual.AttributeIndices().AsRaw()))
	}

	return errs
}

func CompareProfileMappingSlice(expected, actual pprofile.MappingSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of mappings doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numItems := expected.Len()

	matchingItems := make(map[pprofile.Mapping]pprofile.Mapping, numItems)

	var outOfOrderErrs error
	for e := 0; e < numItems; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numItems; a++ {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if reflect.DeepEqual(elr.AttributeIndices().AsRaw(), alr.AttributeIndices().AsRaw()) {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`mappings are out of order: mapping "attributes: %v" expected at index %d, found at index %d`,
							elr.AttributeIndices().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected mapping "attributes: %v"`, elr.AttributeIndices().AsRaw()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile mapping "attributes: %v"`,
				actual.At(i).AttributeIndices().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingItems {
		if !isMappingEqual(elr, alr) {
			errs = multierr.Append(errs, fmt.Errorf(`mapping with "attributes: %v", does not match expected`,
				alr.AttributeIndices().AsRaw()))
		}
	}

	return errs
}

func isMappingEqual(expected, actual pprofile.Mapping) bool {
	return expected.MemoryStart() == actual.MemoryStart() &&
		expected.MemoryLimit() == actual.MemoryLimit() &&
		expected.FileOffset() == actual.FileOffset() &&
		expected.FilenameStrindex() == actual.FilenameStrindex() &&
		reflect.DeepEqual(expected.AttributeIndices().AsRaw(), actual.AttributeIndices().AsRaw()) &&
		expected.HasFunctions() == actual.HasFunctions() &&
		expected.HasFilenames() == actual.HasFilenames() &&
		expected.HasLineNumbers() == actual.HasLineNumbers() &&
		expected.HasInlineFrames() == actual.HasInlineFrames()
}

func CompareProfileFunctionSlice(expected, actual pprofile.FunctionSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of functions doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numItems := expected.Len()

	matchingItems := make(map[pprofile.Function]pprofile.Function, numItems)

	var outOfOrderErrs error
	for e := 0; e < numItems; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numItems; a++ {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if elr.NameStrindex() == alr.NameStrindex() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`functions are out of order: function "name: %d" expected at index %d, found at index %d`,
							elr.NameStrindex(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected function "name: %d"`, elr.NameStrindex()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile function "name: %d"`,
				actual.At(i).NameStrindex()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingItems {
		if !isFunctionEqual(elr, alr) {
			errs = multierr.Append(errs, fmt.Errorf(`function with "name: %d" does not match expected`, alr.NameStrindex()))
		}
	}

	return errs
}

func isFunctionEqual(expected, actual pprofile.Function) bool {
	return expected.NameStrindex() == actual.NameStrindex() &&
		expected.SystemNameStrindex() == actual.SystemNameStrindex() &&
		expected.StartLine() == actual.StartLine() &&
		expected.FilenameStrindex() == actual.FilenameStrindex()
}

func CompareProfileLocationSlice(expected, actual pprofile.LocationSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of locations doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numItems := expected.Len()

	matchingItems := make(map[pprofile.Location]pprofile.Location, numItems)

	var outOfOrderErrs error
	for e := 0; e < numItems; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numItems; a++ {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if reflect.DeepEqual(elr.AttributeIndices().AsRaw(), alr.AttributeIndices().AsRaw()) {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`locations are out of order: location "attributes: %v" expected at index %d, found at index %d`,
							elr.AttributeIndices().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected location "attributes: %v"`, elr.AttributeIndices().AsRaw()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected location "attributes: %v"`,
				actual.At(i).AttributeIndices().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingItems {
		errPrefix := fmt.Sprintf(`location "attributes: %v"`, elr.AttributeIndices().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfileLocation(elr, alr)))
	}

	return errs
}

func CompareProfileLocation(expected, actual pprofile.Location) error {
	var errs error

	if expected.MappingIndex() != actual.MappingIndex() {
		errs = multierr.Append(errs, fmt.Errorf("expected mappingIndex '%d', got '%d'", expected.MappingIndex(), actual.MappingIndex()))
	}

	if expected.Address() != actual.Address() {
		errs = multierr.Append(errs, fmt.Errorf("expected address '%d', got '%d'", expected.Address(), actual.Address()))
	}

	if expected.IsFolded() != actual.IsFolded() {
		errs = multierr.Append(errs, fmt.Errorf("expected isFolded '%v', got '%v'", expected.IsFolded(), actual.IsFolded()))
	}

	if !reflect.DeepEqual(expected.AttributeIndices().AsRaw(), actual.AttributeIndices().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected attributes '%v', got '%v'", expected.AttributeIndices().AsRaw(), actual.AttributeIndices().AsRaw()))
	}

	errPrefix := fmt.Sprintf(`line of location with "attributes: %v"`, expected.AttributeIndices().AsRaw())
	errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfileLineSlice(expected.Line(), actual.Line())))

	return errs
}

func CompareProfileLineSlice(expected, actual pprofile.LineSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of lines doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numItems := expected.Len()

	matchingItems := make(map[pprofile.Line]pprofile.Line, numItems)

	var outOfOrderErrs error
	for e := 0; e < numItems; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numItems; a++ {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if elr.FunctionIndex() == alr.FunctionIndex() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`lines are out of order: line "functionIndex: %d" expected at index %d, found at index %d`,
							elr.FunctionIndex(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected line "functionIndex: %d"`, elr.FunctionIndex()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile line "functionIndex: %d"`,
				actual.At(i).FunctionIndex()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingItems {
		if !isLineEqual(elr, alr) {
			errs = multierr.Append(errs, fmt.Errorf(`line with "functionIndex: %d" does not match expected`, alr.FunctionIndex()))
		}
	}

	return errs
}

func isLineEqual(expected, actual pprofile.Line) bool {
	return expected.FunctionIndex() == actual.FunctionIndex() &&
		expected.Line() == actual.Line() &&
		expected.Column() == actual.Column()
}

func CompareProfileAttributeUnitSlice(expected, actual pprofile.AttributeUnitSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of attributeUnits doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numItems := expected.Len()

	matchingItems := make(map[pprofile.AttributeUnit]pprofile.AttributeUnit, numItems)

	var outOfOrderErrs error
	for e := 0; e < numItems; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numItems; a++ {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if elr.AttributeKeyStrindex() == alr.AttributeKeyStrindex() && elr.UnitStrindex() == alr.UnitStrindex() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`attributeUnits are out of order: attributeUnit "attributeKey: %d" expected at index %d, found at index %d`,
							elr.AttributeKeyStrindex(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected attributeUnit "attributeKey: %d"`, elr.AttributeKeyStrindex()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile attributeUnit "attributeKey: %d"`,
				actual.At(i).AttributeKeyStrindex()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	return errs
}

func CompareProfileLinkSlice(expected, actual pprofile.LinkSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of links doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numItems := expected.Len()

	matchingItems := make(map[pprofile.Link]pprofile.Link, numItems)

	var outOfOrderErrs error
	for e := 0; e < numItems; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numItems; a++ {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if elr.TraceID().String() == alr.TraceID().String() && elr.SpanID().String() == alr.SpanID().String() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`links are out of order: link "spanId: %s, traceId: %s" expected at index %d, found at index %d`,
							elr.SpanID().String(), elr.TraceID().String(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected link "spanId: %s, traceId: %s"`, elr.SpanID().String(), elr.TraceID().String()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile link "spanId: %s, traceId: %s"`,
				actual.At(i).SpanID().String(), actual.At(i).TraceID().String()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	return errs
}
