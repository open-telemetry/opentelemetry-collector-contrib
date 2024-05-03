// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"go.opentelemetry.io/collector/config/configtls"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       OpAMPServer
	Agent        Agent
	Capabilities Capabilities `mapstructure:"capabilities"`
	Storage      *Storage     `mapstructure:"storage"`
}

func (s Supervisor) Validate() error {
	if err := s.Server.Validate(); err != nil {
		return err
	}

	if err := s.Agent.Validate(); err != nil {
		return err
	}

	return nil
}

type Storage struct {
	// Directory is the directory where the Supervisor will store its data.
	Directory string `mapstructure:"directory"`
}

// Capabilities is the set of capabilities that the Supervisor supports.
type Capabilities struct {
	AcceptsRemoteConfig            bool `mapstructure:"accepts_remote_config"`
	AcceptsRestartCommand          bool `mapstructure:"accepts_restart_command"`
	AcceptsOpAMPConnectionSettings bool `mapstructure:"accepts_opamp_connection_settings"`
	ReportsEffectiveConfig         bool `mapstructure:"reports_effective_config"`
	ReportsOwnMetrics              bool `mapstructure:"reports_own_metrics"`
	ReportsHealth                  bool `mapstructure:"reports_health"`
	ReportsRemoteConfig            bool `mapstructure:"reports_remote_config"`
}

type OpAMPServer struct {
	Endpoint   string
	Headers    http.Header
	TLSSetting configtls.ClientConfig `mapstructure:"tls,omitempty"`
}

func (o OpAMPServer) Validate() error {
	if o.Endpoint == "" {
		return errors.New("server::endpoint must be specified")
	}

	url, err := url.Parse(o.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL for server::endpoint: %w", err)
	}

	switch url.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf(`invalid scheme %q for server::endpoint, must be one of "http", "https", "ws", or "wss"`, url.Scheme)
	}

	err = o.TLSSetting.Validate()
	if err != nil {
		return fmt.Errorf("invalid server::tls settings: %w", err)
	}

	return nil
}

type Agent struct {
	Executable string
}

func (a Agent) Validate() error {
	if a.Executable == "" {
		return errors.New("agent::executable must be specified")
	}

	f, err := os.Stat(a.Executable)
	if err != nil {
		return fmt.Errorf("could not stat agent::executable path: %w", err)
	}

	if f.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("agent::executable does not have executable bit set")
	}

	return nil
}

// DefaultSupervisor returns the default supervisor config
func DefaultSupervisor() Supervisor {
	return Supervisor{
		Capabilities: Capabilities{
			AcceptsRemoteConfig:            false,
			AcceptsRestartCommand:          false,
			AcceptsOpAMPConnectionSettings: false,
			ReportsEffectiveConfig:         true,
			ReportsOwnMetrics:              true,
			ReportsHealth:                  true,
			ReportsRemoteConfig:            false,
		},
	}
}
