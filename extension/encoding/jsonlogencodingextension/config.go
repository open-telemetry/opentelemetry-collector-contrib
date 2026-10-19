// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

import "fmt"

type JSONEncodingMode string

const (
	JSONEncodingModeBodyWithInlineAttributes JSONEncodingMode = "body_with_inline_attributes"
	JSONEncodingModeBody                     JSONEncodingMode = "body"
)

type Config struct {
	// Export raw log string instead of log wrapper
	Mode      JSONEncodingMode `mapstructure:"mode,omitempty"`
	ArrayMode bool             `mapstructure:"array_mode,omitempty"`
	// ParseInts preserves integer literals within the int64 range as int64.
	// Decimal, exponent, and out-of-range integer literals are converted to float64.
	ParseInts bool `mapstructure:"parse_ints,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	// validate marshaling mode
	switch c.Mode {
	case JSONEncodingModeBodyWithInlineAttributes, JSONEncodingModeBody:
		return nil
	}

	return fmt.Errorf("invalid mode %q", c.Mode)
}
