// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"regexp/syntax"
	"slices"
	sortpkg "sort"
	"strings"
)

const maxRegexPrefilterLiterals = 4

func newRegexLiteralPrefilter(pattern string) func(string) bool {
	literals := selectRegexPrefilterLiterals(extractRequiredRegexLiterals(pattern))
	if len(literals) == 0 {
		return nil
	}
	return func(s string) bool {
		for _, literal := range literals {
			if !strings.Contains(s, literal) {
				return false
			}
		}
		return true
	}
}

func extractRequiredRegexLiterals(pattern string) []string {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	re = re.Simplify()
	if hasRegexFoldCase(re) {
		return nil
	}
	if re.Op == syntax.OpAlternate {
		return nil
	}
	return requiredRegexLiterals(re)
}

func requiredRegexLiterals(re *syntax.Regexp) []string {
	switch re.Op {
	case syntax.OpLiteral:
		return []string{string(re.Rune)}
	case syntax.OpCapture, syntax.OpPlus:
		return requiredRegexLiterals(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min == 0 {
			return nil
		}
		return requiredRegexLiterals(re.Sub[0])
	case syntax.OpConcat:
		var literals []string
		for _, sub := range re.Sub {
			literals = append(literals, requiredRegexLiterals(sub)...)
		}
		return literals
	default:
		return nil
	}
}

func hasRegexFoldCase(re *syntax.Regexp) bool {
	if re.Flags&syntax.FoldCase != 0 {
		return true
	}
	return slices.ContainsFunc(re.Sub, hasRegexFoldCase)
}

func selectRegexPrefilterLiterals(literals []string) []string {
	return selectPrefilterLiterals(literals, maxRegexPrefilterLiterals)
}

func selectPrefilterLiterals(literals []string, maxLiterals int) []string {
	seen := make(map[string]struct{}, len(literals))
	selected := make([]string, 0, len(literals))
	for _, literal := range literals {
		literal = strings.TrimSpace(literal)
		if len(literal) < 3 {
			continue
		}
		if _, ok := seen[literal]; ok {
			continue
		}
		seen[literal] = struct{}{}
		selected = append(selected, literal)
	}
	sortpkg.Slice(selected, func(i, j int) bool {
		if len(selected[i]) == len(selected[j]) {
			return selected[i] < selected[j]
		}
		return len(selected[i]) > len(selected[j])
	})
	if len(selected) > maxLiterals {
		selected = selected[:maxLiterals]
	}
	return selected
}
