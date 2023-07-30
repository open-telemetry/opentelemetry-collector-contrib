// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subprocessmanager // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"

// SubprocessConfig is the config definition for the subprocess manager
type SubprocessConfig struct {
	// Command is the command to be run (binary + flags, separated by commas)
	Command string `mapstructure:"exec"`
	// Env is a list of env variables to pass to a specific command
	Env []EnvConfig `mapstructure:"env"`
}

// EnvConfig is the config definition of each key-value pair for environment variables
type EnvConfig struct {
	// Name is the name of the environment variable
	Name string `mapstructure:"name"`
	// Value is the value of the variable
	Value string `mapstructure:"value"`
}
