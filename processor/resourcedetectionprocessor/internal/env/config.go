// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package env // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"

// Config holds user-specified configuration for the env detector.
type Config struct {
	// Allowlist restricts which attribute keys from the environment variable
	// are emitted. If unset, all keys are emitted; otherwise only the listed keys.
	Allowlist []string `mapstructure:"allow_list"`
}

// CreateDefaultConfig returns the default configuration for the env detector.
func CreateDefaultConfig() Config {
	return Config{}
}
