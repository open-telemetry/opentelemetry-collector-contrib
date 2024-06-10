package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import "fmt"

type CacheConfig struct {
	maxCacheSizeBytes int64 `mapstructure:"max_cache_size_bytes"`
	maxCachedEntries  int64 `mapstructure:"max_cached_entries"`
}

type DatabaseBlueprintsConfig struct {
	WithSampling bool `mapstructure:"with_sampling"`
	CacheConfig  `mapstructure:"cache_config"`
}

type Config struct {
	DatabaseBlueprintsConfig `mapstructure:"database_blueprints_config"`
}

func (c *Config) Validate() error {
	if c.DatabaseBlueprintsConfig.WithSampling == false {
		if c.DatabaseBlueprintsConfig.CacheConfig.maxCacheSizeBytes > 0 {
			return fmt.Errorf("max_cache_size_bytes cannot be set if with_sampling is false")
		}
		if c.DatabaseBlueprintsConfig.CacheConfig.maxCachedEntries > 0 {
			return fmt.Errorf("buffer_items cannot be set if with_sampling is false")
		}
	} else {
		if c.DatabaseBlueprintsConfig.CacheConfig.maxCacheSizeBytes <= 0 {
			return fmt.Errorf("max_cache_size_bytes must be a positive integer")
		}
		if c.DatabaseBlueprintsConfig.CacheConfig.maxCachedEntries <= 0 {
			return fmt.Errorf("max_cached_entries must be a positive integer")
		}
	}
	return nil
}
