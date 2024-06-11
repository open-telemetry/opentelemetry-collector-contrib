// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"errors"
	"net/url"
	"time"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

// Config contains the configuration for the opamp extension. Trying to mirror
// the OpAMP supervisor config for some consistency.
type Config struct {
	Server *OpAMPServer `mapstructure:"server"`

	// InstanceUID is a UUID formatted as a 36 character string in canonical
	// representation. Auto-generated on start if missing.
	InstanceUID string `mapstructure:"instance_uid"`

	// Capabilities contains options to enable a particular OpAMP capability
	Capabilities Capabilities `mapstructure:"capabilities"`

	// Agent descriptions contains options to modify the AgentDescription message
	AgentDescription AgentDescription `mapstructure:"agent_description"`

	// PPID is the process ID of the parent for the collector. If the PPID is specified,
	// the extension will continuously poll for the status of the parent process, and emit a fatal error
	// when the parent process is no longer running.
	// If unspecified, the orphan detection logic does not run.
	PPID int32 `mapstructure:"ppid"`

	// PPIDPollInterval is the time between polling for whether PPID is running.
	PPIDPollInterval time.Duration `mapstructure:"ppid_poll_interval"`
}

type AgentDescription struct {
	// NonIdentifyingAttributes are a map of key-value pairs that may be specified to provide
	// extra information about the agent to the OpAMP server.
	NonIdentifyingAttributes map[string]string `mapstructure:"non_identifying_attributes"`
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

type commonFields struct {
	Endpoint   string                         `mapstructure:"endpoint"`
	TLSSetting configtls.ClientConfig         `mapstructure:"tls,omitempty"`
	Headers    map[string]configopaque.String `mapstructure:"headers,omitempty"`
}

// OpAMPServer contains the OpAMP transport configuration.
type OpAMPServer struct {
	WS   *commonFields `mapstructure:"ws,omitempty"`
	HTTP *commonFields `mapstructure:"http,omitempty"`
}

func (c *commonFields) Scheme() string {
	uri, err := url.ParseRequestURI(c.Endpoint)
	if err != nil {
		return ""
	}
	return uri.Scheme
}

func (c *commonFields) Validate() error {
	if c.Endpoint == "" {
		return errors.New("opamp server endpoint must be provided")
	}
	return nil
}

func (s OpAMPServer) GetClient(logger *zap.Logger) client.OpAMPClient {
	if s.WS != nil {
		return client.NewWebSocket(newLoggerFromZap(logger.With(zap.String("client", "ws"))))
	}
	return client.NewHTTP(newLoggerFromZap(logger.With(zap.String("client", "http"))))
}

func (s OpAMPServer) GetHeaders() map[string]configopaque.String {
	if s.WS != nil {
		return s.WS.Headers
	} else if s.HTTP != nil {
		return s.HTTP.Headers
	}
	return map[string]configopaque.String{}
}

func (s OpAMPServer) GetTLSSetting() configtls.ClientConfig {
	if s.WS != nil {
		return s.WS.TLSSetting
	} else if s.HTTP != nil {
		return s.HTTP.TLSSetting
	}
	return configtls.ClientConfig{}
}

func (s OpAMPServer) GetEndpoint() string {
	if s.WS != nil {
		return s.WS.Endpoint
	} else if s.HTTP != nil {
		return s.HTTP.Endpoint
	}
	return ""
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	switch {
	case cfg.Server.WS == nil && cfg.Server.HTTP == nil:
		return errors.New("opamp server must have at least ws or http set")
	case cfg.Server.WS != nil && cfg.Server.HTTP != nil:
		return errors.New("opamp server must have only ws or http set")
	case cfg.Server.WS != nil:
		if err := cfg.Server.WS.Validate(); err != nil {
			return err
		}
	case cfg.Server.HTTP != nil:
		if err := cfg.Server.HTTP.Validate(); err != nil {
			return err
		}
	}

	if cfg.InstanceUID != "" {
		_, err := parseInstanceIDString(cfg.InstanceUID)
		if err != nil {
			return errors.New("opamp instance_uid is invalid")
		}
	}

	return nil
}
