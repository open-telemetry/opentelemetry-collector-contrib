// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import "errors"

type Config struct {
	// Directory is where the extension stores Pebble DB files.
	Directory string `mapstructure:"directory"`
	// MaxStorageSizeMiB limits the amount of Pebble storage that may be used.
	// Zero means unlimited.
	MaxStorageSizeMiB int `mapstructure:"max_storage_size_mib"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if c.Directory == "" {
		return errors.New("directory must be set")
	}
	if c.MaxStorageSizeMiB < 0 {
		return errors.New("max_storage_size_mib must be greater than or equal to zero")
	}
	return nil
}
