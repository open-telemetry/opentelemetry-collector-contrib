// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadoglogreceiver

import (
	"fmt"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	Version               string                   `mapstructure:"version"`
	HTTP                  *confighttp.ServerConfig `mapstructure:"http"`
	EnableDdtagsAttribute bool                     `mapstructure:"enable_ddtags_attribute"`
}

func (cfg *Config) Validate() error {
	if cfg.Version != "" && cfg.Version != "v2_api" {
		return fmt.Errorf("invalid version %v, only `v2_api` is supported", cfg.Version)
	}

	return nil
}
