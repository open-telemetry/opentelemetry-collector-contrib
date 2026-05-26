// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package custom implements the value transformer for the
// genainormalizerprocessor's "custom" source.
package custom // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/custom"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Transform returns a value transformer driven by valueMappings, keyed by
// the post-rename target attribute name. Source values are matched
// case-insensitively. Non-string sources, missing rules, and unmatched
// values fall through to a verbatim copy.
func Transform(valueMappings map[string]map[string]string) func(targetKey string, src, dst pcommon.Value) {
	return func(targetKey string, src, dst pcommon.Value) {
		if rules, ok := valueMappings[targetKey]; ok && src.Type() == pcommon.ValueTypeStr {
			if mapped, ok := rules[strings.ToLower(src.Str())]; ok {
				dst.SetStr(mapped)
				return
			}
		}
		src.CopyTo(dst)
	}
}
