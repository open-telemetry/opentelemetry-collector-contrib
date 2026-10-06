// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver"

import (
	"errors"
	"strings"
)

// Config is the receiver config in the collector config.yaml.
type Config struct {
	// ListenAddress is the UDP host:port. Default 0.0.0.0:1620.
	ListenAddress string `mapstructure:"listen_address"`
	// Communities is a v1/v2c allowlist. Empty accepts every community.
	Communities []string `mapstructure:"communities"`
	// IncludeCommunity puts the community string on the log record.
	IncludeCommunity bool `mapstructure:"include_community"`
	// DropUndefined drops traps whose trap OID did not resolve via MIBs.
	DropUndefined bool `mapstructure:"drop_undefined"`
	// MIBPaths are directories of SMI/MIB files for optional gosmi enrichment.
	MIBPaths []string `mapstructure:"mib_paths"`
	// Attributes are extra log attributes stamped on every record.
	Attributes map[string]string `mapstructure:"attributes"`
	// V3 is an optional USM user. When set, the listener is v3-only.
	V3 *V3Config `mapstructure:"v3"`
}

// V3Config is a single USM user for SNMPv3 traps/informs.
type V3Config struct {
	User          string `mapstructure:"user"`
	SecurityLevel string `mapstructure:"security_level"`
	AuthProtocol  string `mapstructure:"auth_protocol"`
	AuthPassword  string `mapstructure:"auth_password"`
	PrivProtocol  string `mapstructure:"priv_protocol"`
	PrivPassword  string `mapstructure:"priv_password"`
}

// Validate checks the configuration.
func (cfg *Config) Validate() error {
	if strings.TrimSpace(cfg.ListenAddress) == "" {
		return errors.New("listen_address must not be empty")
	}
	if cfg.V3 != nil && strings.TrimSpace(cfg.V3.User) == "" {
		return errors.New("v3.user is required when the v3 block is set")
	}
	return nil
}
