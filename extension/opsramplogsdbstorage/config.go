package opsramplogsdbstorage

import (
	"fmt"
)

// Config defines configuration for dbstorage extension.
type Config struct {
	DBPath string `mapstructure:"db_path, omitempty"`
}

func (cfg *Config) Validate() error {
	if cfg.DBPath == "" {
		return fmt.Errorf("missing db_path")
	}

	return nil
}
