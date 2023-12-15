// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"errors"

	"github.com/oklog/ulid/v2"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

// Config contains the configuration for the opamp extension. Trying to mirror
// the OpAMP supervisor config for some consistency.
type Config struct {
	Server *OpAMPServer `mapstructure:"server"`

	// InstanceUID is a ULID formatted as a 26 character string in canonical
	// representation. Auto-generated on start if missing.
	InstanceUID string `mapstructure:"instance_uid"`

	// Capabilities contains options to enable a particular OpAMP capability
	Capabilities Capabilities `mapstructure:"capabilities"`
}

type Capabilities struct {
	// ReportsEffectiveConfig enables the OpAMP ReportsEffectiveConfig Capability. (default: true)
	ReportsEffectiveConfig bool `mapstructure:"reports_effective_config"`
}

func (caps Capabilities) toAgentCapabilities() protobufs.AgentCapabilities {
	// All Agents MUST report status.
	agentCapabilities := protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus

	if caps.ReportsEffectiveConfig {
		agentCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig
	}

	return agentCapabilities
}

// OpAMPServer contains the OpAMP transport configuration.
type OpAMPServer struct {
	WS *OpAMPWebsocket `mapstructure:"ws"`
}

// OpAMPWebsocket contains the OpAMP websocket transport configuration.
type OpAMPWebsocket struct {
	Endpoint   string                         `mapstructure:"endpoint"`
	TLSSetting configtls.TLSClientSetting     `mapstructure:"tls,omitempty"`
	Headers    map[string]configopaque.String `mapstructure:"headers,omitempty"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Server.WS.Endpoint == "" {
		return errors.New("opamp server websocket endpoint must be provided")
	}

	if cfg.InstanceUID != "" {
		_, err := ulid.ParseStrict(cfg.InstanceUID)
		if err != nil {
			return errors.New("opamp instance_uid is invalid")
		}
	}

	return nil
}
