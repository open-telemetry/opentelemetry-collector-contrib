// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// resolveMapping walks an ordered list of sources and returns the first
// non-empty value, plus true. Sources are interpreted as follows:
//   - An entry containing "." is treated as a resource attribute key;
//     resolveMapping looks it up in attrs and uses its string value when
//     present and non-empty.
//   - An entry not containing "." is treated as a string literal terminal
//     default and is always selected when reached.
//
// If no source resolves to a non-empty value, ("", false) is returned and the
// caller is expected to omit the destination tag.
//
// When sources is nil or empty the function returns ("", false); callers
// should treat that as "no override configured" and fall back to their
// historical hardcoded behavior. Validation rejects an explicitly-empty
// configured slice (see TagMappingsConfig.Validate), so an empty slice at this
// layer can only originate from a caller that did not configure tag mappings.
func resolveMapping(attrs pcommon.Map, sources []string) (string, bool) {
	for _, src := range sources {
		if !strings.Contains(src, ".") {
			// Terminal literal default.
			return src, true
		}
		if v, ok := attrs.Get(src); ok {
			if s := v.Str(); s != "" {
				return s, true
			}
		}
	}
	return "", false
}
