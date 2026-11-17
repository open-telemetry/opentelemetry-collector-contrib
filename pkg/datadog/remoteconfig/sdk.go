// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/remoteconfig"

import (
	"errors"
	"net"

	"github.com/DataDog/datadog-agent/pkg/config/model"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"
)

// SDKConfig contains settings resolved by the extension, including its credentials.
type SDKConfig struct {
	Endpoint                 string
	HTTP                     *confighttp.ServerConfig
	RemoteConfig             *Config
	InstrumentationTelemetry InstrumentationTelemetryConfig
	API                      APIConfig
	Backend                  confighttp.ClientConfig
}

func (c *SDKConfig) Enabled() bool {
	return c.InstrumentationTelemetry.Enabled || c.RemoteConfig != nil && c.RemoteConfig.Enabled
}

func (c *SDKConfig) resolved() *Config {
	config := DefaultConfig()
	if c.RemoteConfig != nil {
		*config = *c.RemoteConfig
	}
	config.API, config.Backend = c.API, c.Backend
	if c.HTTP != nil {
		config.ServerConfig = *c.HTTP
	}
	if c.Endpoint != "" {
		config.ServerConfig.NetAddr.Endpoint = c.Endpoint
	}
	return config
}

func ValidateSDKConfig(c *SDKConfig) error {
	if !c.Enabled() {
		return nil
	}
	config := c.resolved()
	if config.API.Key == "" || !sitePattern.MatchString(config.API.Site) {
		return errors.New("SDK forwarding requires a Collector api.key and a valid api.site")
	}
	backend := config.Backend
	if backend.TLS.Insecure || backend.TLS.InsecureSkipVerify || backend.Auth.HasValue() || len(backend.Headers) > 0 || len(backend.Middlewares) > 0 || backend.Compression.IsCompressed() {
		return errors.New("SDK forwarding uses api.key for backend authentication and requires verified backend TLS without custom headers, middleware or compression")
	}
	if config.ServerConfig.MaxRequestBodySize <= 0 {
		return errors.New("SDK listener max_request_body_size must be positive")
	}
	if _, _, err := net.SplitHostPort(config.ServerConfig.NetAddr.Endpoint); err != nil {
		return errors.New("sdk_endpoint must be a host:port address")
	}
	if config.Enabled {
		if err := ValidateConfig(config); err != nil {
			return err
		}
	}
	if c.InstrumentationTelemetry.Enabled {
		return c.InstrumentationTelemetry.validate(config.API.Site)
	}
	return nil
}

// NewSDKServer serves independently enabled RC and SDK telemetry on one listener.
func NewSDKServer(set extension.Settings, c *SDKConfig, cfg model.Reader) (extension.Extension, error) {
	if err := ValidateSDKConfig(c); err != nil {
		return nil, err
	}
	if !c.Enabled() {
		return nil, errors.New("at least one SDK capability must be enabled")
	}
	config := c.resolved()
	var server *remoteConfigExtension
	if config.Enabled {
		rc, err := New(set, config, cfg)
		if err != nil {
			return nil, err
		}
		server = rc.(*remoteConfigExtension)
	} else {
		server = &remoteConfigExtension{config: config, settings: set, telemetryOnly: true}
	}
	server.maxRCRequestBytes = config.ServerConfig.MaxRequestBodySize
	if c.InstrumentationTelemetry.Enabled {
		server.telemetry = newInstrumentationTelemetry(c.InstrumentationTelemetry, config.API, config.Backend)
		config.ServerConfig.MaxRequestBodySize = max(config.ServerConfig.MaxRequestBodySize, c.InstrumentationTelemetry.MaxRequestBytes)
	}
	return server, nil
}
