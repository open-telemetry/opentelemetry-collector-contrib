// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type set = map[string]struct{}

// MultiConditionalAttributeSet maps from string keys to possible values for each of those keys.  The Do function then checks passed in values for each key against the list provided here in the constructor.  If there is a matching value for each key, the attribute changes are applied.
type MultiConditionalAttributeSet struct {
	// map from string keys (in the intended case "event.name" and "span.name") to a set of acceptable values.
	keysToPossibleValues map[string]set
	attrs                AttributeChangeSet
}

type MultiConditionalAttributeSetSlice []*MultiConditionalAttributeSet

func NewMultiConditionalAttributeSet[Match ValueMatch](mappings map[string]string, matches map[string][]Match) MultiConditionalAttributeSet {
	keysToPossibleValues := make(map[string]set)
	for k, values := range matches {
		on := make(map[string]struct{})
		for _, val := range values {
			on[string(val)] = struct{}{}
		}
		keysToPossibleValues[k] = on
	}
	return MultiConditionalAttributeSet{
		keysToPossibleValues: keysToPossibleValues,
		attrs:                NewAttributeChangeSet(mappings),
	}
}

func (ca MultiConditionalAttributeSet) IsMigrator() {}

// Do function applies the attribute changes if the passed in values match the expected values provided in the constructor.  Uses the Do method of the embedded AttributeChangeSet
func (ca *MultiConditionalAttributeSet) Do(ss StateSelector, attrs pcommon.Map, keyToCheckVals map[string]string) (errs error) {
	match, err := ca.check(keyToCheckVals)
	if err != nil {
		return err
	}
	if match {
		errs = ca.attrs.Do(ss, attrs)
	}
	return errs
}

func (ca *MultiConditionalAttributeSet) check(keyToCheckVals map[string]string) (bool, error) {
	if len(ca.keysToPossibleValues) == 0 {
		return true, nil
	}
	if len(ca.keysToPossibleValues) != len(keyToCheckVals) {
		return false, errors.New("passed in wrong number of matchers to MultiConditionalAttributeSet")
	}
	for k, inVal := range keyToCheckVals {
		// We must already have a key matching the input key!  If not, return an error
		// indicates a programming error, should be impossible if using the class correctly
		valToMatch, ok := (ca.keysToPossibleValues)[k]
		if !ok {
			return false, errors.New("passed in a key that doesn't exist in MultiConditionalAttributeSet")
		}
		// if there's nothing in here, match all values
		if len(valToMatch) == 0 {
			continue
		}
		if _, ok := valToMatch[inVal]; !ok {
			return false, nil
		}
	}
	// if we've gone through every one of the keys, and they've all generated matches, return true
	return true, nil
}
