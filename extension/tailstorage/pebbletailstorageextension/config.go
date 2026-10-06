// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"errors"
	"fmt"
)

// ReadErrorPolicy controls what Take returns when part of a stored trace
// cannot be read or decoded.
type ReadErrorPolicy string

const (
	// ReadErrorPolicyDropTrace deletes the stored trace and returns an error
	// from Take so the caller treats the whole trace as failed.
	ReadErrorPolicyDropTrace ReadErrorPolicy = "drop_trace"
	// ReadErrorPolicyReturnPartial deletes the stored trace and returns the
	// batches that could still be read.
	ReadErrorPolicyReturnPartial ReadErrorPolicy = "return_partial"
)

type Config struct {
	// Directory is where the extension stores Pebble DB files.
	Directory string `mapstructure:"directory"`
	// MaxStorageSizeMiB limits the amount of Pebble storage that may be used.
	// Zero means unlimited.
	MaxStorageSizeMiB int `mapstructure:"max_storage_size_mib"`
	// OnReadError selects what Take does when a stored batch of the trace
	// cannot be read or decoded. Defaults to drop_trace.
	OnReadError ReadErrorPolicy `mapstructure:"on_read_error"`
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
	switch c.OnReadError {
	case "", ReadErrorPolicyDropTrace, ReadErrorPolicyReturnPartial:
	default:
		return fmt.Errorf("on_read_error must be one of %q or %q, got %q",
			ReadErrorPolicyDropTrace, ReadErrorPolicyReturnPartial, c.OnReadError)
	}
	return nil
}
