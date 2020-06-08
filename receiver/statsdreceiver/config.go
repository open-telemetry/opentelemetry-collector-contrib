package statsdreceiver

import (
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration for StatsD receiver.
type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`

	Timeout time.Duration `mapstructure:"timeout"`
}
