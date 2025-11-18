// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import "time"

type Config struct {
	// CollectionInterval sets how frequently profiles should be collected.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// Fraction of blocking events that are profiled. A value <= 0 disables
	// profiling. See https://golang.org/pkg/runtime/#SetBlockProfileRate for details.
	BlockProfileFraction int `mapstructure:"block_profile_fraction"`

	// Fraction of mutex contention events that are profiled. A value <= 0
	// disables profiling. See https://golang.org/pkg/runtime/#SetMutexProfileFraction
	// for details.
	MutexProfileFraction int `mapstructure:"mutex_profile_fraction"`
}
