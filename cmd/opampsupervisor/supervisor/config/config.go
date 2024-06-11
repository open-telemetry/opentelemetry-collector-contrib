// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/config/configtls"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       OpAMPServer
	Agent        Agent
	Capabilities Capabilities `mapstructure:"capabilities"`
	Storage      Storage      `mapstructure:"storage"`
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

func (c Capabilities) SupportedCapabilities() protobufs.AgentCapabilities {
	supportedCapabilities := protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus

	if c.ReportsEffectiveConfig {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig
	}

	if c.ReportsHealth {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth
	}

	if c.ReportsOwnMetrics {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics
	}

	if c.AcceptsRemoteConfig {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig
	}

	if c.ReportsRemoteConfig {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig
	}

	if c.AcceptsRestartCommand {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsRestartCommand
	}

	if c.AcceptsOpAMPConnectionSettings {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsOpAMPConnectionSettings
	}

	return supportedCapabilities
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
	Executable              string
	OrphanDetectionInterval time.Duration    `mapstructure:"orphan_detection_interval"`
	Description             AgentDescription `mapstructure:"description"`
}

func (a Agent) Validate() error {
	if a.OrphanDetectionInterval <= 0 {
		return errors.New("agent::orphan_detection_interval must be positive")
	}

	if a.Executable == "" {
		return errors.New("agent::executable must be specified")
	}

	_, err := os.Stat(a.Executable)
	if err != nil {
		return fmt.Errorf("could not stat agent::executable path: %w", err)
	}

	return nil
}

type AgentDescription struct {
	IdentifyingAttributes    map[string]string `mapstructure:"identifying_attributes"`
	NonIdentifyingAttributes map[string]string `mapstructure:"non_identifying_attributes"`
}

// DefaultSupervisor returns the default supervisor config
func DefaultSupervisor() Supervisor {
	defaultStorageDir := "/var/lib/otelcol/supervisor"
	if runtime.GOOS == "windows" {
		// Windows default is "%ProgramData%\Otelcol\Supervisor"
		// If the ProgramData environment variable is not set,
		// it falls back to C:\ProgramData
		programDataDir := os.Getenv("ProgramData")
		if programDataDir == "" {
			programDataDir = `C:\ProgramData`
		}

		defaultStorageDir = filepath.Join(programDataDir, "Otelcol", "Supervisor")
	}

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
		Storage: Storage{
			Directory: defaultStorageDir,
		},
		Agent: Agent{
			OrphanDetectionInterval: 5 * time.Second,
		},
	}
}
