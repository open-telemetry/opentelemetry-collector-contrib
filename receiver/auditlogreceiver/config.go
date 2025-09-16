package auditlogreceiver

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// StorageID specifies the ID of the storage extension to use for persistent storage
	StorageID component.ID `mapstructure:"storage"`

	// ProcessInterval specifies how often the receiver processes stored logs
	// Default: 60s
	ProcessInterval time.Duration `mapstructure:"process_interval"`

	// ProcessAgeThreshold specifies how old logs must be before they are processed
	// Default: 60s
	ProcessAgeThreshold time.Duration `mapstructure:"process_age_threshold"`
}
