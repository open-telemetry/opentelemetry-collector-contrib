// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"errors"
	"fmt"
	"strings"
	"time"
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

// Kind names what a fingerprint selector extracts.
type Kind int

const (
	// KindAttribute reads a named attribute from the configured origin.
	KindAttribute Kind = iota
	// KindSpanDuration buckets each selected span's (end - start) duration.
	KindSpanDuration
	// KindTraceDuration buckets the trace latency (latest end - earliest start).
	KindTraceDuration
)

// Selector is a parsed fingerprint entry. Attribute selectors name an origin
// and attribute key. Duration selectors bucket span or trace latency using
// the sampler's duration_buckets cut points. Without the root prefix a
// selector reads across every span in the trace; with it, only the spans
// matching the processor's root-span condition (attribute and span-duration
// selectors only; trace.duration is always whole-trace).
type Selector struct {
	// Kind selects attribute vs duration extraction.
	Kind Kind
	// Root restricts the read to the matched root span. When false the
	// selector reads across the whole trace. Ignored for KindTraceDuration.
	Root bool
	// Origin names which attribute set (resource, scope, span, or their
	// union) the selector reads. Only used for KindAttribute.
	Origin Origin
	// Key is the attribute name to read. Only used for KindAttribute.
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
// span, or any; or a duration selector `span.duration`, `root.span.duration`,
// or `trace.duration`. Prefixing attribute and span.duration selectors with
// `root.` restricts the read to the trace's root span.
func ParseSelector(s string) (Selector, error) {
	rest := s
	var root bool
	if strings.HasPrefix(rest, rootPrefix) {
		root = true
		rest = rest[len(rootPrefix):]
	}

	switch rest {
	case "trace.duration":
		if root {
			return Selector{}, fmt.Errorf("%q: root.trace.duration is invalid; use trace.duration for whole-trace latency", s)
		}
		return Selector{Kind: KindTraceDuration}, nil
	case "span.duration":
		return Selector{Kind: KindSpanDuration, Root: root, Origin: OriginSpan}, nil
	}

	originName, tail, found := strings.Cut(rest, ".")
	origin, known := originNames[originName]
	if !found || !known {
		if root {
			return Selector{}, fmt.Errorf("%q: root must name an origin; use root.(resource|scope|span|any).attributes[\"<name>\"] or root.span.duration", s)
		}
		return Selector{}, fmt.Errorf("%q is not a scoped attribute or duration selector; use (resource|scope|span|any).attributes[\"<name>\"], span.duration, root.span.duration, or trace.duration", s)
	}

	if tail == "duration" {
		return Selector{}, fmt.Errorf("%q: duration selectors must be span.duration, root.span.duration, or trace.duration", s)
	}

	const prefix, suffix = `attributes["`, `"]`
	if !strings.HasPrefix(tail, prefix) || !strings.HasSuffix(tail, suffix) {
		return Selector{}, fmt.Errorf("%q must have the form %s.attributes[\"<name>\"]", s, originName)
	}
	key := tail[len(prefix) : len(tail)-len(suffix)]
	if key == "" || strings.Contains(key, `"`) {
		return Selector{}, fmt.Errorf("%q must name a single attribute inside attributes[\"...\"]", s)
	}
	return Selector{Kind: KindAttribute, Root: root, Origin: origin, Key: key}, nil
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

// HasDurationSelector reports whether any selector buckets span or trace
// duration and therefore requires duration_buckets.
func HasDurationSelector(selectors []Selector) bool {
	for _, sel := range selectors {
		if sel.Kind == KindSpanDuration || sel.Kind == KindTraceDuration {
			return true
		}
	}
	return false
}

// ValidateDurationBuckets checks that cut points are strictly increasing and
// strictly positive.
func ValidateDurationBuckets(buckets []time.Duration) error {
	if len(buckets) == 0 {
		return errors.New("duration_buckets must contain at least one cut point")
	}
	prev := buckets[0]
	if prev <= 0 {
		return fmt.Errorf("duration_buckets[%d] must be greater than zero", 0)
	}
	for i := 1; i < len(buckets); i++ {
		b := buckets[i]
		if b <= 0 {
			return fmt.Errorf("duration_buckets[%d] must be greater than zero", i)
		}
		if b <= prev {
			return fmt.Errorf("duration_buckets must be strictly increasing; duration_buckets[%d]=%s is not greater than duration_buckets[%d]=%s", i, b, i-1, prev)
		}
		prev = b
	}
	return nil
}

// DurationBucketLabel maps a duration onto half-open buckets defined by the
// ascending cut points. For cuts [2ms, 24ms, 1s] the labels are
// `<2ms`, `2ms-24ms`, `24ms-1s`, and `>=1s`.
func DurationBucketLabel(d time.Duration, buckets []time.Duration) string {
	if len(buckets) == 0 {
		return missingValuePlaceholder
	}
	if d < buckets[0] {
		return "<" + buckets[0].String()
	}
	for i := 0; i < len(buckets)-1; i++ {
		if d < buckets[i+1] {
			return buckets[i].String() + "-" + buckets[i+1].String()
		}
	}
	return ">=" + buckets[len(buckets)-1].String()
}
