// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// Payload modes for CloudWatchStream.Payload. See README for semantics.
const (
	PayloadMessage  = "message"
	PayloadEnvelope = "envelope"
)

// CloudWatchStream maps a logGroup and/or logStream pattern (or a known
// service name) to an inner encoding extension.
type CloudWatchStream struct {
	Name             string       `mapstructure:"name"`
	Encoding         component.ID `mapstructure:"encoding"`
	LogGroupPattern  string       `mapstructure:"log_group_pattern"`
	LogStreamPattern string       `mapstructure:"log_stream_pattern"`
	Payload          string       `mapstructure:"payload"`
}

var defaultCWPatterns = map[string]CloudWatchStream{
	"vpcflow":    {Name: "vpcflow", LogStreamPattern: "eni-*", Payload: PayloadEnvelope},
	"cloudtrail": {Name: "cloudtrail", LogStreamPattern: "*_CloudTrail_*", Payload: PayloadEnvelope},
	"lambda":     {Name: "lambda", LogGroupPattern: "/aws/lambda/*", Payload: PayloadMessage},
	"waf":        {Name: "waf", LogGroupPattern: "aws-waf-logs-*", Payload: PayloadMessage},
	"rds":        {Name: "rds", LogGroupPattern: "/aws/rds/instance/*/*", Payload: PayloadMessage},
	"eks":        {Name: "eks", LogGroupPattern: "/aws/eks/*", Payload: PayloadMessage},
	"apigateway": {Name: "apigateway", LogGroupPattern: "API-Gateway-Execution-Logs_*", Payload: PayloadMessage},
}

// withDefaults fills in unset fields from defaultCWPatterns. User-supplied
// values are never overwritten.
func (r CloudWatchStream) withDefaults() CloudWatchStream {
	d, ok := defaultCWPatterns[r.Name]
	if !ok {
		return r
	}
	if r.LogGroupPattern == "" && r.LogStreamPattern == "" {
		r.LogGroupPattern = d.LogGroupPattern
		r.LogStreamPattern = d.LogStreamPattern
	}
	if r.Payload == "" {
		r.Payload = d.Payload
	}
	return r
}

// ValidateStreams reports configuration errors in a list of CloudWatch streams:
// missing name, missing encoding, missing pattern when the name has no
// defaults, invalid payload mode, and duplicate names.
func ValidateStreams(streams []CloudWatchStream) error {
	var errs []error
	seenNames := make(map[string]int, len(streams))

	for i, r := range streams {
		// Name is required: it identifies the stream in logs / errors and
		// is the deduplication key.
		if r.Name == "" {
			errs = append(errs, fmt.Errorf("cloudwatch.streams[%d]: 'name' is required", i))
		}

		// Encoding is required.
		if r.Encoding == (component.ID{}) {
			errs = append(errs, fmt.Errorf("cloudwatch.streams[%d]: 'encoding' is required", i))
		}

		// When the user supplied no patterns, the name must be a known
		// default; otherwise the stream has no way to match anything.
		hasPattern := r.LogGroupPattern != "" || r.LogStreamPattern != ""
		if !hasPattern && r.Name != "" {
			if _, ok := defaultCWPatterns[r.Name]; !ok {
				errs = append(errs, fmt.Errorf(
					"cloudwatch.streams[%d]: name %q has no defaults; set 'log_group_pattern' or 'log_stream_pattern'",
					i, r.Name,
				))
			}
		}

		// Payload, if set, must be a known mode.
		switch r.Payload {
		case "", PayloadMessage, PayloadEnvelope:
		default:
			errs = append(errs, fmt.Errorf(
				"cloudwatch.streams[%d]: invalid 'payload' value %q; expected %q or %q",
				i, r.Payload, PayloadMessage, PayloadEnvelope,
			))
		}

		// Duplicate name.
		if r.Name != "" {
			if prior, ok := seenNames[r.Name]; ok {
				errs = append(errs, fmt.Errorf(
					"cloudwatch.streams[%d]: duplicate name %q also at cloudwatch.streams[%d]",
					i, r.Name, prior,
				))
			} else {
				seenNames[r.Name] = i
			}
		}
	}

	return errors.Join(errs...)
}

// sortStreams returns a copy of routes ordered by routing precedence:
//
//  1. Catch-all entries (a pattern equal to "*") are placed last.
//  2. Entries with a log_group_pattern come before entries that use only
//     log_stream_pattern.
//  3. Within each group, more-specific patterns come first
//     (see comparePatternSpecificity).
//
// Defaults are applied to each stream via withDefaults before sorting, so the
// returned slice has all patterns populated for known names.
//
// Stable order is preserved among entries that compare equal.
func sortStreams(streams []CloudWatchStream) []CloudWatchStream {
	if len(streams) == 0 {
		return nil
	}

	sorted := make([]CloudWatchStream, len(streams))
	for i, r := range streams {
		sorted[i] = r.withDefaults()
	}

	// Pre-split non-empty patterns once and key by pattern string so the cache
	// remains correct as the sort swaps elements.
	splitCache := make(map[string][]string, len(sorted)*2)
	for _, r := range sorted {
		for _, p := range [...]string{r.LogGroupPattern, r.LogStreamPattern} {
			if p == "" {
				continue
			}
			if _, ok := splitCache[p]; !ok {
				splitCache[p] = strings.Split(p, "/")
			}
		}
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		return compareStreams(sorted[i], sorted[j], splitCache) < 0
	})
	return sorted
}

// compareStreams implements the three-level routing precedence rule.
func compareStreams(a, b CloudWatchStream, splitCache map[string][]string) int {
	// Level 1: catch-all "*" goes last.
	aIsCatchAll := a.LogGroupPattern == catchAllPattern || a.LogStreamPattern == catchAllPattern
	bIsCatchAll := b.LogGroupPattern == catchAllPattern || b.LogStreamPattern == catchAllPattern
	if aIsCatchAll != bIsCatchAll {
		if aIsCatchAll {
			return 1
		}
		return -1
	}

	// Level 2: log_group_pattern before log_stream-only entries.
	aHasGroup := a.LogGroupPattern != ""
	bHasGroup := b.LogGroupPattern != ""
	if aHasGroup != bHasGroup {
		if aHasGroup {
			return -1
		}
		return 1
	}

	// Level 3: more-specific pattern first.
	var patA, patB string
	if aHasGroup {
		patA, patB = a.LogGroupPattern, b.LogGroupPattern
	} else {
		patA, patB = a.LogStreamPattern, b.LogStreamPattern
	}
	return comparePatternSpecificity(splitCache[patA], splitCache[patB])
}

// resolvedStream is a CloudWatchStream with patterns pre-split for fast matching
// and the inner extension already resolved to a typed
// LogsUnmarshalerExtension.
type resolvedStream struct {
	name           string
	inner          encoding.LogsUnmarshalerExtension
	logGroupParts  []string // nil if log_group_pattern is empty after defaults
	logStreamParts []string // nil if log_stream_pattern is empty after defaults
}

// Router dispatches CloudWatch subscription-filter events to inner encoding
// extensions based on logGroup / logStream pattern matching.
type Router struct {
	streams []resolvedStream
}

// NewRouter constructs a Router from validated routes. It applies defaults,
// sorts routes by precedence, and resolves each stream's component.ID against
// host.GetExtensions(), failing fast if any ID is missing, points at a
// component that does not implement encoding.LogsUnmarshalerExtension, or
// refers back to selfID (cycle).
//
// selfID is the component.ID of the extension that owns this router.
func NewRouter(
	streams []CloudWatchStream,
	host component.Host,
	selfID component.ID,
) (*Router, error) {
	sorted := sortStreams(streams)
	extensions := host.GetExtensions()

	resolved := make([]resolvedStream, 0, len(sorted))
	for _, r := range sorted {
		if r.Encoding == selfID {
			return nil, fmt.Errorf(
				"cloudwatch.streams[name=%q]: encoding %q refers back to this extension (cycle)",
				r.Name, r.Encoding,
			)
		}

		ext, ok := extensions[r.Encoding]
		if !ok {
			return nil, fmt.Errorf(
				"cloudwatch.streams[name=%q]: encoding extension %q not found",
				r.Name, r.Encoding,
			)
		}

		inner, ok := ext.(encoding.LogsUnmarshalerExtension)
		if !ok {
			return nil, fmt.Errorf(
				"cloudwatch.streams[name=%q]: extension %q does not implement encoding.LogsUnmarshalerExtension",
				r.Name, r.Encoding,
			)
		}

		rr := resolvedStream{name: r.Name, inner: inner}
		if r.LogGroupPattern != "" {
			rr.logGroupParts = strings.Split(r.LogGroupPattern, "/")
		}
		if r.LogStreamPattern != "" {
			rr.logStreamParts = strings.Split(r.LogStreamPattern, "/")
		}
		resolved = append(resolved, rr)
	}

	return &Router{streams: resolved}, nil
}

// Match returns the inner encoding extension that should handle a CloudWatch
// subscription-filter event with the given logGroup and logStream values. It
// iterates routes in precedence order; the first route with a matching
// pattern wins. Within a single route, log_group_pattern is checked before
// log_stream_pattern.
//
// The route's name is returned alongside the inner extension for logging /
// observability. If no route matches, an error is returned.
func (r *Router) Match(logGroup, logStream string) (encoding.LogsUnmarshalerExtension, string, error) {
	var groupParts, streamParts []string
	for _, rr := range r.streams {
		if rr.logGroupParts != nil {
			if groupParts == nil {
				groupParts = strings.Split(logGroup, "/")
			}
			if matchPrefixWithWildcard(groupParts, rr.logGroupParts) {
				return rr.inner, rr.name, nil
			}
		}
		if rr.logStreamParts != nil {
			if streamParts == nil {
				streamParts = strings.Split(logStream, "/")
			}
			if matchPrefixWithWildcard(streamParts, rr.logStreamParts) {
				return rr.inner, rr.name, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no route matches logGroup=%q logStream=%q", logGroup, logStream)
}
