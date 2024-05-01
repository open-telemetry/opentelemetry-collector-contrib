// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/config/configtls"
	semconv "go.opentelemetry.io/collector/semconv/v1.21.0"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       *OpAMPServer
	Agent        *Agent
	Capabilities *Capabilities `mapstructure:"capabilities"`
	Storage      *Storage      `mapstructure:"storage"`
}

func (s *Supervisor) Validate() error {
	if s.Agent != nil {
		return s.Agent.Validate()
	}

	return nil
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
	Executable  string
	Description AgentDescription `mapstructure:"description"`
}

func (a *Agent) Validate() error {
	return a.Description.Validate()
}

type AgentDescription struct {
	IdentifyingAttributes    map[string]string `mapstructure:"identifying_attributes"`
	NonIdentifyingAttributes map[string]string `mapstructure:"non_identifying_attributes"`
}

func (a *AgentDescription) Validate() error {
	for k := range a.IdentifyingAttributes {
		// Don't allow overriding the instance ID attribute
		if k == semconv.AttributeServiceInstanceID {
			return fmt.Errorf("cannot override identifying attribute %q", semconv.AttributeServiceInstanceID)
		}
	}
	return nil
}
