package spoardicconnector

import "go.opentelemetry.io/collector/component"

type Config struct {

	// 1 for random non-permanent errors
	// 2 for random permanent errors
	// 3 for random avaibility
	decision int `mapstructure:"decision"`
}

func createDefaultConfig() component.Config {
	return &Config{
		decision: 1,
	}
}
