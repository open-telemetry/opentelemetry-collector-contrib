// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/http"

	"go.opentelemetry.io/collector/config/configtls"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       *OpAMPServer
	Agent        *Agent
	Capabilities *Capabilities `mapstructure:"capabilities"`
	Storage      *Storage      `mapstructure:"storage"`
}

type Storage struct {
	// Directory is the directory where the Supervisor will store its data.
	Directory string `mapstructure:"directory"`
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
	Executable string
}
