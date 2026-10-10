// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/config/configopaque"
)

// TrapsConfig configures the optional SNMP notification logs receiver.
type TrapsConfig struct {
	ListenAddress    string                `mapstructure:"listen_address"`
	Versions         []string              `mapstructure:"versions"`
	Communities      []configopaque.String `mapstructure:"communities"`
	IncludeCommunity bool                  `mapstructure:"include_community"`
	QueueSize        int                   `mapstructure:"queue_size"`
	V3               *TrapV3Config         `mapstructure:"v3,omitempty"`
	Attributes       map[string]string     `mapstructure:"attributes"`
}

// TrapV3Config configures a USM user for incoming SNMPv3 traps.
type TrapV3Config struct {
	User            string              `mapstructure:"user"`
	SecurityLevel   string              `mapstructure:"security_level"`
	AuthType        string              `mapstructure:"auth_type"`
	AuthPassword    configopaque.String `mapstructure:"auth_password"`
	PrivacyType     string              `mapstructure:"privacy_type"`
	PrivacyPassword configopaque.String `mapstructure:"privacy_password"`
}

func defaultTrapsConfig() *TrapsConfig {
	return &TrapsConfig{
		ListenAddress: "localhost:1620",
		Versions:      []string{"v1", "v2c"},
		QueueSize:     1024,
	}
}

func (cfg *TrapsConfig) validate() error {
	var errs error
	host, port, err := net.SplitHostPort(cfg.ListenAddress)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("traps.listen_address must be host:port: %w", err))
	} else {
		if host == "" {
			errs = errors.Join(errs, errors.New("traps.listen_address must specify a host"))
		}
		p, parseErr := strconv.Atoi(port)
		if parseErr != nil || p < 0 || p > 65535 {
			errs = errors.Join(errs, errors.New("traps.listen_address port must be between 0 and 65535"))
		}
	}
	if cfg.QueueSize < 1 {
		errs = errors.Join(errs, errors.New("traps.queue_size must be positive"))
	}
	if len(cfg.Versions) == 0 {
		errs = errors.Join(errs, errors.New("traps.versions must contain at least one of v1, v2c, v3"))
	}
	seen := make(map[string]bool, len(cfg.Versions))
	for _, version := range cfg.Versions {
		if version != "v1" && version != "v2c" && version != "v3" {
			errs = errors.Join(errs, fmt.Errorf("invalid traps version %q: must be v1, v2c, or v3", version))
		}
		if seen[version] {
			errs = errors.Join(errs, fmt.Errorf("duplicate traps version %q", version))
		}
		seen[version] = true
	}
	if slices.Contains(cfg.Versions, "v3") {
		if cfg.V3 == nil {
			errs = errors.Join(errs, errors.New("traps.v3 must be configured when traps.versions includes v3"))
		} else {
			v3 := cfg.V3
			if len(v3.User) > 32 {
				errs = errors.Join(errs, errors.New("traps.v3.user must contain at most 32 bytes"))
			}
			securityErr := validateSecurity(&Config{
				User: v3.User, SecurityLevel: v3.SecurityLevel,
				AuthType: v3.AuthType, AuthPassword: v3.AuthPassword,
				PrivacyType: v3.PrivacyType, PrivacyPassword: v3.PrivacyPassword,
			})
			if securityErr != nil {
				errs = errors.Join(errs, fmt.Errorf("traps.v3: %w", securityErr))
			}
			if !strings.EqualFold(v3.SecurityLevel, "no_auth_no_priv") && len(v3.AuthPassword) < 8 {
				errs = errors.Join(errs, errors.New("traps.v3.auth_password must contain at least 8 bytes"))
			}
			if strings.EqualFold(v3.SecurityLevel, "auth_priv") && len(v3.PrivacyPassword) < 8 {
				errs = errors.Join(errs, errors.New("traps.v3.privacy_password must contain at least 8 bytes"))
			}
		}
	} else if cfg.V3 != nil {
		errs = errors.Join(errs, errors.New("traps.v3 requires v3 in traps.versions"))
	}
	for key := range cfg.Attributes {
		if key == "" || strings.HasPrefix(key, "snmp.") || strings.HasPrefix(key, "network.peer.") {
			errs = errors.Join(errs, fmt.Errorf("traps.attributes key %q is empty or reserved", key))
		}
	}
	for _, community := range cfg.Communities {
		if community == "" {
			errs = errors.Join(errs, errors.New("traps.communities must not contain an empty community"))
		}
	}
	return errs
}
