// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package canonicaljson // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/canonicaljson"

import (
	"fmt"
	"unicode/utf8"
)

// checkUTF8 validates that a string contains valid UTF-8.
// json.Marshal would replace invalid bytes with U+FFFD, causing
// different inputs to produce identical canonical JSON.
func checkUTF8(s, what string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%s contains invalid UTF-8", what)
	}
	return nil
}
