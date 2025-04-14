// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"errors"

	"go.opentelemetry.io/collector/config/configopaque"
)

var (
	errMissingHeader        = errors.New("missing header name")
	errMissingHeadersConfig = errors.New("missing headers configuration")
	errMissingSource        = errors.New("missing header source, must be 'from_context', 'from_attribute' or 'value'")
	errConflictingSources   = errors.New("invalid header source, must either 'from_context', 'from_attribute' or 'value'")
)

type Config struct {
	HeadersConfig []HeaderConfig `mapstructure:"headers"`
}

type HeaderConfig struct {
	Action        ActionValue          `mapstructure:"action"`
	Key           *string              `mapstructure:"key"`
	Value         *string              `mapstructure:"value"`
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
			if header.FromContext == nil && header.FromAttribute == nil && header.Value == nil {
				return errMissingSource
			}
			if (header.FromContext != nil && header.FromAttribute != nil) ||
				(header.FromContext != nil && header.Value != nil) ||
				(header.Value != nil && header.FromAttribute != nil) {
				return errConflictingSources
			}
		}
	}
	return nil
}
