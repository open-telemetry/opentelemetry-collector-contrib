// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
)

// ValueMatch defines the expected match type
// that is used on creation of `ConditionalAttributeSet`
type ValueMatch interface {
	~string
}

// ConditionalAttributeSet represents a rename_attribute that will happen only if the passed in value matches the `on` set.
type ConditionalAttributeSet struct {
	on    map[string]struct{}
	attrs AttributeChangeSet
}

type ConditionalAttributeSetSlice []*ConditionalAttributeSet

func NewConditionalAttributeSet[Match ValueMatch](mappings ast.AttributeMap, matches ...Match) ConditionalAttributeSet {
	on := make(map[string]struct{})
	for _, m := range matches {
		on[string(m)] = struct{}{}
	}
	return ConditionalAttributeSet{
		on:    on,
		attrs: NewAttributeChangeSet(mappings),
	}
}

func (ca ConditionalAttributeSet) IsMigrator() {}

// Do applies the attribute changes specified in the constructor if any of the values in values matches the matches specified in the constructor.
func (ca *ConditionalAttributeSet) Do(ss StateSelector, attrs pcommon.Map, values ...string) (errs error) {
	if ca.check(values...) {
		errs = ca.attrs.Do(ss, attrs)
	}
	return errs
}

func (ca *ConditionalAttributeSet) check(values ...string) bool {
	if len(ca.on) == 0 {
		return true
	}
	for _, v := range values {
		if _, ok := (ca.on)[v]; !ok {
			return false
		}
	}
	return true
}
