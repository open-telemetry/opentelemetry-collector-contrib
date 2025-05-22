// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

// Config is the configuration for the processor.
type Config struct {
	// GroupByKeys describes the attribute names that are going to be used for grouping.
	// Empty value is allowed, since processor in such case can compact data
	GroupByKeys []string `mapstructure:"keys"`

	// prevent unkeyed literal initialization
	_ struct{}
}
