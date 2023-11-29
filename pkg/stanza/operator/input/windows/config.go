// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "windows_eventlog_input"

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
