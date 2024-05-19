// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/config/configtls"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       *OpAMPServer
	Agent        *Agent
	Capabilities *Capabilities `mapstructure:"capabilities"`
	Storage      Storage       `mapstructure:"storage"`
}

type Storage struct {
	// Directory is the directory where the Supervisor will store its data.
	Directory string `mapstructure:"directory"`
}

// DirectoryOrDefault returns the configured storage directory if it was configured,
// otherwise it returns the system default.
func (s Storage) DirectoryOrDefault() string {
	if s.Directory == "" {
		switch runtime.GOOS {
		case "windows":
			// Windows default is "%ProgramData%\Otelcol\Supervisor"
			// If the ProgramData environment variable is not set,
			// it falls back to C:\ProgramData
			programDataDir := os.Getenv("ProgramData")
			if programDataDir == "" {
				programDataDir = `C:\ProgramData`
			}
			return filepath.Join(programDataDir, "Otelcol", "Supervisor")
		default:
			// Default for non-windows systems
			return "/var/lib/otelcol/supervisor"
		}
	}

	return s.Directory
}

// Capabilities is the set of capabilities that the Supervisor supports.
type Capabilities struct {
	AcceptsRemoteConfig            *bool `mapstructure:"accepts_remote_config"`
	AcceptsRestartCommand          *bool `mapstructure:"accepts_restart_command"`
	AcceptsOpAMPConnectionSettings *bool `mapstructure:"accepts_opamp_connection_settings"`
	ReportsEffectiveConfig         *bool `mapstructure:"reports_effective_config"`
	ReportsOwnMetrics              *bool `mapstructure:"reports_own_metrics"`
	ReportsHealth                  *bool `mapstructure:"reports_health"`
	ReportsRemoteConfig            *bool `mapstructure:"reports_remote_config"`
}

type OpAMPServer struct {
	Endpoint   string
	Headers    http.Header
	TLSSetting configtls.ClientConfig `mapstructure:"tls,omitempty"`
}

type Agent struct {
	Executable              string
	OrphanDetectionInterval time.Duration    `mapstructure:"orphan_detection_interval"`
	Description             AgentDescription `mapstructure:"description"`
}

type AgentDescription struct {
	IdentifyingAttributes    map[string]string `mapstructure:"identifying_attributes"`
	NonIdentifyingAttributes map[string]string `mapstructure:"non_identifying_attributes"`
}
