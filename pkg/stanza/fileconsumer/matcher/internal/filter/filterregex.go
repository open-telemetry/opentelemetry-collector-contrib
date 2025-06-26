// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/filter"

import (
	"regexp"
)

// filterRegex is the structure that holds the regex filters
type filterRegex struct {
	IncludeRegex              []*regexp.Regexp
	ExcludeRegex              []*regexp.Regexp
	CapturePathSubstringRegex *regexp.Regexp
}

// apply filters the items based on the Include and Exclude regex rules
func (fr *filterRegex) apply(items []*item) ([]*item, error) {
	var filteredItems []*item

	// If the capture path regex is nil, return empty result
	if fr.CapturePathSubstringRegex == nil {
		return filteredItems, nil
	}

	// Loop over items and filter based on regex patterns
	for _, item := range items {
		match := fr.CapturePathSubstringRegex.FindStringSubmatch(item.value)
		if len(match) <= 1 {
			continue
		}

		capturedPathSubstring := match[1]

		// Applying both IncludeRegex and ExcludeRegex in one condition
		if (len(fr.IncludeRegex) == 0 || matchRegexList(capturedPathSubstring, fr.IncludeRegex)) &&
			(len(fr.ExcludeRegex) == 0 || !matchRegexList(capturedPathSubstring, fr.ExcludeRegex)) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems, nil
}

// FilterRegex creates a filterRegex with precompiled regex patterns
func FilterRegex(capturePathSubstringRegex string, includeRegex, excludeRegex []string) Option {
	// Precompile regex patterns for include and exclude lists
	includeRegexCompiled := compileRegexList(includeRegex)
	excludeRegexCompiled := compileRegexList(excludeRegex)
	capturePathSubstringCompiled := regexp.MustCompile(capturePathSubstringRegex)

	return &filterRegex{
		IncludeRegex:              includeRegexCompiled,
		ExcludeRegex:              excludeRegexCompiled,
		CapturePathSubstringRegex: capturePathSubstringCompiled,
	}
}

// compileRegexList compiles a list of regex patterns into a slice of *regexp.Regexp
func compileRegexList(regexList []string) []*regexp.Regexp {
	var compiled []*regexp.Regexp
	for _, pattern := range regexList {
		if pattern != "" {
			compiled = append(compiled, regexp.MustCompile(pattern))
		}
	}
	return compiled
}

// matchRegexList checks if any pattern in the provided regex list matches the value
func matchRegexList(value string, regexList []*regexp.Regexp) bool {
	for _, re := range regexList {
		if re.MatchString(value) {
			return true
		}
	}
	return false
}
