// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"errors"
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

	return nil
}
