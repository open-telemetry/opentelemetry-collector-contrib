// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"regexp/syntax"
	"strings"
	"unicode/utf8"
)

const maxGrokPrefilterLiterals = 4

func newGrokLiteralPrefilter(pattern string, patternDefinitions ...string) func(string) bool {
	defs := grokPrefilterPatternDefinitions(patternDefinitions)
	expanded := expandGrokPrefilterPattern(pattern, defs, 0)
	literals := selectGrokPrefilterLiterals(extractRequiredGrokLiterals(expanded))
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

func grokPrefilterPatternDefinitions(patternDefinitions []string) map[string]string {
	if len(patternDefinitions) == 0 {
		return nil
	}
	defs := make(map[string]string, len(patternDefinitions))
	for _, patternDefinition := range patternDefinitions {
		name, pattern, ok := strings.Cut(patternDefinition, "=")
		if !ok || name == "" {
			continue
		}
		defs[name] = pattern
	}
	return defs
}

func expandGrokPrefilterPattern(pattern string, defs map[string]string, depth int) string {
	if depth > 8 {
		return pattern
	}

	var expanded strings.Builder
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if ch != '%' || i+1 >= len(pattern) || pattern[i+1] != '{' {
			expanded.WriteByte(ch)
			continue
		}

		start := i + 2
		i = start
		for i < len(pattern) && pattern[i] != '}' {
			i++
		}
		if i >= len(pattern) {
			expanded.WriteByte(ch)
			continue
		}

		name := grokPrefilterPatternName(pattern[start:i])
		if definition, ok := defs[name]; ok {
			expanded.WriteString(expandGrokPrefilterPattern(definition, defs, depth+1))
			continue
		}

		// Preserve a delimiter so literals on both sides of an unknown grok
		// placeholder are not merged into a single required literal.
		expanded.WriteByte('.')
	}
	return expanded.String()
}

func grokPrefilterPatternName(expr string) string {
	if idx := strings.IndexByte(expr, ':'); idx >= 0 {
		return expr[:idx]
	}
	return expr
}

func extractRequiredGrokLiterals(pattern string) []string {
	if literals, ok := extractRequiredGrokRegexLiterals(pattern); ok {
		return literals
	}
	if hasGrokInlineCaseInsensitiveFlag(pattern) {
		return nil
	}

	var literals []string
	var current strings.Builder
	parenDepth := 0
	inClass := false
	topLevelAlternation := false

	flush := func() {
		literal := strings.TrimSpace(current.String())
		current.Reset()
		if len(literal) >= 3 {
			literals = append(literals, literal)
		}
	}
	flushQuantifiedOptionalLiteral := func() {
		literal := current.String()
		if literal != "" {
			_, size := utf8.DecodeLastRuneInString(literal)
			if size > 0 {
				current.Reset()
				current.WriteString(literal[:len(literal)-size])
			}
		}
		flush()
	}

	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if ch == '%' && i+1 < len(pattern) && pattern[i+1] == '{' {
			flush()
			i += 2
			for i < len(pattern) && pattern[i] != '}' {
				i++
			}
			continue
		}

		if ch == '\\' {
			if i+1 >= len(pattern) {
				flush()
				continue
			}
			next := pattern[i+1]
			i++
			if parenDepth == 0 && !inClass && isEscapedGrokLiteral(next) {
				current.WriteByte(next)
				continue
			}
			flush()
			continue
		}

		if inClass {
			if ch == ']' {
				inClass = false
			}
			continue
		}

		if parenDepth > 0 {
			switch ch {
			case '[':
				inClass = true
			case '(':
				parenDepth++
			case ')':
				parenDepth--
			}
			continue
		}

		switch ch {
		case '(':
			flush()
			if strings.HasPrefix(pattern[i:], "(?:") {
				groupEnd := findClosingGrokGroup(pattern, i)
				if groupEnd > i && isRequiredGrokGroup(pattern, groupEnd) {
					literals = append(literals, extractRequiredGrokLiterals(pattern[i+3:groupEnd])...)
				}
				if groupEnd > i {
					i = groupEnd
					continue
				}
			}
			parenDepth++
		case '[':
			flush()
			inClass = true
		case '|':
			flush()
			topLevelAlternation = true
		case '*', '?':
			flushQuantifiedOptionalLiteral()
		case '+':
			flush()
		case '{':
			quantifierEnd := i + 1
			for quantifierEnd < len(pattern) && pattern[quantifierEnd] != '}' {
				quantifierEnd++
			}
			if quantifierEnd < len(pattern) {
				if isZeroMinimumGrokBraceQuantifier(pattern[i+1 : quantifierEnd]) {
					flushQuantifiedOptionalLiteral()
				} else {
					flush()
				}
				i = quantifierEnd
				continue
			}
			flush()
		case '.', '^', '$', '}':
			flush()
		default:
			if ch < 0x20 {
				flush()
				continue
			}
			if ch < 0x7f {
				current.WriteByte(ch)
				continue
			}
			r, size := utf8.DecodeRuneInString(pattern[i:])
			if r == utf8.RuneError && size == 1 {
				flush()
				continue
			}
			current.WriteString(pattern[i : i+size])
			i += size - 1
		}
	}
	flush()

	if topLevelAlternation {
		return nil
	}
	return literals
}

func hasGrokInlineCaseInsensitiveFlag(pattern string) bool {
	for i := 0; i+2 < len(pattern); i++ {
		if pattern[i] != '(' || pattern[i+1] != '?' {
			continue
		}

		negated := false
		for j := i + 2; j < len(pattern); j++ {
			ch := pattern[j]
			switch {
			case ch >= 'a' && ch <= 'z':
				if ch == 'i' && !negated {
					return true
				}
			case ch == '-':
				negated = true
			case ch == ')' || ch == ':':
				i = j
				goto next
			default:
				goto next
			}
		}
	next:
	}
	return false
}

func isZeroMinimumGrokBraceQuantifier(quantifier string) bool {
	minimum, _, _ := strings.Cut(quantifier, ",")
	return strings.TrimSpace(minimum) == "0"
}

func findClosingGrokGroup(pattern string, open int) int {
	depth := 0
	inClass := false
	for i := open; i < len(pattern); i++ {
		ch := pattern[i]
		if ch == '\\' {
			i++
			continue
		}
		if inClass {
			if ch == ']' {
				inClass = false
			}
			continue
		}
		switch ch {
		case '[':
			inClass = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isRequiredGrokGroup(pattern string, groupEnd int) bool {
	next := groupEnd + 1
	if next >= len(pattern) {
		return true
	}
	switch pattern[next] {
	case '?', '*':
		return false
	case '{':
		return next+1 >= len(pattern) || pattern[next+1] != '0'
	default:
		return true
	}
}

func extractRequiredGrokRegexLiterals(pattern string) ([]string, bool) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, false
	}
	re = re.Simplify()
	if hasRegexFoldCase(re) || re.Op == syntax.OpAlternate {
		return nil, true
	}
	return requiredRegexLiterals(re), true
}

func isEscapedGrokLiteral(ch byte) bool {
	switch ch {
	case '\\', '"', '\'', '[', ']', '(', ')', '{', '}', '.', '*', '+', '?', '^', '$', '|', ':', '-', ',', '/', ' ':
		return true
	default:
		return false
	}
}

func selectGrokPrefilterLiterals(literals []string) []string {
	return selectPrefilterLiterals(literals, maxGrokPrefilterLiterals)
}
