// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

type Config struct{
	// Export raw log string instead of log wrapper
	RawLog bool `mapstructure:"raw_log,omitempty"`
}

func (c *Config) Validate() error {
	return nil
}
