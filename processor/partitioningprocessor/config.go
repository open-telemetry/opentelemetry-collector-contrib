// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
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
	return validateExpressions(c.Keys)
}

// validateExpressions reports an error only if no signal can parse all key
// expressions together. The same keys apply to every pipeline the processor is
// used in, so processor creation still enforces each pipeline's signal.
func validateExpressions(keys map[string]string) error {
	_, expressions := sortedPartitionKeys(keys)
	set := component.TelemetrySettings{Logger: zap.NewNop()}
	parsers := []struct {
		signal string
		parse  func() error
	}{
		{"logs", func() error { _, err := newLogsPartitioner(expressions, set); return err }},
		{"traces", func() error { _, err := newTracesPartitioner(expressions, set); return err }},
		{"metrics", func() error { _, err := newMetricsPartitioner(expressions, set); return err }},
	}
	errs := make([]error, 0, len(parsers))
	for _, p := range parsers {
		err := p.parse()
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", p.signal, err))
	}
	return fmt.Errorf("key expressions cannot be used with any signal: %w", errors.Join(errs...))
}
