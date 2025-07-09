// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
)

const (
	// The config field id to load the protocol map from
	protocolsFieldName = "protocols"
)

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC *configgrpc.ServerConfig `mapstructure:"grpc"`
	HTTP *confighttp.ServerConfig `mapstructure:"http"`
}

// Config defines configuration for skywalking receiver.
type Config struct {
	Protocols `mapstructure:"protocols"`
}

var (
	_ component.Config    = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.GRPC == nil && cfg.HTTP == nil {
		return errors.New("must specify at least one protocol when using the Skywalking receiver")
	}

	if cfg.GRPC != nil {
		var err error
		if _, err = extractPortFromEndpoint(cfg.GRPC.NetAddr.Endpoint); err != nil {
			return fmt.Errorf("unable to extract port for the gRPC endpoint: %w", err)
		}
	}

	if cfg.HTTP != nil {
		if _, err := extractPortFromEndpoint(cfg.HTTP.Endpoint); err != nil {
			return fmt.Errorf("unable to extract port for the HTTP endpoint: %w", err)
		}
	}

	return nil
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil || len(componentParser.AllKeys()) == 0 {
		return errors.New("empty config for Skywalking receiver")
	}

	// UnmarshalExact will not set struct properties to nil even if no key is provided,
	// so set the protocol structs to nil where the keys were omitted.
	err := componentParser.Unmarshal(cfg)
	if err != nil {
		return err
	}

	protocols, err := componentParser.Sub(protocolsFieldName)
	if err != nil {
		return err
	}

	if !protocols.IsSet(protoGRPC) {
		cfg.GRPC = nil
	}

	if !protocols.IsSet(protoHTTP) {
		cfg.HTTP = nil
	}

	return nil
}
