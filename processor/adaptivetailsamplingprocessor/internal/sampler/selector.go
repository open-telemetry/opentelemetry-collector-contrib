// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"fmt"
	"strings"
)

// Origin names which attribute set a fingerprint selector reads from. The
// resource, scope, and span origins match OTTL's span-context path names so
// conditions and fingerprint entries share one spelling. any is a trace-level
// origin OTTL cannot express (it unions the other three).
type Origin int

const (
	// OriginResource reads resource attributes.
	OriginResource Origin = iota
	// OriginScope reads instrumentation scope attributes.
	OriginScope
	// OriginSpan reads span attributes.
	OriginSpan
	// OriginAny reads the union of resource, scope, and span attributes.
	OriginAny
)

// Selector is a parsed fingerprint entry: an attribute origin, an attribute
// key, and whether the read is restricted to the trace's root span. Without
// the root prefix a selector reads across every span in the trace; with it,
// only the spans matching the processor's root-span condition.
type Selector struct {
	// Root restricts the read to the matched root span. When false the
	// selector reads across the whole trace.
	Root bool
	// Origin names which attribute set (resource, scope, span, or their
	// union) the selector reads.
	Origin Origin
	// Key is the attribute name to read.
	Key string
}

const rootPrefix = "root."

var originNames = map[string]Origin{
	"resource": OriginResource,
	"scope":    OriginScope,
	"span":     OriginSpan,
	"any":      OriginAny,
}

// ParseSelector parses a fingerprint entry of the form
// `<origin>.attributes["<key>"]`, where origin is one of resource, scope,
// span, or any. Prefixing with `root.` (e.g. `root.resource.attributes["..."]`)
// restricts the read to the trace's root span.
func ParseSelector(s string) (Selector, error) {
	rest := s
	var root bool
	if strings.HasPrefix(rest, rootPrefix) {
		root = true
		rest = rest[len(rootPrefix):]
	}

	originName, tail, found := strings.Cut(rest, ".")
	origin, known := originNames[originName]
	if !found || !known {
		if root {
			return Selector{}, fmt.Errorf("%q: root must name an origin; use root.(resource|scope|span|any).attributes[\"<name>\"]", s)
		}
		return Selector{}, fmt.Errorf("%q is not a scoped attribute selector; use (resource|scope|span|any).attributes[\"<name>\"] optionally prefixed with root., e.g. any.attributes[%q]", s, s)
	}

	const prefix, suffix = `attributes["`, `"]`
	if !strings.HasPrefix(tail, prefix) || !strings.HasSuffix(tail, suffix) {
		return Selector{}, fmt.Errorf("%q must have the form %s.attributes[\"<name>\"]", s, originName)
	}
	key := tail[len(prefix) : len(tail)-len(suffix)]
	if key == "" || strings.Contains(key, `"`) {
		return Selector{}, fmt.Errorf("%q must name a single attribute inside attributes[\"...\"]", s)
	}
	return Selector{Root: root, Origin: origin, Key: key}, nil
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
