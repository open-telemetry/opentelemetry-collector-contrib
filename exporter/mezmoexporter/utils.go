// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mezmoexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter"

// truncateString Truncates the given string to a maximum length provided by max.
func truncateString(s string, max int) string {
	if len(s) < max {
		return s
	}

	return s[:max]
}
