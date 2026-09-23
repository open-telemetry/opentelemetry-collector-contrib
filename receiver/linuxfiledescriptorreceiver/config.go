package linuxfiledescriptorreceiver

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	Path     string        `mapstructure:"path"`
	Interval time.Duration `mapstructure:"collection_interval"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Path:     "/proc/sys/fs/file-nr",
		Interval: 1 * time.Minute,
	}
}
