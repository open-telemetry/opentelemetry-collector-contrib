// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"errors"
	"fmt"
	"strings"
)

// Config is the configuration for the partitioning processor.
type Config struct {
	// Keys maps partition key names to OTTL value expressions.
	// Each expression is evaluated against each incoming item; items that
	// produce the same set of values are collected into the same partition,
	// which is forwarded downstream with the key/value pairs added to the
	// outgoing request metadata.
	Keys map[string]string `mapstructure:"keys"`
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	if len(c.Keys) == 0 {
		return errors.New("at least one key must be configured")
	}
	// Key names become client.Metadata keys, which are case-insensitive.
	lower := make(map[string]string, len(c.Keys))
	for name, value := range c.Keys {
		if value == "" {
			return fmt.Errorf("key %q has an empty value expression", name)
		}
		l := strings.ToLower(name)
		if other, ok := lower[l]; ok {
			return fmt.Errorf("keys %q and %q collide: key names are case-insensitive", other, name)
		}
		lower[l] = name
	}
	return nil
}
