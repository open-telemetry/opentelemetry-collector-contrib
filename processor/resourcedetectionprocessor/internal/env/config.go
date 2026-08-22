// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package env // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"

// Config holds user-specified configuration for the env detector.
type Config struct {
	// Attributes filters the resource attributes emitted by the env detector.
	Attributes AttributesConfig `mapstructure:"attributes"`
}

// AttributesConfig configures which resource attribute keys the env detector emits.
// Both included and excluded entries support `*` as a wildcard matching zero or more
// characters. When included is empty, every key is considered included by default.
// excluded is applied after included, so a key matched by both is dropped.
// See https://github.com/open-telemetry/opentelemetry-configuration/blob/v1.1.0/schema/common.yaml#L2-L27
type AttributesConfig struct {
	Included []string `mapstructure:"included"`
	Excluded []string `mapstructure:"excluded"`
}

// CreateDefaultConfig returns the default configuration for the env detector.
func CreateDefaultConfig() Config {
	return Config{}
}
