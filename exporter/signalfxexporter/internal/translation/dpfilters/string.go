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

package dpfilters // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"

import (
	"regexp"

	"github.com/gobwas/glob"
)

// StringFilter will match if any one of the given strings is a match.
type StringFilter struct {
	staticSet        map[string]bool
	regexps          []regexMatcher
	globs            []globMatcher
	anyStaticNegated bool
}

// NewStringFilter returns a filter that can match against the provided items.
func NewStringFilter(items []string) (*StringFilter, error) {
	staticSet := make(map[string]bool)
	var regexps []regexMatcher
	var globs []globMatcher

	anyStaticNegated := false
	for _, i := range items {
		m, negated := stripNegation(i)
		switch {
		case isRegex(m):
			var re *regexp.Regexp
			var err error

			reText := stripSlashes(m)
			re, err = regexp.Compile(reText)

			if err != nil {
				return nil, err
			}

			regexps = append(regexps, regexMatcher{re: re, negated: negated})
		case isGlobbed(m):
			g, err := glob.Compile(m)
			if err != nil {
				return nil, err
			}

			globs = append(globs, globMatcher{glob: g, negated: negated})
		default:
			staticSet[m] = negated
			if negated {
				anyStaticNegated = true
			}
		}
	}

	return &StringFilter{
		staticSet:        staticSet,
		regexps:          regexps,
		globs:            globs,
		anyStaticNegated: anyStaticNegated,
	}, nil
}

// Matches if s is positively matched by the filter items OR
// if it is positively matched by a non-glob/regex pattern exactly
// and is negated as well.  See the unit tests for examples.
func (f *StringFilter) Matches(s string) bool {
	if f == nil {
		return true
	}
	negated, matched := f.staticSet[s]
	// If a metric is negated and it matched it won't match anything else by
	// definition.
	if matched && negated {
		return false
	}

	for _, reMatch := range f.regexps {
		reMatched, negated := reMatch.Matches(s)
		if reMatched && negated {
			return false
		}
		matched = matched || reMatched
	}

	for _, globMatcher := range f.globs {
		globMatched, negated := globMatcher.Matches(s)
		if globMatched && negated {
			return false
		}
		matched = matched || globMatched
	}
	return matched
}

func (f *StringFilter) UnmarshalText(in []byte) error {
	sf, err := NewStringFilter([]string{string(in)})
	if err != nil {
		return err
	}
	*f = *sf
	return nil
}
