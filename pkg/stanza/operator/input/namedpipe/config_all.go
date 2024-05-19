// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namedpipe // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	operatorType      = "namedpipe"
	DefaultMaxLogSize = 1024 * 1024
)

func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfig creates a new stdin input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
	}
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
