// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"fmt"
	"strings"
)

// SelectorScope names where a fingerprint selector reads attribute values
// from. Fingerprints are trace-level, so the scopes include trace concepts
// (root, any) that item-level OTTL paths cannot express; the resource, scope,
// and span prefixes match OTTL's span-context path names so conditions and
// fingerprint entries share one spelling.
type SelectorScope int

const (
	// ScopeResource reads from each ResourceSpans' resource attributes.
	ScopeResource SelectorScope = iota
	// ScopeScope reads from each instrumentation scope's attributes.
	ScopeScope
	// ScopeSpan reads from every span's attributes.
	ScopeSpan
	// ScopeRoot reads from the spans that satisfy the processor's root-span
	// condition.
	ScopeRoot
	// ScopeAny reads from resource, instrumentation scope, and span
	// attributes.
	ScopeAny
)

// Selector is a parsed fingerprint entry: a scope and an attribute key.
type Selector struct {
	Scope SelectorScope
	Key   string
}

var scopeNames = map[string]SelectorScope{
	"resource": ScopeResource,
	"scope":    ScopeScope,
	"span":     ScopeSpan,
	"root":     ScopeRoot,
	"any":      ScopeAny,
}

// ParseSelector parses a fingerprint entry of the form
// `<scope>.attributes["<key>"]`, where scope is one of resource, scope, span,
// root, or any.
func ParseSelector(s string) (Selector, error) {
	scopeName, rest, found := strings.Cut(s, ".")
	scope, known := scopeNames[scopeName]
	if !found || !known {
		return Selector{}, fmt.Errorf("%q is not a scoped attribute selector; use (resource|scope|span|root|any).attributes[\"<name>\"], e.g. any.attributes[%q]", s, s)
	}
	const prefix, suffix = `attributes["`, `"]`
	if !strings.HasPrefix(rest, prefix) || !strings.HasSuffix(rest, suffix) {
		return Selector{}, fmt.Errorf("%q must have the form %s.attributes[\"<name>\"]", s, scopeName)
	}
	key := rest[len(prefix) : len(rest)-len(suffix)]
	if key == "" || strings.Contains(key, `"`) {
		return Selector{}, fmt.Errorf("%q must name a single attribute inside attributes[\"...\"]", s)
	}
	return Selector{Scope: scope, Key: key}, nil
}

// ParseSelectors parses every fingerprint entry, reporting the index of the
// first invalid one.
func ParseSelectors(entries []string) ([]Selector, error) {
	selectors := make([]Selector, len(entries))
	for i, e := range entries {
		sel, err := ParseSelector(e)
		if err != nil {
			return nil, fmt.Errorf("fingerprint_attributes[%d]: %w", i, err)
		}
		selectors[i] = sel
	}
	return selectors, nil
}
