// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"

import (
	"fmt"
	"reflect"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/multierr"
)

// CompareProfiles compares each part of two given Profiles and returns
// an error if they don't match. The error describes what didn't match.
func CompareProfiles(expected, actual pprofile.Profiles, options ...CompareProfilesOption) error {
	exp, act := pprofile.NewProfiles(), pprofile.NewProfiles()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	// r := exp.ResourceProfiles()
	// exp.IsReadOnly()
	// exp.MarkReadOnly()
	// exp.SampleCount()

	// r1 := r.At(0)
	// r1.Resource().Attributes()
	// r1.Resource().DroppedAttributesCount()
	// s := r1.ScopeProfiles().At(0)

	// s.SchemaUrl()
	// s.Scope()

	// p := s.Profiles().At(0)

	// p.Attributes()
	// p.DroppedAttributesCount()
	// p.EndTime()
	// pp := p.Profile()
	// p.ProfileID()
	// p.StartTime()

	// pp.AttributeTable()
	// pp.AttributeUnits()
	// pp.Comment()
	// pp.DefaultSampleType()
	// pp.DropFrames()
	// pp.Duration()
	// pp.Function()
	// pp.KeepFrames()
	// pp.LinkTable()
	// pp.Location()
	// pp.LocationIndices()

	for _, option := range options {
		option.applyOnProfiles(exp, act)
	}

	if exp.IsReadOnly() != act.IsReadOnly() {
		return fmt.Errorf("readOnly state differs from expected '%s'", exp.IsReadOnly())
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
		errs = multierr.Append(errs, fmt.Errorf("profileID does not match expected '%d', actual '%d'", expected.KeepFrames(), actual.KeepFrames()))
	}

	if expected.StartTime() != actual.StartTime() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d", expected.StartTime(), actual.StartTime()))
	}

	if expected.Duration() != actual.Duration() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d", expected.Duration(), actual.Duration()))
	}

	if expected.Period() != actual.Period() {
		errs = multierr.Append(errs, fmt.Errorf("profileID does not match expected '%d', actual '%d'", expected.Period(), actual.Period()))
	}

	if expected.DefaultSampleType() != actual.DefaultSampleType() {
		errs = multierr.Append(errs, fmt.Errorf("profileID does not match expected '%d', actual '%d'", expected.DefaultSampleType(), actual.DefaultSampleType()))
	}

	// sampletype
	expected.SampleType()
	// sample
	expected.Sample()
	// mapping
	expected.Mapping()
	//location
	expected.Location()
	//function
	expected.Function()
	//attributeunits
	expected.AttributeUnits()
	//linktable
	expected.LinkTable()
	//periodtype
	expected.PeriodType()

	return errs
}
