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
	Arrow ArrowConfig             `mapstructure:"arrow"`
}

// ArrowConfig support configuring the Arrow receiver.
type ArrowConfig struct {
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
var _ component.ConfigValidator = (*ArrowConfig)(nil)

func (cfg *ArrowConfig) Validate() error {
	if err := cfg.Zstd.Validate(); err != nil {
		return fmt.Errorf("zstd decoder: invalid configuration: %w", err)
	}
	return nil
}
