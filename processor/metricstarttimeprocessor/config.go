// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstarttimeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

// Config holds configuration of the metric start time processor.
type Config struct{}

var _ component.Config = (*Config)(nil)

// Validate checks the configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
