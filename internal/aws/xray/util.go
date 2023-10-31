// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"

// String returns a pointer to the provided string, or nil if it is an empty string.
func String(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// StringOrEmpty returns empty string if the input is nil, otherwise returns the string itself.
func StringOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
