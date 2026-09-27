// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import "errors"

// Config defines configuration for the fluentforward receiver.
type Config struct {
	// The address to listen on for incoming Fluent Forward events.  Should be
	// of the form `<ip addr>:<port>` (TCP) or `unix://<socket_path>` (Unix
	// domain socket).
	ListenAddress string `mapstructure:"endpoint"`

	// MaxConnections caps simultaneously open connections. By default,
	// connections over the cap wait in the accept backlog until a slot frees.
	// 0 means no limit.
	MaxConnections int `mapstructure:"max_connections"`

	// RefuseOverLimit closes connections over MaxConnections on accept
	// instead of queueing them, so long-lived clients reconnect, possibly to
	// another instance, rather than waiting on a slot that may never free.
	RefuseOverLimit bool `mapstructure:"refuse_over_limit"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if c.MaxConnections < 0 {
		return errors.New("max_connections must be greater than or equal to 0")
	}
	if c.RefuseOverLimit && c.MaxConnections == 0 {
		return errors.New("refuse_over_limit requires max_connections to be greater than 0")
	}
	return nil
}
