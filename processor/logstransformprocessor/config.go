// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstransformprocessor"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

// Config defines configuration for Resource processor.
type Config struct {
	adapter.BaseConfig `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.BaseConfig.Operators) == 0 {
		return errors.New("no operators were configured for this logs transform processor")
	}
	return nil
}
