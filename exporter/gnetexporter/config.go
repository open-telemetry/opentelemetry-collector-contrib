// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/gnetexporter"

import (
	"errors"
	"time"
)

// Config holds configuration for the gnet exporter.
type Config struct {
	// Endpoint is the TCP address of the gnet receiver.
	// Example: "otel-gateway:4320"
	Endpoint string `mapstructure:"endpoint"`

	// DialTimeout is the timeout for establishing the initial TCP connection.
	// Default: 5s
	DialTimeout time.Duration `mapstructure:"dial_timeout"`

	// WriteTimeout is the per-frame write deadline.
	// Default: 10s
	WriteTimeout time.Duration `mapstructure:"write_timeout"`

	// ReconnectDelay is the backoff delay between reconnect attempts.
	// Default: 1s
	ReconnectDelay time.Duration `mapstructure:"reconnect_delay"`

	// MaxReconnectDelay caps the exponential backoff.
	// Default: 30s
	MaxReconnectDelay time.Duration `mapstructure:"max_reconnect_delay"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint must not be empty")
	}
	if c.DialTimeout <= 0 {
		return errors.New("dial_timeout must be a positive duration")
	}
	if c.WriteTimeout <= 0 {
		return errors.New("write_timeout must be a positive duration")
	}
	return nil
}
