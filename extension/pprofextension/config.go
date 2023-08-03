// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
)

// Config has the configuration for the extension enabling the golang
// net/http/pprof (Performance Profiler) extension.
type Config struct {

	// TCPAddr is the address and port in which the pprof will be listening to.
	// Use localhost:<port> to make it available only locally, or ":<port>" to
	// make it available on all network interfaces.
	TCPAddr confignet.TCPAddr `mapstructure:",squash"`

	// Fraction of blocking events that are profiled. A value <= 0 disables
	// profiling. See https://golang.org/pkg/runtime/#SetBlockProfileRate for details.
	BlockProfileFraction int `mapstructure:"block_profile_fraction"`

	// Fraction of mutex contention events that are profiled. A value <= 0
	// disables profiling. See https://golang.org/pkg/runtime/#SetMutexProfileFraction
	// for details.
	MutexProfileFraction int `mapstructure:"mutex_profile_fraction"`

	// Optional file name to save the CPU profile to. The profiling starts when the
	// Collector starts and is saved to the file when the Collector is terminated.
	SaveToFile string `mapstructure:"save_to_file"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
