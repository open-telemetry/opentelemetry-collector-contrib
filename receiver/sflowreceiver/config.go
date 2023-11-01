// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sflowreceiver"

import (
	"go.opentelemetry.io/collector/config/confignet"
)

type Config struct {
	confignet.NetAddr `mapstructure:",squash"`
	Labels            map[string]string `mapstructure:"labels"`
}

func (cfg *Config) Validate() error {
	return nil
}
