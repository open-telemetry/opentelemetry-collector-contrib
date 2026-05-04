// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchlogencodingextension // import "github.com/cloudoperators/opentelemetry-collector-contrib/extension/encoding/opensearchlogencodingextension"

// Config holds configuration for the OpenSearch log encoding extension.
type Config struct {
	_ struct{} // prevent unkeyed literal initialization
}

// Validate checks the extension configuration.
func (c *Config) Validate() error {
	return nil
}
