package awsecscontainermetricsreceiver

import (
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration for aws ecs container metrics receiver.
type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`

	// CollectionInterval is the interval at which metrics should be collected
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
}
