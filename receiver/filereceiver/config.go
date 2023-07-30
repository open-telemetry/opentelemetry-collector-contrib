// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the file receiver.
type Config struct {
	// Path of the file to read from. Path is relative to current directory.
	Path string `mapstructure:"path"`
	// Throttle determines how fast telemetry is replayed. A value of zero means
	// that it will be replayed as fast as the system will allow. A value of 1 means
	// that it will be replayed at the same rate as the data came in, as indicated
	// by the timestamps on the input file's telemetry data. Higher values mean that
	// replay will be slower by a corresponding amount. Use a value between 0 and 1
	// to replay telemetry at a higher speed. Default: 1.
	Throttle float64 `mapstructure:"throttle"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Throttle: 1,
	}
}

func (c Config) Validate() error {
	if c.Path == "" {
		return errors.New("path cannot be empty")
	}
	if c.Throttle < 0 {
		return errors.New("throttle cannot be negative")
	}
	return nil
}
