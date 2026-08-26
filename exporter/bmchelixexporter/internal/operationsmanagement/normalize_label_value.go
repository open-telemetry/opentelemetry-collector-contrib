// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/operationsmanagement"

import (
	"strings"
)

// NormalizeLabelValue normalizes the label value by replacing commas with whitespace.
// Commas are not allowed in label values as they may interfere with parsing.
func NormalizeLabelValue(value string) string {
	return strings.ReplaceAll(value, ",", " ")
}
