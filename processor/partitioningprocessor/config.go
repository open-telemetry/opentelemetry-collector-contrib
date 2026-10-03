// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"errors"
	"fmt"
	"runtime"
)

// Config is the configuration for the partitioning processor.
type Config struct {
	// Keys maps partition key names to OTTL value expressions.
	// Each expression is evaluated against each incoming item; items that
	// produce the same set of values are collected into the same partition,
	// which is forwarded downstream with the key/value pairs added to the
	// outgoing request metadata.
	Keys map[string]string `mapstructure:"keys"`

	// MaxConcurrentPartitions limits how many partition deliveries may run at
	// once. A value of 0 uses the default limit (runtime.GOMAXPROCS(0)).
	MaxConcurrentPartitions int `mapstructure:"max_concurrent_partitions"`
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	if len(c.Keys) == 0 {
		return errors.New("at least one key must be configured")
	}
	for name, value := range c.Keys {
		if value == "" {
			return fmt.Errorf("key %q has an empty value expression", name)
		}
	}
	if c.MaxConcurrentPartitions < 0 {
		return errors.New("max_concurrent_partitions must be greater than or equal to 0")
	}
	return nil
}

func defaultMaxConcurrentPartitions() int {
	return runtime.GOMAXPROCS(0)
}

func configuredMaxConcurrentPartitions(c *Config) int {
	if c.MaxConcurrentPartitions > 0 {
		return c.MaxConcurrentPartitions
	}
	return defaultMaxConcurrentPartitions()
}
