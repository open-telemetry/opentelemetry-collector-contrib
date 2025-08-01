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
