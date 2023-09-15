package sporadicconnector

import "go.opentelemetry.io/collector/component"

var _ component.ConfigValidator = (*Config)(nil)

type Config struct {
	// 0 for random non-permanent errors
	// 1 for random permanent errors
	// 2 for random avaibility
	Decision int `mapstructure:"decision"`
}

func (c *Config) Validate() error {
	return nil
}
