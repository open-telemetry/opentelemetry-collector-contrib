// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
)

type stringMatcher struct {
	standardItems       map[string]bool
	anyNegatedStandards bool
	regexItems          []regexItem
	globItems           []globbedItem
}

// This utility defines a regex as
// any string between two '/' characters
// with the option of a leading '!' to
// signify negation.
type regexItem struct {
	re        *regexp.Regexp
	isNegated bool
}

func isRegex(s string) bool {
	return len(s) > 2 && s[0] == '/' && s[len(s)-1] == '/'
}

type globbedItem struct {
	glob      glob.Glob
	isNegated bool
}

func isGlobbed(s string) bool {
	return strings.ContainsAny(s, "*?[]{}!")
}

func newStringMatcher(items []string) (*stringMatcher, error) {
	standards := make(map[string]bool)
	var regexes []regexItem
	var globs []globbedItem

	anyNegatedStandards := false
	for _, i := range items {
		item, isNegated := isNegatedItem(i)
		switch {
		case isRegex(item):
			var re *regexp.Regexp
			var err error

			// by definition this must lead and end with '/' chars
			reText := item[1 : len(item)-1]
			re, err = regexp.Compile(reText)

			if err != nil {
				return nil, fmt.Errorf("invalid regex item: %w", err)
			}

			regexes = append(regexes, regexItem{re: re, isNegated: isNegated})
		case isGlobbed(item):
			g, err := glob.Compile(item)
			if err != nil {
				return nil, fmt.Errorf("invalid glob item: %w", err)
			}

			globs = append(globs, globbedItem{glob: g, isNegated: isNegated})
		default:
			standards[item] = isNegated
			if isNegated {
				anyNegatedStandards = true
			}
		}
	}

	return &stringMatcher{
		standardItems:       standards,
		regexItems:          regexes,
		globItems:           globs,
		anyNegatedStandards: anyNegatedStandards,
	}, nil
}

// isNegatedItem strips a leading '!' and returns
// the remaining substring and true.  If no leading
// '!' is found, it returns the input string and false.
func isNegatedItem(value string) (string, bool) {
	if strings.HasPrefix(value, "!") {
		return value[1:], true
	}
	return value, false
}

func (f *stringMatcher) matches(s string) bool {
	negated, matched := f.standardItems[s]
	if matched {
		return !negated
	}

	// If negated standard item "!something" is provided
	// and "anything else" is evaluated we will always match.
	if f.anyNegatedStandards {
		return true
	}

	for _, reMatch := range f.regexItems {
		if reMatch.re.MatchString(s) != reMatch.isNegated {
			return true
		}
	}

	for _, globMatch := range f.globItems {
		if globMatch.glob.Match(s) != globMatch.isNegated {
			return true
		}
	}

	return false
}
