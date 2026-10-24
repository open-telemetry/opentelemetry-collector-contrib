// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnetreceiver"

import (
	"errors"
)

// Config holds configuration for the gnet receiver.
type Config struct {
	// Endpoint is the TCP address the receiver listens on.
	// Default: 0.0.0.0:4320
	Endpoint string `mapstructure:"endpoint"`

	// NumEventLoops is the number of gnet event-loop goroutines.
	// 0 means one per logical CPU core (recommended).
	NumEventLoops int `mapstructure:"num_event_loops"`

	// ReadBufferCap is the initial per-connection read buffer size in bytes.
	// Default: 4096
	ReadBufferCap int `mapstructure:"read_buffer_cap"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint must not be empty")
	}
	if c.NumEventLoops < 0 {
		return errors.New("num_event_loops must be >= 0")
	}
	if c.ReadBufferCap < 0 {
		return errors.New("read_buffer_cap must be >= 0")
	}
	return nil
}
