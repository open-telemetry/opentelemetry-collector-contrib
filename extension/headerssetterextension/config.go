// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
)

var (
	errMissingHeader        = errors.New("missing header name")
	errMissingHeadersConfig = errors.New("missing headers configuration")
	errMissingSource        = errors.New("missing header source, must be 'from_context', 'from_attribute', 'value', or 'value_file'")
	errConflictingSources   = errors.New("invalid header source, must be only one of 'from_context', 'from_attribute', 'value', or 'value_file'")
)

type Config struct {
	HeadersConfig  []HeaderConfig `mapstructure:"headers"`
	AdditionalAuth *component.ID  `mapstructure:"additional_auth"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type HeaderConfig struct {
	Action        ActionValue          `mapstructure:"action"`
	Key           *string              `mapstructure:"key"`
	Value         *string              `mapstructure:"value"`
	ValueFile     *string              `mapstructure:"value_file"`
	FromContext   *string              `mapstructure:"from_context"`
	FromAttribute *string              `mapstructure:"from_attribute"`
	DefaultValue  *configopaque.String `mapstructure:"default_value"`
}

// ActionValue is the enum to capture the four types of actions to perform on a header
type ActionValue string

const (
	// INSERT inserts the new header if it does not exist
	INSERT ActionValue = "insert"

	// UPDATE updates the header value if it exists
	UPDATE ActionValue = "update"

	// UPSERT inserts a header if it does not exist and updates the header
	// if it exists
	UPSERT ActionValue = "upsert"

	// DELETE deletes the header
	DELETE ActionValue = "delete"
)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.HeadersConfig) == 0 {
		return errMissingHeadersConfig
	}
	for _, header := range cfg.HeadersConfig {
		if header.Key == nil || *header.Key == "" {
			return errMissingHeader
		}

		if header.Action != DELETE {
			if header.FromContext == nil && header.FromAttribute == nil && header.Value == nil && header.ValueFile == nil {
				return errMissingSource
			}
			sourceCount := 0
			if header.FromContext != nil {
				sourceCount++
			}
			if header.FromAttribute != nil {
				sourceCount++
			}
			if header.Value != nil {
				sourceCount++
			}
			if header.ValueFile != nil {
				sourceCount++
			}
			if sourceCount > 1 {
				return errConflictingSources
			}
		}
	}
	return nil
}
