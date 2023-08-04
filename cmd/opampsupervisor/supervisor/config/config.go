// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"go.opentelemetry.io/collector/config/configtls"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       *OpAMPServer
	Agent        *Agent
	Capabilities *Capabilities `mapstructure:"capabilities"`
}

// Capabilities is the set of capabilities that the Supervisor supports.
type Capabilities struct {
	AcceptsRemoteConfig    *bool `mapstructure:"accepts_remote_config"`
	ReportsEffectiveConfig *bool `mapstructure:"reports_effective_config"`
	ReportsOwnMetrics      *bool `mapstructure:"reports_own_metrics"`
	ReportsHealth          *bool `mapstructure:"reports_health"`
	ReportsRemoteConfig    *bool `mapstructure:"reports_remote_config"`
}

type OpAMPServer struct {
	Endpoint   string
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
}

type Agent struct {
	Executable string
}
