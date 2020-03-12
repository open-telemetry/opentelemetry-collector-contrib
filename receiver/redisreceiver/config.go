package redisreceiver

import "github.com/open-telemetry/opentelemetry-collector/config/configmodels"

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	Password                      string `mapstructure:"password"`
}
