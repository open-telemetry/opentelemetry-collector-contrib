package cloudwatchencodingextension

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct{}

const defaultDelimiter = "\n"

func createDefaultConfig() component.Config {
	return &Config{}
}

func (c *Config) Validate() error {
	return nil
}
