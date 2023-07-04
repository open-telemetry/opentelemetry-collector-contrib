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
type Capabilities struct {
	AcceptsRemoteConfig            *bool `mapstructure:"accepts_remote_config"`
	AcceptsPackages                *bool `mapstructure:"accepts_packages"`
	AcceptsOpAMPConnectionSettings *bool `mapstructure:"accepts_opamp_connection_settings"`
	AcceptsOtherConnectionSettings *bool `mapstructure:"accepts_other_connection_settings"`
	AcceptsRestartCommand          *bool `mapstructure:"accepts_restart_command"`
	ReportsStatus                  *bool `mapstructure:"reports_status"`
	ReportsEffectiveConfig         *bool `mapstructure:"reports_effective_config"`
	ReportsPackageStatuses         *bool `mapstructure:"reports_package_statuses"`
	ReportsOwnTraces               *bool `mapstructure:"reports_own_traces"`
	ReportsOwnMetrics              *bool `mapstructure:"reports_own_metrics"`
	ReportsOwnLogs                 *bool `mapstructure:"reports_own_logs"`
	ReportsHealth                  *bool `mapstructure:"reports_health"`
	ReportsRemoteConfig            *bool `mapstructure:"reports_remote_config"`
}

type OpAMPServer struct {
	Endpoint string
}

type Agent struct {
	Executable string
}
