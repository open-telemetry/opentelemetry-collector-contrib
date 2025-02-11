// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/featuregate"
)

var disallowHTTPDefaultProtocol = featuregate.GlobalRegistry().MustRegister(
	"zipkinreceiver.httpDefaultProtocol.disallow",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, usage of the default http configuration is disallowed"),
	featuregate.WithRegisterFromVersion("v0.114.0"),
)

const (
	// Protocol values.
	protoHTTP = "protocols::http"
)

// Config defines configuration for Zipkin receiver.
type Config struct {
	// Configures the receiver server protocol.
	//
	// Deprecated: Parameter exists for historical compatibility
	// and should not be used. To set the server configurations,
	// use the Protocols parameter instead.
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	// If enabled the zipkin receiver will attempt to parse string tags/binary annotations into int/bool/float.
	// Disabled by default
	ParseStringTags bool `mapstructure:"parse_string_tags"`
	// Protocols define the supported protocols for the receiver
	Protocols ProtocolTypes `mapstructure:"protocols"`
}

type ProtocolTypes struct {
	HTTP confighttp.ServerConfig `mapstructure:"http"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if isServerConfigDefined(cfg.ServerConfig) {
		if disallowHTTPDefaultProtocol.IsEnabled() {
			return fmt.Errorf("the server config setup is disabled, please use protocols::http or enable it by setting zipkinreceiver.httpDefaultProtocol.disallow feaure gate to false")
		}
		if isServerConfigDefined(cfg.Protocols.HTTP) {
			return fmt.Errorf("cannot use protocols::http together with default server config setup")
		}
	}

	return nil
}

// Unmarshal a confmap.Conf into the config struct.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	// first load the config normally
	err := conf.Unmarshal(cfg)
	if err != nil {
		return err
	}

	if !conf.IsSet(protoHTTP) {
		cfg.Protocols.HTTP = cfg.ServerConfig
		cfg.ServerConfig = confighttp.ServerConfig{}
	}

	return nil
}

// IsServerConfigDefined checks if the ServerConfig is defined by the user
func isServerConfigDefined(cfg confighttp.ServerConfig) bool {
	return cfg.Endpoint != "" ||
		cfg.TLSSetting != nil ||
		cfg.CORS != nil ||
		cfg.Auth != nil ||
		cfg.MaxRequestBodySize != 0 ||
		cfg.IncludeMetadata ||
		len(cfg.ResponseHeaders) != 0 ||
		len(cfg.CompressionAlgorithms) != 0 ||
		cfg.ReadHeaderTimeout != 0 ||
		cfg.ReadTimeout != 0 ||
		cfg.WriteTimeout != 0 ||
		cfg.IdleTimeout != 0
}
