// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unrollprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/unrollprocessor"

// Config is the configuration for the unroll processor.
type Config struct {
	Recursive bool `mapstructure:"recursive"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate is a no-op for this as there's no configuration that is possibly invalid after unmarshalling
func (*Config) Validate() error {
	return nil
}
