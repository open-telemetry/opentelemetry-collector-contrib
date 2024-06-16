package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import "fmt"

type sampingConfig struct {
	maxCacheSizeMib  int64 `mapstructure:"max_cache_size_mib"`
	maxCachedEntries int64 `mapstructure:"max_cached_entries"`
}

type databaseBlueprintsConfig struct {
	sampling sampingConfig `mapstructure:"sampling"`
}

type Config struct {
	databaseBlueprintsConfig `mapstructure:"database_blueprints_config"`
}

func (c *Config) Validate() error {
	if c.databaseBlueprintsConfig.sampling.maxCacheSizeMib <= 0 {
		return fmt.Errorf("max_cache_size_mib must be a positive integer")
	}
	if c.databaseBlueprintsConfig.sampling.maxCachedEntries <= 0 {
		return fmt.Errorf("max_cached_entries must be a positive integer")
	}
	return nil
}
