// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

// AttributeKey is used to allow strings and type defined
// strings to be used within the creation of creating an `AttributeChangeSet`
type AttributeKey interface {
	~string
}

// AttributeChangeSet represents an unscoped entry that can be applied.
//
// The listed changes are duplicated twice
// to allow for simplified means of transition to or from a revision.
type AttributeChangeSet struct {
	updates  map[alias.AttributeKey]alias.AttributeKey
	rollback map[alias.AttributeKey]alias.AttributeKey
}

// AttributeChangeSetSlice allows for `AttributeChangeSet`
// to be chained together as they are defined within the schema
// and be applied sequentially to ensure deterministic behavior.
type AttributeChangeSetSlice []*AttributeChangeSet

// NewAttributes allows for typed strings to be used as part
// of the invocation that will be converted into the default string type.
func NewAttributes[Key AttributeKey, Value AttributeKey](mappings map[Key]Value) *AttributeChangeSet {
	attr := &AttributeChangeSet{
		updates:  make(map[string]string, len(mappings)),
		rollback: make(map[string]string, len(mappings)),
	}
	for k, v := range mappings {
		attr.updates[string(k)] = string(v)
		attr.rollback[string(v)] = string(k)
	}
	return attr
}

func (a *AttributeChangeSet) Apply(attrs pcommon.Map) error {
	return a.do(StateSelectorApply, attrs)
}

func (a *AttributeChangeSet) Rollback(attrs pcommon.Map) error {
	return a.do(StateSelectorRollback, attrs)
}

func (a *AttributeChangeSet) do(ss StateSelctor, attrs pcommon.Map) (errs error) {
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

// NewAttributeChangeSetSlice combines all the provided `AttributeChangeSets`
// and allows them to be executed in the provided order.
func NewAttributeChangeSetSlice(changes ...*AttributeChangeSet) *AttributeChangeSetSlice {
	values := new(AttributeChangeSetSlice)
	for _, c := range changes {
		(*values) = append((*values), c)
	}
	return values
}

func (slice *AttributeChangeSetSlice) Apply(attrs pcommon.Map) error {
	return slice.do(StateSelectorApply, attrs)
}

func (slice *AttributeChangeSetSlice) Rollback(attrs pcommon.Map) error {
	return slice.do(StateSelectorRollback, attrs)
}

func (slice *AttributeChangeSetSlice) do(ss StateSelctor, attrs pcommon.Map) (errs error) {
	for i := 0; i < len(*slice); i++ {
		switch ss {
		case StateSelectorApply:
			errs = multierr.Append(errs, (*slice)[i].Apply(attrs))
		case StateSelectorRollback:
			errs = multierr.Append(errs, (*slice)[len(*slice)-1-i].Rollback(attrs))
		}
	}
	return errs
}
