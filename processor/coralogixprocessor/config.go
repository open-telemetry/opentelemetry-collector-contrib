package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import "fmt"

type cacheConfig struct {
	maxCacheSizeMebibytes int64 `mapstructure:"max_cache_size_mib"`
	maxCachedEntries      int64 `mapstructure:"max_cached_entries"`
}

type databaseBlueprintsConfig struct {
	withSampling bool `mapstructure:"with_sampling"`
	cacheConfig  `mapstructure:"cache_config"`
}

type Config struct {
	databaseBlueprintsConfig `mapstructure:"database_blueprints_config"`
}

func (c *Config) Validate() error {
	if c.databaseBlueprintsConfig.withSampling == true {
		if c.databaseBlueprintsConfig.cacheConfig.maxCacheSizeMebibytes <= 0 {
			return fmt.Errorf("max_cache_size_bytes must be a positive integer")
		}
		if c.databaseBlueprintsConfig.cacheConfig.maxCachedEntries <= 0 {
			return fmt.Errorf("max_cached_entries must be a positive integer")
		}
	} else {
		if c.databaseBlueprintsConfig.cacheConfig.maxCacheSizeMebibytes > 0 {
			return fmt.Errorf("max_cache_size_bytes cannot be set if with_sampling is false")
		}
		if c.databaseBlueprintsConfig.cacheConfig.maxCachedEntries > 0 {
			return fmt.Errorf("buffer_items cannot be set if with_sampling is false")
		}
	}
	return nil
}
