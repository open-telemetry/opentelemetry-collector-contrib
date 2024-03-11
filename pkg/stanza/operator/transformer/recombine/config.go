// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const defaultCombineWith = "\n"

// Deprecated [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfig() *Config {
	return NewFactory().NewDefaultConfig(operatorType.String()).(*Config)
}

// Deprecated [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfigWithID(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a recombine operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	IsFirstEntry             string          `mapstructure:"is_first_entry"`
	IsLastEntry              string          `mapstructure:"is_last_entry"`
	MaxBatchSize             int             `mapstructure:"max_batch_size"`
	CombineField             entry.Field     `mapstructure:"combine_field"`
	CombineWith              string          `mapstructure:"combine_with"`
	SourceIdentifier         entry.Field     `mapstructure:"source_identifier"`
	OverwriteWith            string          `mapstructure:"overwrite_with"`
	ForceFlushTimeout        time.Duration   `mapstructure:"force_flush_period"`
	MaxSources               int             `mapstructure:"max_sources"`
	MaxLogSize               helper.ByteSize `mapstructure:"max_log_size,omitempty"`
}

// Deprecated [v0.97.0] Use Factory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	set := component.TelemetrySettings{}
	if logger != nil {
		set.Logger = logger.Desugar()
	}
	return NewFactory().CreateOperator(&c, set)
}
