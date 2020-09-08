package logzioexporter

import (
	"errors"

	"go.opentelemetry.io/collector/config/configmodels"
)

type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"`
	Token                         string `mapstructure:"account_token"`
	Region                        string `mapstructure:"region"`
	CustomListenerAddress         string `mapstructure:"custom_listener_address"`
}

func (c *Config) validate() error {
	if c.Token == "" {
		return errors.New("`Account Token` not specified")
	}
	return nil
}
