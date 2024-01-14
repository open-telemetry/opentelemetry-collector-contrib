// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "journald_input"

// NewConfig creates a new input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
		StartAt:     "end",
		Priority:    "info",
	}
}

// Config is the configuration of a journald input operator
type Config struct {
	helper.InputConfig `mapstructure:",squash"`

	Directory   *string       `mapstructure:"directory,omitempty"`
	Files       []string      `mapstructure:"files,omitempty"`
	StartAt     string        `mapstructure:"start_at,omitempty"`
	Units       []string      `mapstructure:"units,omitempty"`
	Priority    string        `mapstructure:"priority,omitempty"`
	Matches     []MatchConfig `mapstructure:"matches,omitempty"`
	Identifiers []string      `mapstructure:"identifiers,omitempty"`
	Grep        string        `mapstructure:"grep,omitempty"`
	Dmesg       bool          `mapstructure:"dmesg,omitempty"`
}

type MatchConfig map[string]string
