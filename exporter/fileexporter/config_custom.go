package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/confmap"
)

const (
	rotationFieldName = "rotation"
	backupsFieldName  = "max_backups"
)

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return errors.New("empty config for file exporter")
	}
	// first load the config normally
	err := componentParser.Unmarshal(cfg, confmap.WithErrorUnused())
	if err != nil {
		return err
	}

	// next manually search for protocols in the confmap.Conf,
	// if rotation is not present it means it is disabled.
	if !componentParser.IsSet(rotationFieldName) {
		cfg.Rotation = nil
	}

	// set flush interval to 1 second if not set.
	if cfg.FlushInterval == nil {
		var sec = time.Second
		cfg.FlushInterval = &sec
	}
	return nil
}

func (cfg *Config) Validate() error {
	if err := cfg.ValidateHelper(); err != nil {
		return err
	}
	if cfg.Path == "" {
		return errors.New("path must be non-empty")
	}
	if cfg.FlushInterval != nil && *cfg.FlushInterval < 0 {
		return errors.New("flush_interval must be larger than zero")
	}
	return nil
}
