// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import "fmt"

// Config defines configuration for the fluentforward receiver.
type Config struct {
	// The address to listen on for incoming Fluent Forward events.  Should be
	// of the form `<ip addr>:<port>` (TCP) or `unix://<socket_path>` (Unix
	// domain socket).
	ListenAddress string `mapstructure:"endpoint"`

	// MaxPackedForwardBytes is the maximum number of bytes accepted for a
	// packed-forward raw payload (the entries blob before decompression).
	// Defaults to 16 MiB. Increase this if senders produce large uncompressed
	// PackedForward frames.
	MaxPackedForwardBytes int `mapstructure:"max_packed_forward_bytes"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if c.MaxPackedForwardBytes <= 0 {
		return fmt.Errorf("max_packed_forward_bytes must be positive, got %d", c.MaxPackedForwardBytes)
	}
	if uint64(c.MaxPackedForwardBytes) > uint64(^uint32(0)) {
		return fmt.Errorf("max_packed_forward_bytes must be at most %d, got %d", uint64(^uint32(0)), c.MaxPackedForwardBytes)
	}
	return nil
}
