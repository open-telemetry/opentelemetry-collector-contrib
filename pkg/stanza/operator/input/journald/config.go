// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Deprecated [v0.97.0] Use Factory.CreateDefaultConfig instead.
func NewConfig() *Config {
	return NewFactory().NewDefaultConfig(operatorType.String()).(*Config)
}

// Deprecated [v0.97.0] Use Factory.CreateDefaultConfig instead.
func NewConfigWithID(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
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
	All         bool          `mapstructure:"all,omitempty"`
}

type MatchConfig map[string]string
