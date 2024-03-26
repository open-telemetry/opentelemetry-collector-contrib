// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/redisstorage"

import (
	"errors"
)

// Config defines configuration for redisstorage extension.
type Config struct {
	Address  string `mapstructure:"address"`
	Username string `mapstructure:"username,omitempty"`
	Password string `mapstructure:"password,omitempty"`
}

func (cfg *Config) Validate() error {
	if cfg.Address == "" {
		return errors.New("missing datasource")
	}

	return nil
}
