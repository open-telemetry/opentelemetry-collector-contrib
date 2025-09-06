// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/confmap"
)

// Protocols is the configuration for the supported protocols.
type Protocols struct {
	GRPC  configgrpc.ServerConfig `mapstructure:"grpc"`
	Arrow ArrowConfig             `mapstructure:"arrow"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type AdmissionConfig struct {
	// RequestLimitMiB limits the number of requests that are received by the stream based on
	// uncompressed request size. Request size is used to control how much traffic we admit
	// for processing.  When this field is zero, admission control is disabled.
	RequestLimitMiB uint64 `mapstructure:"request_limit_mib"`

	// WaitingLimitMiB is the limit on the amount of data waiting to be consumed.
	// This is a dimension of memory limiting to ensure waiters are not consuming an
	// unexpectedly large amount of memory in the arrow receiver.
	WaitingLimitMiB uint64 `mapstructure:"waiting_limit_mib"`

	// prevent unkeyed literal initialization
	_ struct{}
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

	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines configuration for OTel Arrow receiver.
type Config struct {
	// Protocols is the configuration for gRPC and Arrow.
	Protocols `mapstructure:"protocols"`
	// Admission is the configuration for controlling amount of request memory entering the receiver.
	Admission AdmissionConfig `mapstructure:"admission"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Unmarshal will apply deprecated field values to assist the user with migration.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(cfg); err != nil {
		return err
	}
	if cfg.Admission.RequestLimitMiB == 0 && cfg.Arrow.DeprecatedAdmissionLimitMiB != 0 {
		cfg.Admission.RequestLimitMiB = cfg.Arrow.DeprecatedAdmissionLimitMiB
	}
	return nil
}
