package redisreceiver

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
)

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	// The duration between Redis metric fetches.
	RefreshInterval               time.Duration `mapstructure:"refresh_interval"`
	// Optional password. Must match the password specified in the
	// requirepass server configuration option.
	Password                      string        `mapstructure:"password"`
}
