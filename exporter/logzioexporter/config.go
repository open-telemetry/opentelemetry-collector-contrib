package logzioexporter

import (
	"go.opentelemetry.io/collector/config/configmodels"
)


type Config struct {
	configmodels.ExporterSettings 	`mapstructure:",squash"`
	Token string					`mapstructure:"account_token"`
	Region string					`mapstructure:"region"`
	CustomListenerAddress string	`mapstructure:"custom_listener_address"`
}
