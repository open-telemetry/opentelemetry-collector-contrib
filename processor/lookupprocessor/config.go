// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

type Config struct {
	Source SourceConfig `mapstructure:"source"`
}

type SourceConfig struct {
	// Type is the source type identifier (e.g., "noop", "yaml", "dns").
	Type string `mapstructure:"type"`

	// Config holds the source-specific configuration.
	// This is populated during config unmarshaling based on the Type.
	Config lookupsource.SourceConfig `mapstructure:"-"`
}

var _ component.Config = (*Config)(nil)

func (*Config) Validate() error {
	return nil
}
