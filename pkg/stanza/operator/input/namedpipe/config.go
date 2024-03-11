// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namedpipe // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const DefaultMaxLogSize = 1024 * 1024

// Deprecated: [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfig() *Config {
	return NewFactory().NewDefaultConfig(operatorType.String()).(*Config)
}

// Deprecated: [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfigWithID(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a stdin input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

type BaseConfig struct {
	Path        string       `mapstructure:"path"`
	Permissions uint32       `mapstructure:"mode"`
	Encoding    string       `mapstructure:"encoding"`
	SplitConfig split.Config `mapstructure:"multiline,omitempty"`
	TrimConfig  trim.Config  `mapstructure:",squash"`
	MaxLogSize  int          `mapstructure:"max_log_size"`
}
