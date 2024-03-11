// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Deprecated: [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfig() *Config {
	return NewFactory().NewDefaultConfig(operatorType.String()).(*Config)
}

// Deprecated: [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfigWithID(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a windows event log operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	Channel            string        `mapstructure:"channel"`
	MaxReads           int           `mapstructure:"max_reads,omitempty"`
	StartAt            string        `mapstructure:"start_at,omitempty"`
	PollInterval       time.Duration `mapstructure:"poll_interval,omitempty"`
	Raw                bool          `mapstructure:"raw,omitempty"`
	ExcludeProviders   []string      `mapstructure:"exclude_providers,omitempty"`
}

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	set := component.TelemetrySettings{}
	if logger != nil {
		set.Logger = logger.Desugar()
	}
	return NewFactory().CreateOperator(&c, set)
}
