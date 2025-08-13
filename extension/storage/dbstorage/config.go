// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"errors"
	"fmt"
)

const (
	driverPostgreSQL   = "pgx"
	driverSQLite       = "sqlite"
	driverSQLiteLegacy = "sqlite3"
)

// Config defines configuration for dbstorage extension.
type Config struct {
	DriverName string `mapstructure:"driver,omitempty"`
	DataSource string `mapstructure:"datasource,omitempty"`
}

func (cfg *Config) Validate() error {
	if cfg.DataSource == "" {
		return errors.New("missing datasource")
	}
	if cfg.DriverName == "" {
		return errors.New("missing driver name")
	}

	if cfg.DriverName != driverPostgreSQL &&
		cfg.DriverName != driverSQLite &&
		cfg.DriverName != driverSQLiteLegacy {
		return fmt.Errorf("unsupported driver %s", cfg.DriverName)
	}

	return nil
}
