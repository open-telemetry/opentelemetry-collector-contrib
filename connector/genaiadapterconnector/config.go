// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genaiadapterconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector"

// Config defines configuration for the genaiadapterconnector.
type Config struct{}

// Validate checks the connector configuration for errors.
func (cfg *Config) Validate() error {
	return nil
}
