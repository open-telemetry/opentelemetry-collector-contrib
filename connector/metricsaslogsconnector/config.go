package metricsaslogsconnector

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	IncludeResourceAttributes bool `mapstructure:"include_resource_attributes"`

	IncludeScopeInfo bool `mapstructure:"include_scope_info"`

	_ struct{}
}

func (c *Config) Validate() error {
	return nil
}

func createDefaultConfig() component.Config {
	return &Config{
		IncludeResourceAttributes: true,
		IncludeScopeInfo:          true,
	}
}
