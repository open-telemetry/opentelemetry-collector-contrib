// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

import (
	"fmt"
	"strings"

	"github.com/goccy/go-json"
)

type JSONEncodingMode string

const (
	JSONEncodingModeBodyWithInlineAttributes JSONEncodingMode = "body_with_inline_attributes"
	JSONEncodingModeBody                     JSONEncodingMode = "body"
)

type Config struct {
	// Export raw log string instead of log wrapper
	Mode      JSONEncodingMode `mapstructure:"mode,omitempty"`
	ArrayMode bool             `mapstructure:"array_mode,omitempty"`

	// Unwrap is a JSONPath expression used to extract individual log records
	// from a wrapper structure during unmarshaling. For example, "$.records[*]"
	// extracts each element from a {"records": [...]} wrapper. When set,
	// array_mode is ignored for unmarshaling. Leave empty to use existing
	// array_mode behavior.
	Unwrap string `mapstructure:"unwrap,omitempty"`

	// UnwrapTarget is the body map key where the raw JSON string of each
	// extracted record is stored. When set (e.g., "message"), the body becomes
	// a map like {"message": "<raw JSON>"}. When empty, the JSON is fully
	// parsed into an OpenTelemetry map (the default behavior). Only used when
	// unwrap is set.
	UnwrapTarget string `mapstructure:"unwrap_target,omitempty"`

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

	// validate unwrap JSONPath expression if set
	if c.Unwrap != "" {
		if !strings.HasPrefix(c.Unwrap, "$") {
			return fmt.Errorf("invalid unwrap expression %q: must be a JSONPath starting with $", c.Unwrap)
		}
		if _, err := json.CreatePath(c.Unwrap); err != nil {
			return fmt.Errorf("invalid unwrap JSONPath expression %q: %w", c.Unwrap, err)
		}
	}

	// unwrap_target requires unwrap
	if c.UnwrapTarget != "" && c.Unwrap == "" {
		return fmt.Errorf("unwrap_target requires unwrap to be set")
	}

	return nil
}
