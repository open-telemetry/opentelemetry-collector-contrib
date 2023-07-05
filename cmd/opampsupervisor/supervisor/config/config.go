// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       *OpAMPServer
	Agent        *Agent
	Capabilities *Capabilities `mapstructure:"capabilities"`
}

// Capabilities is the set of capabilities that the Supervisor supports.
// TODO: Support more capabilities
type Capabilities struct {
	AcceptsRemoteConfig            *bool `mapstructure:"accepts_remote_config"`
	AcceptsOpAMPConnectionSettings *bool `mapstructure:"accepts_opamp_connection_settings"`
	AcceptsRestartCommand          *bool `mapstructure:"accepts_restart_command"`
	ReportsStatus                  *bool `mapstructure:"reports_status"`
	ReportsEffectiveConfig         *bool `mapstructure:"reports_effective_config"`
	ReportsOwnMetrics              *bool `mapstructure:"reports_own_metrics"`
	ReportsHealth                  *bool `mapstructure:"reports_health"`
	ReportsRemoteConfig            *bool `mapstructure:"reports_remote_config"`
}

type OpAMPServer struct {
	Endpoint string
}

type Agent struct {
	Executable string
}
