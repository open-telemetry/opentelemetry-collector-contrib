// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/operationsmanagement"

import (
	"strings"
)

// NormalizeMetricName normalizes the metric name so that it matches [a-zA-Z_:.][a-zA-Z0-9_:.]*
// Only ASCII letters, digits, underscores, colons, and dots are accepted.
// Unsupported characters are replaced with underscores.
// If the metric name starts with a digit, it is prefixed with an underscore.
// Consecutive underscores are collapsed into a single underscore.
func NormalizeMetricName(name string) string {
	if name == "" {
		return name
	}

	// Replace all unsupported characters with underscores
	name = strings.Map(sanitizeMetricNameRune, name)

	// Metric name cannot start with a digit, so prefix it with "_" in this case
	if name != "" && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	// Collapse consecutive underscores
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}

	return name
}

// sanitizeMetricNameRune returns the rune if it's valid for a metric name,
// otherwise returns '_'.
// Valid characters: ASCII letters (a-z, A-Z), ASCII digits (0-9), underscore, colon, dot.
func sanitizeMetricNameRune(r rune) rune {
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == ':' || r == '.' {
		return r
	}
	return '_'
}
