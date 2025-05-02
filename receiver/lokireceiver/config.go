// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
)

const (
	// Protocol values.
	protoGRPC = "protocols::grpc"
	protoHTTP = "protocols::http"
)

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC *configgrpc.ServerConfig `mapstructure:"grpc"`
	HTTP *confighttp.ServerConfig `mapstructure:"http"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines configuration for the lokireceiver receiver.
type Config struct {
	// Protocols is the configuration for the supported protocols, currently gRPC and HTTP (Proto and JSON).
	Protocols     `mapstructure:"protocols"`
	KeepTimestamp bool `mapstructure:"use_incoming_timestamp"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var (
	_ component.Config    = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.GRPC == nil && cfg.HTTP == nil {
		return errors.New("must specify at least one protocol when using the Loki receiver")
	}
	return nil
}

// Unmarshal a confmap.Conf into the config struct.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	err := conf.Unmarshal(cfg)
	if err != nil {
		return err
	}

	if !conf.IsSet(protoGRPC) {
		cfg.GRPC = nil
	}

	if !conf.IsSet(protoHTTP) {
		cfg.HTTP = nil
	}

	return nil
}
