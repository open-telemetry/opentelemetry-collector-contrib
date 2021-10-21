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

// denylister holds a list of regular expressions which will match resources
// on spans that should be dropped.
// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L15-L19
type denylister struct {
	list []*regexp.Regexp
}

// allows returns true if the Denylister permits this span.
// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L21-L29
func (f *denylister) allows(span *pb.Span) bool {
	for _, entry := range f.list {
		if entry.MatchString(span.Resource) {
			return false
		}
	}
	return true
}

// newDenylister creates a new Denylister based on the given list of
// regular expressions.
// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L41-L45
func newDenylister(exprs []string) *denylister {
	return &denylister{list: compileRules(exprs)}
}

// compileRules compiles as many rules as possible from the list of expressions.
// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/filters/blacklister.go#L47-L59
func compileRules(exprs []string) []*regexp.Regexp {
	list := make([]*regexp.Regexp, 0, len(exprs))
	for _, entry := range exprs {
		rule := regexp.MustCompile(entry)
		list = append(list, rule)
	}
	return list
}
