// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package url // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/url"

import "errors"

type URLSanitizationConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// MaxSegments is the maximum number of segments in a path.
	MaxSegments int `mapstructure:"max_segments"`
	// ReplaceWith is the character that will replace the segments in a path.
	ReplaceWith string `mapstructure:"replace_with"`
	// CacheSize is the size of the cache for the classifier.
	CacheSize int `mapstructure:"cache_size"`
	// Attributes is the list of attributes that will be sanitized.
	Attributes []string `mapstructure:"attributes"`
	// SanitizeSpanName is a flag to sanitize the span name.
	SanitizeSpanName bool `mapstructure:"sanitize_span_name"`
}

func (u *URLSanitizationConfig) Validate() error {
	if len(u.ReplaceWith) > 1 {
		return errors.New("replace_with must be a single character")
	}

	return nil
}
