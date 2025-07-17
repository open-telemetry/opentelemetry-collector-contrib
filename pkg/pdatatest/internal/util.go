// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/multierr"
)

func MaskResourceAttributeValue(res pcommon.Resource, attr string) {
	if _, ok := res.Attributes().Get(attr); ok {
		res.Attributes().Remove(attr)
	}
}

func ChangeResourceAttributeValue(res pcommon.Resource, attr string, changeFn func(string) string) {
	if _, ok := res.Attributes().Get(attr); ok {
		if v, ok := res.Attributes().Get(attr); ok {
			res.Attributes().PutStr(attr, changeFn(v.Str()))
		}
	}
}

func MatchResourceAttributeValue(res pcommon.Resource, attr string, re *regexp.Regexp) {
	if _, ok := res.Attributes().Get(attr); ok {
		if v, ok := res.Attributes().Get(attr); ok {
			results := re.FindStringSubmatch(v.Str())
			if len(results) > 0 {
				res.Attributes().PutStr(attr, results[0])
			}
		}
	}
}

// AddErrPrefix adds a prefix to every multierr error.
func AddErrPrefix(prefix string, in error) error {
	var out error
	for _, err := range multierr.Errors(in) {
		out = multierr.Append(out, fmt.Errorf("%s: %w", prefix, err))
	}
	return out
}

func CompareResource(expected, actual pcommon.Resource) error {
	return multierr.Combine(
		CompareAttributes(expected.Attributes(), actual.Attributes()),
		CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()),
	)
}

func CompareInstrumentationScope(expected, actual pcommon.InstrumentationScope) error {
	var errs error
	if expected.Name() != actual.Name() {
		errs = multierr.Append(errs, fmt.Errorf("name doesn't match expected: %s, actual: %s",
			expected.Name(), actual.Name()))
	}
	if expected.Version() != actual.Version() {
		errs = multierr.Append(errs, fmt.Errorf("version doesn't match expected: %s, actual: %s",
			expected.Version(), actual.Version()))
	}
	return multierr.Combine(errs,
		CompareAttributes(expected.Attributes(), actual.Attributes()),
		CompareDroppedAttributesCount(expected.DroppedAttributesCount(), actual.DroppedAttributesCount()))
}

func CompareSchemaURL(expected, actual string) error {
	if expected != actual {
		return fmt.Errorf("schema url doesn't match expected: %s, actual: %s", expected, actual)
	}
	return nil
}

func CompareAttributes(expected, actual pcommon.Map) error {
	if !reflect.DeepEqual(expected.AsRaw(), actual.AsRaw()) {
		return fmt.Errorf("attributes don't match expected: %v, actual: %v", expected.AsRaw(), actual.AsRaw())
	}
	return nil
}

func CompareDroppedAttributesCount(expected, actual uint32) error {
	if expected != actual {
		return fmt.Errorf("dropped attributes count doesn't match expected: %d, actual: %d", expected, actual)
	}
	return nil
}

func OrderMapByKey(input map[string]any) map[string]any {
	// Create a slice to hold the keys
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}

	// Sort the keys
	sort.Strings(keys)

	// Create a new map to hold the sorted key-value pairs
	orderedMap := make(map[string]any, len(input))
	for _, k := range keys {
		orderedMap[k] = input[k]
	}

	return orderedMap
}
