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

package datadogexporter

import (
	"regexp"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
)

// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L15-L19
// Blocklister holds a list of regular expressions which will match resources
// on spans that should be dropped.
type Blocklister struct {
	list []*regexp.Regexp
}

// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L21-L29
// Allows returns true if the Blocklister permits this span.
func (f *Blocklister) Allows(span *pb.Span) bool {
	for _, entry := range f.list {
		if entry.MatchString(span.Resource) {
			return false
		}
	}
	return true
}

// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L41-L45
// NewBlocklister creates a new Blocklister based on the given list of
// regular expressions.
func NewBlocklister(exprs []string) *Blocklister {
	return &Blocklister{list: compileRules(exprs)}
}

// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L47-L59
// compileRules compiles as many rules as possible from the list of expressions.
func compileRules(exprs []string) []*regexp.Regexp {
	list := make([]*regexp.Regexp, 0, len(exprs))
	for _, entry := range exprs {
		rule, err := regexp.Compile(entry)
		if err != nil {
			// log.Errorf("Invalid resource filter: %q", entry)
			continue
		}
		list = append(list, rule)
	}
	return list
}
