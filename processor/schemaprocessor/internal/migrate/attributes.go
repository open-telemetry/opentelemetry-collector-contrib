// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/multierr"
)

// AttributeChangeSet represents a rename_attributes type operation.
// The listed changes are duplicated twice
// to allow for simplified means of transition to or from a revision.
type AttributeChangeSet struct {
	// The keys are the old attribute name used in the previous version, the values are the
	// new attribute name starting from this version (comment from ast.AttributeMap)
	updates map[string]string
	// the inverse of the updates map
	rollback       map[string]string
	copyAttributes bool
}

// NewAttributeChangeSet allows for typed strings to be used as part
// of the invocation that will be converted into the default string type.
func NewAttributeChangeSet(mappings map[string]string, copyAttributes bool) AttributeChangeSet {
	// for ambiguous rollbacks (if updates contains entries with multiple keys that have the same value), rollback contains the last key iterated over in mappings
	attr := AttributeChangeSet{
		updates:        make(map[string]string, len(mappings)),
		rollback:       make(map[string]string, len(mappings)),
		copyAttributes: copyAttributes,
	}
	for k, v := range mappings {
		attr.updates[k] = v
		attr.rollback[v] = k
	}
	return attr
}

func (AttributeChangeSet) IsMigrator() {}

func (a *AttributeChangeSet) Do(ss StateSelector, attrs pcommon.Map) (errs error) {
	var (
		updated = make(map[string]struct{})
		results = pcommon.NewMap()
	)
	for k, v := range attrs.All() {
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
			updated[key] = struct{}{}
			if a.copyAttributes {
				// In copy mode, preserve the original attribute and also
				// write the renamed copy (unless the target already exists).
				if _, targetExists := attrs.Get(key); !targetExists {
					v.CopyTo(results.PutEmpty(key))
				}
				// Original key k is preserved — falls through to CopyTo below.
			} else {
				k = key
			}
		} else if _, overridden := updated[k]; overridden && !a.copyAttributes {
			// The schema file format spec does not define behavior for name
			// conflicts (when the target attribute already exists). We treat
			// identical values as a no-op and only report an error when they differ.
			if renamed, ok := results.Get(k); ok && !v.Equal(renamed) {
				errs = multierr.Append(errs, fmt.Errorf("value %q already exists", k))
			}
			continue
		}
		v.CopyTo(results.PutEmpty(k))
	}
	results.CopyTo(attrs)
	return errs
}
