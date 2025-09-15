// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	Source SourceConfig `mapstructure:"source"`
	_      struct{}     `mapstructure:"-"`
}

type SourceConfig struct {
	Extension component.ID `mapstructure:"extension"`
	_         struct{}     `mapstructure:"-"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.Source.Extension == (component.ID{}) {
		return errors.New("lookup extension id must be configured")
	}
	return nil
}
