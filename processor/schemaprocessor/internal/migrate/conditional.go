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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/multierr"
)

// ValueMatch defines the expected match type
// that is used on creation of `ConditionalAttributeSet`
type ValueMatch interface {
	~string
}

type ConditionalAttributeSet struct {
	on    *map[string]struct{}
	attrs *AttributeChangeSet
}

type ConditionalAttributeSetSlice []*ConditionalAttributeSet

func NewConditionalAttributeSet[Key AttributeKey, Value AttributeKey, Match ValueMatch](mappings map[Key]Value, matches ...Match) *ConditionalAttributeSet {
	on := make(map[string]struct{})
	for _, m := range matches {
		on[string(m)] = struct{}{}
	}
	return &ConditionalAttributeSet{
		on:    &on,
		attrs: NewAttributes(mappings),
	}
}

func (ca *ConditionalAttributeSet) Apply(attrs pcommon.Map, values ...string) (errs error) {
	if ca.check(values...) {
		errs = ca.attrs.Apply(attrs)
	}
	return errs
}

func (ca *ConditionalAttributeSet) Rollback(attrs pcommon.Map, values ...string) (errs error) {
	if ca.check(values...) {
		errs = ca.attrs.Rollback(attrs)
	}
	return errs
}

func (ca *ConditionalAttributeSet) check(values ...string) bool {
	if len(*ca.on) == 0 {
		return true
	}
	for _, v := range values {
		if _, ok := (*ca.on)[v]; !ok {
			return false
		}
	}
	return true
}

func NewConditionalAttributeSetSlice(conditions ...*ConditionalAttributeSet) *ConditionalAttributeSetSlice {
	values := new(ConditionalAttributeSetSlice)
	for _, c := range conditions {
		(*values) = append((*values), c)
	}
	return values
}

func (slice *ConditionalAttributeSetSlice) Apply(attrs pcommon.Map, values ...string) error {
	return slice.do(StateSelectorApply, attrs, values)
}

func (slice *ConditionalAttributeSetSlice) Rollback(attrs pcommon.Map, values ...string) error {
	return slice.do(StateSelectorRollback, attrs, values)
}

func (slice *ConditionalAttributeSetSlice) do(ss StateSelector, attrs pcommon.Map, values []string) (errs error) {
	for i := 0; i < len((*slice)); i++ {
		switch ss {
		case StateSelectorApply:
			errs = multierr.Append(errs, (*slice)[i].Apply(attrs, values...))
		case StateSelectorRollback:
			errs = multierr.Append(errs, (*slice)[len((*slice))-i-1].Rollback(attrs, values...))
		}
	}
	return errs
}
