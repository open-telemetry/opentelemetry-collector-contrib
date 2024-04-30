// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"fmt"

	"github.com/open-telemetry/otel-arrow/collector/compression/zstd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
)

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC  configgrpc.ServerConfig `mapstructure:"grpc"`
	Arrow ArrowSettings           `mapstructure:"arrow"`
}

// ArrowSettings support configuring the Arrow receiver.
type ArrowSettings struct {
	// MemoryLimitMiB is the size of a shared memory region used
	// by all Arrow streams, in MiB.  When too much load is
	// passing through, they will see ResourceExhausted errors.
	MemoryLimitMiB uint64 `mapstructure:"memory_limit_mib"`

	// Zstd settings apply to OTel-Arrow use of gRPC specifically.
	Zstd zstd.DecoderConfig `mapstructure:"zstd"`
}

// Config defines configuration for OTel Arrow receiver.
type Config struct {
	// Protocols is the configuration for gRPC and Arrow.
	Protocols `mapstructure:"protocols"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if err := cfg.Arrow.Validate(); err != nil {
		return err
	}
	return nil
}

func (cfg *ArrowSettings) Validate() error {
	if err := cfg.Zstd.Validate(); err != nil {
		return fmt.Errorf("zstd decoder: invalid configuration: %w", err)
	}
	return nil
}
