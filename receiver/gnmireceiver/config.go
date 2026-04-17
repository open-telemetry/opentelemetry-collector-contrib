// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnmireceiver

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
)

// Config defines the configuration for the gNMI receiver.
type Config struct {
	// Targets is the list of gNMI devices to subscribe to.
	// Each target can have its own specific connectivity and subscription settings.
	Targets []TargetConfig `mapstructure:"targets"`

	// ModulePaths is an optional list of directories containing .yang files.
	// Used for level-1 Counter/Gauge resolution — same as yanggrpcreceiver.
	// If empty, only structural analysis (level 2) is used.
	ModulePaths []string `mapstructure:"module_paths"`
}

// TargetConfig defines the connectivity and authentication for a single gNMI device.
type TargetConfig struct {
	// Address is the host:port of the gNMI server (e.g., "10.0.0.1:57400" for Cisco).
	Address string `mapstructure:"address"`

	// Username and Password for gRPC basic authentication.
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`

	// Encoding is the gNMI encoding to request (e.g., "proto", "json", "json_ietf").
	// Cisco often uses "proto", while Arista supports "json" via port 6030.
	Encoding string `mapstructure:"encoding"`

	// Redial is the delay before attempting to reconnect after a session failure.
	Redial time.Duration `mapstructure:"redial"`

	// Timeout is the maximum time allowed for a single gRPC connection attempt.
	Timeout time.Duration `mapstructure:"timeout"`

	// TLS configuration for the secure gRPC connection.
	TLSSetting configtls.ClientConfig `mapstructure:"tls"`

	// Subscriptions is the list of YANG paths to monitor on this specific device.
	Subscriptions []SubscriptionConfig `mapstructure:"subscriptions"`
}

// SubscriptionConfig defines the parameters for a specific gNMI path subscription.
type SubscriptionConfig struct {
	// Name is a descriptive name for the metric (used as a prefix or identifier).
	Name string `mapstructure:"name"`

	// Origin is the YANG model origin (e.g., "openconfig", "cisco-ios-xe", "eos-native").
	Origin string `mapstructure:"origin"`

	// Path is the gNMI path (e.g., "/interfaces/interface/state/counters").
	Path string `mapstructure:"path"`

	// SubscriptionMode defines how data is sent: "sample", "on_change", or "target_defined".
	SubscriptionMode string `mapstructure:"subscription_mode"`

	// SampleInterval is the interval for "sample" mode.
	SampleInterval time.Duration `mapstructure:"sample_interval"`

	// HeartbeatInterval forces an update at this interval even if no change occurs (useful for Cisco/Arista status).
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`

	// SuppressRedundant avoids sending identical data points to the collector.
	SuppressRedundant bool `mapstructure:"suppress_redundant"`
}

// Validate checks if the receiver configuration is valid and follows OTel standards.
func (cfg *Config) Validate() error {
	if len(cfg.Targets) == 0 {
		return errors.New("at least one target must be specified")
	}

	for i, target := range cfg.Targets {
		// Basic connectivity check
		if target.Address == "" {
			return fmt.Errorf("target[%d]: address is required (e.g., host:port)", i)
		}

		// Validate gNMI encoding options
		switch target.Encoding {
		case "", "proto", "json", "json_ietf":
			// These are standard gNMI encodings
		default:
			return fmt.Errorf("target[%d]: invalid encoding %q. Supported: 'proto', 'json', 'json_ietf'", i, target.Encoding)
		}

		if len(target.Subscriptions) == 0 {
			return fmt.Errorf("target[%d]: at least one subscription must be defined", i)
		}

		if target.Redial > 0 && target.Redial < 1*time.Second {
			return fmt.Errorf("target[%d]: redial interval must be at least 1s to avoid CPU spikes", i)
		}

		// Validate each subscription's logic
		for j, sub := range target.Subscriptions {
			if sub.Path == "" {
				return fmt.Errorf("target[%d].subscription[%d]: path is required", i, j)
			}

			switch sub.SubscriptionMode {
			case "sample":
				if sub.SampleInterval <= 0 {
					return fmt.Errorf("target[%d].subscription[%d]: sample_interval must be > 0 for 'sample' mode", i, j)
				}
			case "on_change", "target_defined":
				// These modes are valid and don't strictly require a sample interval
			case "":
				return fmt.Errorf("target[%d].subscription[%d]: subscription_mode is required", i, j)
			default:
				return fmt.Errorf("target[%d].subscription[%d]: invalid mode %q", i, j, sub.SubscriptionMode)
			}
		}
	}

	return nil
}

// Ensure Config implements the component.Config interface
var _ component.Config = (*Config)(nil)
