// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisfleettrackerextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

// Config configures the redis_fleet_tracker extension.
type Config struct {
	// Endpoint is the Redis server address as host:port.
	Endpoint string `mapstructure:"endpoint"`
	// Username authenticates to Redis when ACLs are in use (Redis 6 and
	// later). Optional; leave unset for password-only auth, which Redis
	// treats as the default user.
	Username string `mapstructure:"username"`
	// Password authenticates to Redis. Optional.
	Password configopaque.String `mapstructure:"password"`
	// DB is the Redis logical database number.
	DB int `mapstructure:"db"`
	// TLS configures the connection to Redis. Disabled (plain TCP) by
	// default; set tls::insecure: false to opt in.
	TLS configtls.ClientConfig `mapstructure:"tls"`
	// Fleet names the group of instances this extension counts members for.
	// Defaults to the extension's own component ID, so instances sharing an
	// identical config count together automatically.
	Fleet string `mapstructure:"fleet"`
	// KeyPrefix namespaces every key this extension writes.
	KeyPrefix string `mapstructure:"key_prefix"`
	// HeartbeatInterval is how often this instance refreshes its membership.
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	// TTL is how long a member is considered live after its last heartbeat.
	TTL time.Duration `mapstructure:"ttl"`
}

func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if cfg.HeartbeatInterval <= 0 {
		return errors.New("heartbeat_interval must be greater than zero")
	}
	if cfg.TTL <= cfg.HeartbeatInterval {
		return errors.New("ttl must be greater than heartbeat_interval")
	}
	return nil
}
