// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"
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

	// AdmissionLimitMiB limits the number of requests that are received by the stream based on
	// request size information available. Request size is used to control how much traffic we admit
	// for processing, but does not control how much memory is used during request processing.
	AdmissionLimitMiB uint64 `mapstructure:"admission_limit_mib"`

	// WaiterLimit is the limit on the number of waiters waiting to be processed and consumed.
	// This is a dimension of memory limiting to ensure waiters are not consuming an
	// unexpectedly large amount of memory in the arrow receiver.
	WaiterLimit int64 `mapstructure:"waiter_limit"`

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
