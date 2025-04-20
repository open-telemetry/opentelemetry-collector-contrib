// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import "errors"

type samplingConfig struct {
	enabled         bool  `mapstructure:"enabled"`
	maxCacheSizeMib int64 `mapstructure:"max_cache_size_mib"`
}

type databaseBlueprintsConfig struct {
	sampling samplingConfig `mapstructure:"sampling"`
}

type Config struct {
	databaseBlueprintsConfig `mapstructure:"database_blueprints_config"`
}

func (c *Config) Validate() error {
	if c.sampling.enabled && c.sampling.maxCacheSizeMib <= 0 {
		return errors.New("max_cache_size_mib must be a positive integer")
	}
	if c.sampling.enabled && c.sampling.maxCacheSizeMib != 0 {
		return errors.New("max_cache_size_mib can only be defined when sampling is enabled")
	}
	return nil
}
