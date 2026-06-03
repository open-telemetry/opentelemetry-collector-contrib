// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package shared // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"

import (
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
)

// strcase incorrectly treats digit-to-letter transitions as word boundaries,
// inserting underscores (e.g. "k8s" -> "k8_s").
// This regex reverses that.
// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/46571
var digitToLetter = regexp.MustCompile(`(\d)_([a-z])`)

// ToSnakeCase converts a string to snake_case while preserving characters in
// the ignore set, and fixes strcase's incorrect splitting at digit-to-letter
// boundaries like "k8s" into "k8_s".
func ToSnakeCase(s, ignore string) string {
	result := strcase.ToSnakeWithIgnore(s, ignore)
	// Fast path: if there are no digits, skip the regex entirely
	if !strings.ContainsAny(result, "0123456789") {
		return result
	}
	return digitToLetter.ReplaceAllString(result, "${1}${2}")
}
