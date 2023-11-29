// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// NewConfig will return an event log config with default values.
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfig will return an event log config with default values.
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig:  helper.NewInputConfig(operatorID, operatorType),
		MaxReads:     100,
		StartAt:      "end",
		PollInterval: 1 * time.Second,
	}
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

// Build will build a windows event log operator.
func (c *Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.Channel == "" {
		return nil, fmt.Errorf("missing required `channel` field")
	}

	if c.MaxReads < 1 {
		return nil, fmt.Errorf("the `max_reads` field must be greater than zero")
	}

	if c.StartAt != "end" && c.StartAt != "beginning" {
		return nil, fmt.Errorf("the `start_at` field must be set to `beginning` or `end`")
	}

	return &Input{
		InputOperator:    inputOperator,
		buffer:           NewBuffer(),
		channel:          c.Channel,
		maxReads:         c.MaxReads,
		startAt:          c.StartAt,
		pollInterval:     c.PollInterval,
		raw:              c.Raw,
		excludeProviders: c.ExcludeProviders,
	}, nil
}
