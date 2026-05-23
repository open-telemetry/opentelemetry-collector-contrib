// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package url // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/url"

type URLSanitizationConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Attributes is the list of attributes that will be sanitized.
	Attributes []string `mapstructure:"attributes"`
	// SanitizeSpanName controls whether span names should be sanitized.
	SanitizeSpanName *bool `mapstructure:"sanitize_span_name"`
}
