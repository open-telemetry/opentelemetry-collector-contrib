// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/component"
)

// CloudWatchRoute defines a single routing rule for CloudWatch Logs
// subscription-filter events. It maps a logGroup and/or logStream pattern to
// an inner encoding extension.
type CloudWatchRoute struct {
	// Name identifies the route. Required. It is used as the deduplication key, in
	// log and error messages, and as the lookup key into the default-pattern
	// map. For known names (vpcflow, cloudtrail, lambda, waf, rds, eks,
	// apigateway) the corresponding default log_group_pattern or
	// log_stream_pattern is applied at Start time when the user supplies no
	// explicit pattern. User-supplied patterns always win; defaults never
	// overwrite them.
	Name string `mapstructure:"name"`

	// Encoding is the component ID of an inner encoding extension that
	// implements encoding.LogsUnmarshalerExtension. Required.
	Encoding component.ID `mapstructure:"encoding"`

	// LogGroupPattern is matched against the subscription event's logGroup.
	LogGroupPattern string `mapstructure:"log_group_pattern"`

	// LogStreamPattern is matched against the subscription event's logStream.
	LogStreamPattern string `mapstructure:"log_stream_pattern"`
}

// defaultCWPatterns maps known route names to default log_group / log_stream
// patterns based on AWS conventions. When a user writes a route with only a
// known name and no explicit patterns, the corresponding default is applied
// at Start time.
var defaultCWPatterns = map[string]CloudWatchRoute{
	"vpcflow":    {Name: "vpcflow", LogStreamPattern: "eni-*"},
	"cloudtrail": {Name: "cloudtrail", LogStreamPattern: "*_CloudTrail_*"},
	"lambda":     {Name: "lambda", LogGroupPattern: "/aws/lambda/*"},
	"waf":        {Name: "waf", LogGroupPattern: "aws-waf-logs-*"},
	"rds":        {Name: "rds", LogGroupPattern: "/aws/rds/instance/*/*"},
	"eks":        {Name: "eks", LogGroupPattern: "/aws/eks/*"},
	"apigateway": {Name: "apigateway", LogGroupPattern: "API-Gateway-Execution-Logs_*"},
}

// withDefaults returns a copy of the route with default patterns applied if
// the user supplied no patterns and the name is in the defaults map.
// User-supplied patterns are never overwritten.
func (r CloudWatchRoute) withDefaults() CloudWatchRoute {
	if r.LogGroupPattern != "" || r.LogStreamPattern != "" {
		return r
	}
	if d, ok := defaultCWPatterns[r.Name]; ok {
		r.LogGroupPattern = d.LogGroupPattern
		r.LogStreamPattern = d.LogStreamPattern
	}
	return r
}

// ValidateRoutes reports configuration errors in a list of CloudWatch routes:
// missing name, missing encoding, missing pattern when the name has no
// defaults, and duplicate names.
func ValidateRoutes(routes []CloudWatchRoute) error {
	var errs []error
	seenNames := make(map[string]int, len(routes))

	for i, r := range routes {
		// Name is required: it identifies the route in logs / errors and
		// is the deduplication key.
		if r.Name == "" {
			errs = append(errs, fmt.Errorf("cloudwatch_routes[%d]: 'name' is required", i))
		}

		// Encoding is required.
		if r.Encoding == (component.ID{}) {
			errs = append(errs, fmt.Errorf("cloudwatch_routes[%d]: 'encoding' is required", i))
		}

		// When the user supplied no patterns, the name must be a known
		// default; otherwise the route has no way to match anything.
		hasPattern := r.LogGroupPattern != "" || r.LogStreamPattern != ""
		if !hasPattern && r.Name != "" {
			if _, ok := defaultCWPatterns[r.Name]; !ok {
				errs = append(errs, fmt.Errorf(
					"cloudwatch_routes[%d]: name %q has no defaults; set 'log_group_pattern' or 'log_stream_pattern'",
					i, r.Name,
				))
			}
		}

		// Duplicate name.
		if r.Name != "" {
			if prior, ok := seenNames[r.Name]; ok {
				errs = append(errs, fmt.Errorf(
					"cloudwatch_routes[%d]: duplicate name %q also at cloudwatch_routes[%d]",
					i, r.Name, prior,
				))
			} else {
				seenNames[r.Name] = i
			}
		}
	}

	return errors.Join(errs...)
}

// SortRoutes returns a copy of routes ordered by routing precedence:
//
//  1. Catch-all entries (a pattern equal to "*") are placed last.
//  2. Entries with a log_group_pattern come before entries that use only
//     log_stream_pattern.
//  3. Within each group, more-specific patterns come first
//     (see comparePatternSpecificity).
//
// Defaults are applied to each route via withDefaults before sorting, so the
// returned slice has all patterns populated for known names.
//
// Stable order is preserved among entries that compare equal.
func SortRoutes(routes []CloudWatchRoute) []CloudWatchRoute {
	if len(routes) == 0 {
		return nil
	}

	sorted := make([]CloudWatchRoute, len(routes))
	for i, r := range routes {
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
		return compareRoutes(sorted[i], sorted[j], splitCache) < 0
	})
	return sorted
}

// compareRoutes implements the three-level routing precedence rule.
func compareRoutes(a, b CloudWatchRoute, splitCache map[string][]string) int {
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
