// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
	"go.uber.org/multierr"
)

// AttributeChangeSet represents a rename_attributes type operation.
// The listed changes are duplicated twice
// to allow for simplified means of transition to or from a revision.
type AttributeChangeSet struct {
	// The keys are the old attribute name used in the previous version, the values are the
	// new attribute name starting from this version (comment from ast.AttributeMap)
	updates ast.AttributeMap
	// the inverse of the updates map
	rollback ast.AttributeMap
}

// NewAttributeChangeSet allows for typed strings to be used as part
// of the invocation that will be converted into the default string type.
func NewAttributeChangeSet(mappings ast.AttributeMap) AttributeChangeSet {
	// for ambiguous rollbacks (if updates contains entries with multiple keys that have the same value), rollback contains the last key iterated over in mappings
	attr := AttributeChangeSet{
		updates:  make(map[string]string, len(mappings)),
		rollback: make(map[string]string, len(mappings)),
	}
	for k, v := range mappings {
		attr.updates[k] = v
		attr.rollback[v] = k
	}
	return attr
}

func (a AttributeChangeSet) IsMigrator() {}

func (a *AttributeChangeSet) Do(ss StateSelector, attrs pcommon.Map) (errs error) {
	var (
		updated = make(map[string]struct{})
		results = pcommon.NewMap()
	)
	attrs.Range(func(k string, v pcommon.Value) bool {
		var (
			key     string
			matched bool
		)
		switch ss {
		case StateSelectorApply:
			key, matched = a.updates[k]
		case StateSelectorRollback:
			key, matched = a.rollback[k]
		}
		if matched {
			k, updated[key] = key, struct{}{}
		} else {
			// TODO: Since the spec hasn't decided the behavior on what
			//       should happen on a name conflict, this will assume
			//       the rewrite has priority and will set it to the original
			//       entry's value, not the existing value.
			if _, overridden := updated[k]; overridden {
				errs = multierr.Append(errs, fmt.Errorf("value %q already exists", k))
				return true
			}
		}
		v.CopyTo(results.PutEmpty(k))
		return true
	})
	results.CopyTo(attrs)
	return errs
}
