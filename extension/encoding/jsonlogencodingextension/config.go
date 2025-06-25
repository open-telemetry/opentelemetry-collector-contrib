// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

import "fmt"

type JSONEncodingMode string

type ProcessingMode string

const (
	JSONEncodingModeBodyWithInlineAttributes JSONEncodingMode = "body_with_inline_attributes"
	JSONEncodingModeBody                     JSONEncodingMode = "body"
	ArrayMode                                ProcessingMode   = "array"
	SingleMode                               ProcessingMode   = "single"
	NDJsonMode                               ProcessingMode   = "ndjson"
)

type Config struct {
	// Export raw log string instead of log wrapper
	Mode           JSONEncodingMode `mapstructure:"mode,omitempty"`
	ProcessingMode ProcessingMode   `mapstructure:"processing_mode,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	// validate marshaling mode
	switch c.Mode {
	case JSONEncodingModeBodyWithInlineAttributes, JSONEncodingModeBody:
	default:
		return fmt.Errorf("invalid mode %q", c.Mode)
	}

	// validate unmarshaling mode
	switch c.ProcessingMode {
	case ArrayMode, SingleMode, NDJsonMode:
		return nil
	default:
		return fmt.Errorf("invalid decoding mode %q", c.ProcessingMode)
	}
}
