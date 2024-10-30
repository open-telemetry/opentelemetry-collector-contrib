// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"
)

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC  configgrpc.ServerConfig `mapstructure:"grpc"`
	Arrow ArrowConfig             `mapstructure:"arrow"`
}

type AdmissionConfig struct {
	// RequestLimitMiB limits the number of requests that are received by the stream based on
	// uncompressed request size. Request size is used to control how much traffic we admit
	// for processing.
	RequestLimitMiB uint64 `mapstructure:"request_limit_mib"`

	// DeprecatedWaiterLimit is deprecated.  This field configures
	// a limit defined in terms of the number of requests waiting
	// when the request_limit_mib limit has been reached.  This field
	// will be replaced by a limit implemented in terms of waiting
	// request size instead of waiting request count.
	//
	// This field will continue to function until a new field
	// named `waiting_limit_mib` is introduced, at which time this
	// field will have no effect.
	DeprecatedWaiterLimit int64 `mapstructure:"waiter_limit"`
}

// ArrowConfig support configuring the Arrow receiver.
type ArrowConfig struct {
	// MemoryLimitMiB is the size of a shared memory region used
	// by all Arrow streams, in MiB.  When too much load is
	// passing through, they will see ResourceExhausted errors.
	MemoryLimitMiB uint64 `mapstructure:"memory_limit_mib"`

	// Deprecated: This field is no longer supported, use cfg.Admission.RequestLimitMiB instead.
	DeprecatedAdmissionLimitMiB uint64 `mapstructure:"admission_limit_mib"`

	// Deprecated: This field is no longer supported, use cfg.Admission.WaiterLimit instead.
	DeprecatedWaiterLimit int64 `mapstructure:"waiter_limit"`

	// Zstd settings apply to OTel-Arrow use of gRPC specifically.
	Zstd zstd.DecoderConfig `mapstructure:"zstd"`
}

// Config defines configuration for OTel Arrow receiver.
type Config struct {
	// Protocols is the configuration for gRPC and Arrow.
	Protocols `mapstructure:"protocols"`
	// Admission is the configuration for controlling amount of request memory entering the receiver.
	Admission AdmissionConfig `mapstructure:"admission"`
}

var _ component.Config = (*Config)(nil)
var _ component.ConfigValidator = (*ArrowConfig)(nil)

func (cfg *ArrowConfig) Validate() error {
	if err := cfg.Zstd.Validate(); err != nil {
		return fmt.Errorf("zstd decoder: invalid configuration: %w", err)
	}
	return nil
}

func (cfg *Config) Validate() error {
	if err := cfg.GRPC.Validate(); err != nil {
		return err
	}
	if err := cfg.Arrow.Validate(); err != nil {
		return err
	}
	return nil
}

// Unmarshal will apply deprecated field values to assist the user with migration.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(cfg); err != nil {
		return err
	}
	if cfg.Admission.RequestLimitMiB == 0 && cfg.Arrow.DeprecatedAdmissionLimitMiB != 0 {
		cfg.Admission.RequestLimitMiB = cfg.Arrow.DeprecatedAdmissionLimitMiB
	}
	if cfg.Admission.DeprecatedWaiterLimit == 0 && cfg.Arrow.DeprecatedWaiterLimit != 0 {
		cfg.Admission.DeprecatedWaiterLimit = cfg.Arrow.DeprecatedWaiterLimit
	}
	return nil
}
