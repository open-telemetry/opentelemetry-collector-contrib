// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
)

const (
	protoHTTP = "protocols::http"
	// Future protocol constants can be added here
	// protoUDP = "protocols::udp"
)

// HTTPConfig defines configuration for Zipkin HTTP receiver.
type HTTPConfig struct {
	ServerConfig confighttp.ServerConfig `mapstructure:",squash"`
}

// UDPConfig defines configuration for a potential future Zipkin UDP receiver.
// type UDPConfig struct {
// 	Endpoint string `mapstructure:"endpoint"`
// 	// Add other UDP-specific configuration options here
// }

// Protocols holds the configuration for all supported protocols.
type Protocols struct {
	HTTP *HTTPConfig `mapstructure:"http"`
	// Future protocols can be added here
	// UDP *UDPConfig `mapstructure:"udp"`
}

// Config defines configuration for Zipkin receiver.
type Config struct {
	// Protocols is the configuration for the supported protocols, currently only HTTP.
	Protocols `mapstructure:"protocols"`

	// Deprecated: [v0.128.0] Use protocols.http instead.
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// If enabled the zipkin receiver will attempt to parse string tags/binary annotations into int/bool/float.
	// Disabled by default
	ParseStringTags bool `mapstructure:"parse_string_tags"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	// Check if legacy configuration is used
	if cfg.Endpoint != "" {
		// Legacy configuration is used, no need to validate protocols
		return nil
	}

	// Check if protocols configuration is used
	if cfg.Protocols.HTTP == nil {
		// Currently, only HTTP protocol is supported
		// When more protocols are added, this validation can be updated
		// to check if at least one protocol is configured
		return errors.New("must specify at least one protocol when using protocols configuration")
	}

	return nil
}

// Unmarshal is a custom unmarshaler for Config
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	// First, unmarshal the standard fields
	err := conf.Unmarshal(cfg)
	if err != nil {
		return err
	}

	// Check if any protocol configuration is used
	// Currently only checking for HTTP, but can be expanded for future protocols
	if !conf.IsSet(protoHTTP) {
		// No protocols section, using legacy configuration
		return nil
	}

	// If protocols section exists, clear the legacy endpoint to avoid confusion
	if cfg.Endpoint != "" && conf.IsSet(protoHTTP) {
		// Both legacy and new configuration used, log a warning
		fmt.Println("Warning: Both legacy endpoint and protocols.http configuration found. Using protocols.http configuration.")
		// Clear the legacy endpoint
		cfg.Endpoint = ""
	}

	return nil
}
