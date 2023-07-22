// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"fmt"
)

var (
	errMissingHeader        = fmt.Errorf("missing header name")
	errMissingHeadersConfig = fmt.Errorf("missing headers configuration")
	errMissingSource        = fmt.Errorf("missing header source, must be 'from_context' or 'value'")
	errConflictingSources   = fmt.Errorf("invalid header source, must either 'from_context' or 'value'")
)

type Config struct {
	HeadersConfig []HeaderConfig `mapstructure:"headers"`
}

type HeaderConfig struct {
	Action      ActionValue `mapstructure:"action"`
	Key         *string     `mapstructure:"key"`
	Value       *string     `mapstructure:"value"`
	FromContext *string     `mapstructure:"from_context"`
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
	if cfg.HeadersConfig == nil || len(cfg.HeadersConfig) == 0 {
		return errMissingHeadersConfig
	}
	for _, header := range cfg.HeadersConfig {
		if header.Key == nil || *header.Key == "" {
			return errMissingHeader
		}

		if header.Action != DELETE {
			if header.FromContext == nil && header.Value == nil {
				return errMissingSource
			}
			if header.FromContext != nil && header.Value != nil {
				return errConflictingSources
			}
		}
	}
	return nil
}
