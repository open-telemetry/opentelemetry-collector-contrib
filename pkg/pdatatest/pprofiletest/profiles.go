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
	for e := range numResources {
		er := expectedProfiles.At(e)
		var foundMatch bool
		for a := range numResources {
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

	for i := range numResources {
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
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareResourceProfiles(exp.Dictionary(), act.Dictionary(), er, ar)))
	}

	return errs
}

// CompareResourceProfiles compares each part of two given ResourceProfiles and returns
// an error if they don't match. The error describes what didn't match.
func CompareResourceProfiles(expectedDic, actualDic pprofile.ProfilesDictionary, expected, actual pprofile.ResourceProfiles) error {
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
	for e := range numScopeProfiles {
		esl := expected.ScopeProfiles().At(e)
		var foundMatch bool
		for a := range numScopeProfiles {
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

	for i := range numScopeProfiles {
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
		errPrefix := fmt.Sprintf(`scope %q`, esls.At(i).Scope().Name())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareScopeProfiles(expectedDic, actualDic, esls.At(i), asls.At(i))))
	}

	return errs
}

// CompareScopeProfiles compares each part of two given ProfilesSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareScopeProfiles(expectedDic, actualDic pprofile.ProfilesDictionary, expected, actual pprofile.ScopeProfiles) error {
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
	for e := range numProfiles {
		elr := expected.Profiles().At(e)
		errs = multierr.Append(errs, ValidateProfile(expectedDic, elr))
		em := profileAttributesToMap(expectedDic, elr)

		var foundMatch bool
		for a := range numProfiles {
			alr := actual.Profiles().At(a)
			errs = multierr.Append(errs, ValidateProfile(actualDic, alr))
			if _, ok := matchingProfiles[alr]; ok {
				continue
			}
			am := profileAttributesToMap(actualDic, alr)

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

	for i := range numProfiles {
		if _, ok := matchingProfiles[actual.Profiles().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected profile: %v",
				profileAttributesToMap(actualDic, actual.Profiles().At(i))))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingProfiles {
		errPrefix := fmt.Sprintf(`profile "%v"`, profileAttributesToMap(expectedDic, elr))
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfile(expectedDic, actualDic, elr, alr)))
	}
	return errs
}

func compareAttributes(expectedDic, actualDic pprofile.ProfilesDictionary, a, b pprofile.Profile) error {
	aa := profileAttributesToMap(expectedDic, a)
	ba := profileAttributesToMap(actualDic, b)

	if !reflect.DeepEqual(aa, ba) {
		return fmt.Errorf("attributes don't match expected: %v, actual: %v", aa, ba)
	}

	return nil
}

func CompareProfile(expectedDic, actualDic pprofile.ProfilesDictionary, expected, actual pprofile.Profile) error {
	errs := multierr.Combine(
		compareAttributes(expectedDic, actualDic, expected, actual),
		internal.CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)

	if expected.ProfileID().String() != actual.ProfileID().String() {
		errs = multierr.Append(errs, fmt.Errorf("profileID does not match expected '%s', actual '%s'", expected.ProfileID().String(), actual.ProfileID().String()))
	}

	if expected.Time() != actual.Time() {
		errs = multierr.Append(errs, fmt.Errorf("time doesn't match expected: %d, actual: %d", expected.Time(), actual.Time()))
	}

	if expected.OriginalPayloadFormat() != actual.OriginalPayloadFormat() {
		errs = multierr.Append(errs, fmt.Errorf("originalPayloadFormat does not match expected '%s', actual '%s'", expected.OriginalPayloadFormat(), actual.OriginalPayloadFormat()))
	}

	if !bytes.Equal(expected.OriginalPayload().AsRaw(), actual.OriginalPayload().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("keepFrames does not match expected '%s', actual '%s'", expected.OriginalPayload().AsRaw(), actual.OriginalPayload().AsRaw()))
	}

	if expected.Duration() != actual.Duration() {
		errs = multierr.Append(errs, fmt.Errorf("duration doesn't match expected: %d, actual: %d", expected.Duration(), actual.Duration()))
	}

	if expected.Period() != actual.Period() {
		errs = multierr.Append(errs, fmt.Errorf("period does not match expected '%d', actual '%d'", expected.Period(), actual.Period()))
	}

	if expected.PeriodType().TypeStrindex() != actual.PeriodType().TypeStrindex() ||
		expected.PeriodType().UnitStrindex() != actual.PeriodType().UnitStrindex() {
		errs = multierr.Append(errs, fmt.Errorf("periodType does not match expected 'unit: %d, type: %d', actual 'unit: %d, type: %d,", expected.PeriodType().UnitStrindex(), expected.PeriodType().TypeStrindex(),
			actual.PeriodType().UnitStrindex(), actual.PeriodType().TypeStrindex()))
	}

	errs = multierr.Append(errs, internal.AddErrPrefix("sampleType", CompareProfileValueType(expected.SampleType(), actual.SampleType())))

	errs = multierr.Append(errs, internal.AddErrPrefix("sample", CompareProfileSampleSlice(expected.Samples(), actual.Samples())))

	return errs
}

func CompareProfileValueType(expected, actual pprofile.ValueType) error {
	if !isValueTypeEqual(expected, actual) {
		return fmt.Errorf(`expected valueType "unit: %d, type: %d",`+
			`got "unit: %d, type: %d"`, expected.UnitStrindex(), expected.TypeStrindex(),
			actual.UnitStrindex(), actual.TypeStrindex())
	}

	return nil
}

func isValueTypeEqual(expected, actual pprofile.ValueType) bool {
	return expected.TypeStrindex() == actual.TypeStrindex() &&
		expected.UnitStrindex() == actual.UnitStrindex()
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
	for e := range numSlice {
		elr := expected.At(e)
		var foundMatch bool
		for a := range numSlice {
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

	for i := range numSlice {
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

	if !reflect.DeepEqual(expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected timestampUnixNano '%v', got '%v'", expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()))
	}

	if !reflect.DeepEqual(expected.Values().AsRaw(), actual.Values().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected values '%v', got '%v'", expected.Values().AsRaw(), actual.Values().AsRaw()))
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
	for e := range numItems {
		elr := expected.At(e)
		var foundMatch bool
		for a := range numItems {
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

	for i := range numItems {
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
		if !elr.Equal(alr) {
			errs = multierr.Append(errs, fmt.Errorf(`mapping with "attributes: %v", does not match expected`,
				alr.AttributeIndices().AsRaw()))
		}
	}

	return errs
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
	for e := range numItems {
		elr := expected.At(e)
		var foundMatch bool
		for a := range numItems {
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

	for i := range numItems {
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
	for e := range numItems {
		elr := expected.At(e)
		var foundMatch bool
		for a := range numItems {
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

	for i := range numItems {
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

	if !reflect.DeepEqual(expected.AttributeIndices().AsRaw(), actual.AttributeIndices().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected attributes '%v', got '%v'", expected.AttributeIndices().AsRaw(), actual.AttributeIndices().AsRaw()))
	}

	errPrefix := fmt.Sprintf(`line of location with "attributes: %v"`, expected.AttributeIndices().AsRaw())
	errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfileLineSlice(expected.Lines(), actual.Lines())))

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
	for e := range numItems {
		elr := expected.At(e)
		var foundMatch bool
		for a := range numItems {
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

	for i := range numItems {
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

func CompareKeyValueAndUnitSlice(expected, actual pprofile.KeyValueAndUnitSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of keyValueAndUnits doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numItems := expected.Len()

	matchingItems := make(map[pprofile.KeyValueAndUnit]pprofile.KeyValueAndUnit, numItems)

	var outOfOrderErrs error
	for e := range numItems {
		elr := expected.At(e)
		var foundMatch bool
		for a := range numItems {
			alr := actual.At(a)
			if _, ok := matchingItems[alr]; ok {
				continue
			}
			if elr.KeyStrindex() == alr.KeyStrindex() && elr.Value().AsRaw() == alr.Value().AsRaw() && elr.UnitStrindex() == alr.UnitStrindex() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`keyValueAndUnits are out of order: keyValueAndUnit "key: %d" expected at index %d, found at index %d`,
							elr.KeyStrindex(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected keyValueAndUnit "key: %d"`, elr.KeyStrindex()))
		}
	}

	for i := range numItems {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile keyValueAndUnit "key: %d"`,
				actual.At(i).KeyStrindex()))
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
	for e := range numItems {
		elr := expected.At(e)
		var foundMatch bool
		for a := range numItems {
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

	for i := range numItems {
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
