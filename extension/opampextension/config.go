// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

// Default value for HTTP client's polling interval, set to 30 seconds in
// accordance with the OpAMP spec.
const httpPollingIntervalDefault = 30 * time.Second

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
	// IncludeResourceAttributes determines whether the agent should copy its resource attributes
	// to the non identifying attributes. (default: false)
	IncludeResourceAttributes bool `mapstructure:"include_resource_attributes"`
}

type Capabilities struct {
	// ReportsEffectiveConfig enables the OpAMP ReportsEffectiveConfig Capability. (default: true)
	ReportsEffectiveConfig bool `mapstructure:"reports_effective_config"`
	// ReportsHealth enables the OpAMP ReportsHealth Capability. (default: true)
	ReportsHealth bool `mapstructure:"reports_health"`
	// ReportsAvailableComponents enables the OpAMP ReportsAvailableComponents Capability (default: true)
	ReportsAvailableComponents bool `mapstructure:"reports_available_components"`
}

func (caps Capabilities) toAgentCapabilities() protobufs.AgentCapabilities {
	// All Agents MUST report status.
	agentCapabilities := protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus

	if caps.ReportsEffectiveConfig {
		agentCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig
	}
	if caps.ReportsHealth {
		agentCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth
	}

	if caps.ReportsAvailableComponents {
		agentCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents
	}

	return agentCapabilities
}

type commonFields struct {
	Endpoint string                         `mapstructure:"endpoint"`
	TLS      configtls.ClientConfig         `mapstructure:"tls,omitempty"`
	Headers  map[string]configopaque.String `mapstructure:"headers,omitempty"`
	Auth     component.ID                   `mapstructure:"auth,omitempty"`
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

type httpFields struct {
	commonFields `mapstructure:",squash"`

	PollingInterval time.Duration `mapstructure:"polling_interval"`
}

func (h *httpFields) Validate() error {
	if err := h.commonFields.Validate(); err != nil {
		return err
	}

	if h.PollingInterval < 0 {
		return errors.New("polling interval must be 0 or greater")
	}

	return nil
}

// OpAMPServer contains the OpAMP transport configuration.
type OpAMPServer struct {
	WS   *commonFields `mapstructure:"ws,omitempty"`
	HTTP *httpFields   `mapstructure:"http,omitempty"`
}

func (s OpAMPServer) GetClient(logger *zap.Logger) client.OpAMPClient {
	if s.WS != nil {
		return client.NewWebSocket(newLoggerFromZap(logger.With(zap.String("client", "ws"))))
	}

	httpClient := client.NewHTTP(newLoggerFromZap(logger.With(zap.String("client", "http"))))
	httpClient.SetPollingInterval(s.GetPollingInterval())
	return httpClient
}

func (s OpAMPServer) GetHeaders() map[string]configopaque.String {
	if s.WS != nil {
		return s.WS.Headers
	} else if s.HTTP != nil {
		return s.HTTP.Headers
	}
	return map[string]configopaque.String{}
}

// GetTLSConfig returns a TLS config if the endpoint is secure (wss or https)
func (s OpAMPServer) GetTLSConfig(ctx context.Context) (*tls.Config, error) {
	parsedURL, err := url.Parse(s.GetEndpoint())
	if err != nil {
		return nil, fmt.Errorf("parse server endpoint: %w", err)
	}

	if parsedURL.Scheme != "wss" && parsedURL.Scheme != "https" {
		return nil, nil
	}

	return s.getTLS().LoadTLSConfig(ctx)
}

func (s OpAMPServer) getTLS() configtls.ClientConfig {
	if s.WS != nil {
		return s.WS.TLS
	} else if s.HTTP != nil {
		return s.HTTP.TLS
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

func (s OpAMPServer) GetAuthExtensionID() component.ID {
	if s.WS != nil {
		return s.WS.Auth
	} else if s.HTTP != nil {
		return s.HTTP.Auth
	}

	var emptyComponentID component.ID
	return emptyComponentID
}

func (s OpAMPServer) GetPollingInterval() time.Duration {
	if s.HTTP != nil && s.HTTP.PollingInterval > 0 {
		return s.HTTP.PollingInterval
	}

	return httpPollingIntervalDefault
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
