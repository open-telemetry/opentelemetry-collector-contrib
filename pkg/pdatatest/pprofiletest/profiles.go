// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"

import (
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
		errs = multierr.Append(errs, fmt.Errorf("number of profile containers doesn't match expected: %d, actual: %d",
			expected.Profiles().Len(), actual.Profiles().Len()))
		return errs
	}

	numProfiles := expected.Profiles().Len()

	// Keep track of matching containers so that each container can only be matched once
	matchingProfiles := make(map[pprofile.ProfileContainer]pprofile.ProfileContainer, numProfiles)

	var outOfOrderErrs error
	for e := 0; e < numProfiles; e++ {
		elr := expected.Profiles().At(e)
		var foundMatch bool
		for a := 0; a < numProfiles; a++ {
			alr := actual.Profiles().At(a)
			if _, ok := matchingProfiles[alr]; ok {
				continue
			}
			if reflect.DeepEqual(elr.Attributes().AsRaw(), alr.Attributes().AsRaw()) {
				foundMatch = true
				matchingProfiles[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`profile containers are out of order: profile container "%v" expected at index %d, found at index %d`,
							elr.Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected profile container: %v", elr.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numProfiles; i++ {
		if _, ok := matchingProfiles[actual.Profiles().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected profile container: %v",
				actual.Profiles().At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingProfiles {
		errPrefix := fmt.Sprintf(`profile container "%v"`, elr.Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfileContainer(elr, alr)))
	}
	return errs
}

func CompareProfileContainer(expected, actual pprofile.ProfileContainer) error {
	errs := multierr.Combine(
		internal.CompareAttributes(expected.Attributes(), actual.Attributes()),
		internal.CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)

	if expected.ProfileID().String() != actual.ProfileID().String() {
		errs = multierr.Append(errs, fmt.Errorf("profileID does not match expected '%s', actual '%s'", expected.ProfileID().String(), actual.ProfileID().String()))
	}

	if expected.StartTime() != actual.StartTime() {
		errs = multierr.Append(errs, fmt.Errorf("start timestamp doesn't match expected: %d, "+"actual: %d", expected.StartTime(), actual.StartTime()))
	}

	if expected.EndTime() != actual.EndTime() {
		errs = multierr.Append(errs, fmt.Errorf("end timestamp doesn't match expected: %d, "+"actual: %d", expected.EndTime(), actual.EndTime()))
	}

	errPrefix := fmt.Sprintf(`profile "%v"`, expected.Attributes().AsRaw())
	errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfile(expected.Profile(), actual.Profile())))

	return errs
}

func CompareProfile(expected, actual pprofile.Profile) error {
	errs := multierr.Combine(
		internal.CompareAttributes(expected.AttributeTable(), actual.AttributeTable()),
	)

	if !reflect.DeepEqual(expected.LocationIndices(), actual.LocationIndices()) {
		errs = multierr.Append(errs, fmt.Errorf("locationIndicies do not match expected"))
	}

	if !reflect.DeepEqual(expected.Comment(), actual.Comment()) {
		errs = multierr.Append(errs, fmt.Errorf("comment does not match expected"))
	}

	if !reflect.DeepEqual(expected.StringTable(), actual.StringTable()) {
		errs = multierr.Append(errs, fmt.Errorf("stringTable does not match expected"))
	}

	if expected.DropFrames() != actual.DropFrames() {
		errs = multierr.Append(errs, fmt.Errorf("profileID does not match expected '%d', actual '%d'", expected.DropFrames(), actual.DropFrames()))
	}

	if expected.KeepFrames() != actual.KeepFrames() {
		errs = multierr.Append(errs, fmt.Errorf("keepFrames does not match expected '%d', actual '%d'", expected.KeepFrames(), actual.KeepFrames()))
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

	if expected.DefaultSampleType() != actual.DefaultSampleType() {
		errs = multierr.Append(errs, fmt.Errorf("defaultSampleType does not match expected '%d', actual '%d'", expected.DefaultSampleType(), actual.DefaultSampleType()))
	}

	if expected.PeriodType().Type() != actual.PeriodType().Type() ||
		expected.PeriodType().Unit() != actual.PeriodType().Unit() ||
		expected.PeriodType().AggregationTemporality() != actual.PeriodType().AggregationTemporality() {
		errs = multierr.Append(errs, fmt.Errorf("periodType does not match expected 'unit: %d, type: %d, aggregationTemporality: %d', actual 'unit: %d, type: %d,"+
			"aggregationTemporality: %d'", expected.PeriodType().Unit(), expected.PeriodType().Type(), expected.PeriodType().AggregationTemporality(),
			actual.PeriodType().Unit(), actual.PeriodType().Type(), actual.PeriodType().AggregationTemporality()))
	}

	errs = multierr.Append(errs, internal.AddErrPrefix("sampleType", CompareProfileValueTypeSlice(expected.SampleType(), actual.SampleType())))

	errs = multierr.Append(errs, internal.AddErrPrefix("sample", CompareProfileSampleSlice(expected.Sample(), actual.Sample())))

	errs = multierr.Append(errs, internal.AddErrPrefix("mapping", CompareProfileMappingSlice(expected.Mapping(), actual.Mapping())))

	errs = multierr.Append(errs, internal.AddErrPrefix("location", CompareProfileLocationSlice(expected.Location(), actual.Location())))

	errs = multierr.Append(errs, internal.AddErrPrefix("function", CompareProfileFunctionSlice(expected.Function(), actual.Function())))

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
			if elr.Type() == alr.Type() && elr.Unit() == alr.Unit() {
				foundMatch = true
				matchingValueTypes[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`valueTypes are out of order: valueType "unit: %d, type: %d, aggregationTemporality: %d" expected at index %d, found at index %d`,
							elr.Unit(), elr.Type(), elr.AggregationTemporality(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected valueType "unit: %d, type: %d, aggregationTemporality: %d`, elr.Unit(), elr.Type(), elr.AggregationTemporality()))
		}
	}

	for i := 0; i < numValueTypes; i++ {
		if _, ok := matchingValueTypes[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected valueType "unit: %d, type: %d, aggregationTemporality: %d`,
				actual.At(i).Unit(), actual.At(i).Type(), actual.At(i).AggregationTemporality()))
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
				`got "unit: %d, type: %d, aggregationTemporality: %d"`, elr.Unit(), elr.Type(), elr.AggregationTemporality(),
				alr.Unit(), alr.Type(), alr.AggregationTemporality()))
		}
	}

	return errs
}

func isValueTypeEqual(expected, actual pprofile.ValueType) bool {
	return expected.Type() == actual.Type() &&
		expected.Unit() == actual.Unit() &&
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
			if reflect.DeepEqual(elr.Attributes().AsRaw(), alr.Attributes().AsRaw()) {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`samples are out of order: sample "%v" expected at index %d, found at index %d`,
							elr.Attributes().AsRaw(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected sample: %v`, elr.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numSlice; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected sample: %v",
				actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingItems {
		errPrefix := fmt.Sprintf(`sample "%v"`, elr.Attributes().AsRaw())
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

	if expected.StacktraceIdIndex() != actual.StacktraceIdIndex() {
		errs = multierr.Append(errs, fmt.Errorf("expected stacktraceIdIndex '%d', got '%d'", expected.StacktraceIdIndex(), actual.StacktraceIdIndex()))
	}

	if expected.Link() != actual.Link() {
		errs = multierr.Append(errs, fmt.Errorf("expected link '%d', got '%d'", expected.Link(), actual.Link()))
	}

	if !reflect.DeepEqual(expected.LocationIndex().AsRaw(), actual.LocationIndex().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected locationIndex '%v', got '%v'", expected.LocationIndex().AsRaw(), actual.LocationIndex().AsRaw()))
	}

	if !reflect.DeepEqual(expected.Value().AsRaw(), actual.Value().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected value '%v', got '%v'", expected.Value().AsRaw(), actual.Value().AsRaw()))
	}

	if !reflect.DeepEqual(expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected timestampUnixNano '%v', got '%v'", expected.TimestampsUnixNano().AsRaw(), actual.TimestampsUnixNano().AsRaw()))
	}

	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected attributes '%v', got '%v'", expected.Attributes().AsRaw(), actual.Attributes().AsRaw()))
	}

	errPrefix := fmt.Sprintf(`labelSlice "%v"`, expected.Attributes().AsRaw())
	errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfileLabelSlice(expected.Label(), actual.Label())))

	return errs
}

func CompareProfileLabelSlice(expected, actual pprofile.LabelSlice) error {
	var errs error
	if expected.Len() != actual.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of labels doesn't match expected: %d, actual: %d",
			expected.Len(), actual.Len()))
		return errs
	}

	numLabels := expected.Len()

	matchingLabels := make(map[pprofile.Label]pprofile.Label, numLabels)

	var outOfOrderErrs error
	for e := 0; e < numLabels; e++ {
		elr := expected.At(e)
		var foundMatch bool
		for a := 0; a < numLabels; a++ {
			alr := actual.At(a)
			if _, ok := matchingLabels[alr]; ok {
				continue
			}
			if elr.Key() == alr.Key() {
				foundMatch = true
				matchingLabels[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`labels are out of order: label "key: %d" expected at index %d, found at index %d`,
							elr.Key(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected label "key: %d`, elr.Key()))
		}
	}

	for i := 0; i < numLabels; i++ {
		if _, ok := matchingLabels[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected label "key: %d`,
				actual.At(i).Key()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingLabels {
		if !isLabelEqual(elr, alr) {
			errs = multierr.Append(errs, fmt.Errorf(`label with "key: %d" does not match expected,`, alr.Key()))
		}
	}

	return errs
}

func isLabelEqual(expected, actual pprofile.Label) bool {
	return expected.Key() == actual.Key() &&
		expected.Str() == actual.Str() &&
		expected.Num() == actual.Num() &&
		expected.NumUnit() == actual.NumUnit()
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
			if reflect.DeepEqual(elr.Attributes().AsRaw(), alr.Attributes().AsRaw()) && elr.ID() == alr.ID() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`mappings are out of order: mapping "attributes: %v, id: %d" expected at index %d, found at index %d`,
							elr.Attributes().AsRaw(), elr.ID(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected mapping "attributes: %v, id: %d"`, elr.Attributes().AsRaw(), elr.ID()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile mapping "attributes: %v, id: %d"`,
				actual.At(i).Attributes().AsRaw(), actual.At(i).ID()))
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
			errs = multierr.Append(errs, fmt.Errorf(`mapping with "attributes: %v, id: %d", does not match expected`,
				alr.Attributes().AsRaw(), alr.ID()))
		}
	}

	return errs
}

func isMappingEqual(expected, actual pprofile.Mapping) bool {
	return expected.ID() == actual.ID() &&
		expected.MemoryStart() == actual.MemoryStart() &&
		expected.MemoryLimit() == actual.MemoryLimit() &&
		expected.FileOffset() == actual.FileOffset() &&
		expected.Filename() == actual.Filename() &&
		expected.BuildID() == actual.BuildID() &&
		expected.BuildIDKind() == actual.BuildIDKind() &&
		reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) &&
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
			if elr.Name() == alr.Name() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`functions are out of order: function "name: %d" expected at index %d, found at index %d`,
							elr.Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected function "name: %d"`, elr.Name()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile function "name: %d"`,
				actual.At(i).Name()))
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
			errs = multierr.Append(errs, fmt.Errorf(`function with "name: %d" does not match expected`, alr.Name()))
		}
	}

	return errs
}

func isFunctionEqual(expected, actual pprofile.Function) bool {
	return expected.ID() == actual.ID() &&
		expected.Name() == actual.Name() &&
		expected.SystemName() == actual.SystemName() &&
		expected.StartLine() == actual.StartLine() &&
		expected.Filename() == actual.Filename()
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
			if reflect.DeepEqual(elr.Attributes().AsRaw(), alr.Attributes().AsRaw()) && elr.ID() == alr.ID() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`location is out of order: location "attributes: %v, id: %d" expected at index %d, found at index %d`,
							elr.Attributes().AsRaw(), elr.ID(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected location "attributes: %v, id: %d"`, elr.Attributes().AsRaw(), elr.ID()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected location "attributes: %v, id: %d"`,
				actual.At(i).Attributes().AsRaw(), actual.At(i).ID()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for alr, elr := range matchingItems {
		errPrefix := fmt.Sprintf(`location "id: %d"`, elr.ID())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareProfileLocation(elr, alr)))
	}

	return errs
}

func CompareProfileLocation(expected, actual pprofile.Location) error {
	var errs error
	if expected.ID() != actual.ID() {
		return fmt.Errorf("expected id '%d', got '%d'", expected.ID(), actual.ID())
	}

	if expected.MappingIndex() != actual.MappingIndex() {
		return fmt.Errorf("expected mappingIndex '%d', got '%d'", expected.MappingIndex(), actual.MappingIndex())
	}

	if expected.Address() != actual.Address() {
		return fmt.Errorf("expected address '%d', got '%d'", expected.Address(), actual.Address())
	}

	if expected.IsFolded() != actual.IsFolded() {
		return fmt.Errorf("expected isFolded '%v', got '%v'", expected.IsFolded(), actual.IsFolded())
	}

	if expected.TypeIndex() != actual.TypeIndex() {
		return fmt.Errorf("expected typeIndex '%d', got '%d'", expected.TypeIndex(), actual.TypeIndex())
	}

	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		errs = multierr.Append(errs, fmt.Errorf("expected attributes '%v', got '%v'", expected.Attributes().AsRaw(), actual.Attributes().AsRaw()))
	}

	errPrefix := fmt.Sprintf(`line with location "id: %d"`, expected.ID())
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
			if elr.AttributeKey() == alr.AttributeKey() && elr.Unit() == alr.Unit() {
				foundMatch = true
				matchingItems[alr] = elr
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`attributeUnits are out of order: attributeUnit "attributeKey: %d" expected at index %d, found at index %d`,
							elr.AttributeKey(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected attributeUnit "attributeKey: %d"`, elr.AttributeKey()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile attributeUnit "attributeKey: %d"`,
				actual.At(i).AttributeKey()))
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
							elr.SpanID(), elr.TraceID(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf(`missing expected link "spanId: %s, traceId: %s"`, elr.SpanID(), elr.TraceID()))
		}
	}

	for i := 0; i < numItems; i++ {
		if _, ok := matchingItems[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf(`unexpected profile link "spanId: %s, traceId: %s"`,
				actual.At(i).SpanID(), actual.At(i).TraceID()))
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
