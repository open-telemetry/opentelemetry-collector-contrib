//go:build linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"fmt"
	"os"
)

// validatePasswordFilePermissions checks that the password file has appropriate Unix permissions (400 or 600)
func (c *Config) validatePasswordFilePermissions() error {
	info, err := os.Stat(c.PasswordFile)
	if err != nil {
		return fmt.Errorf("`password_file` is inaccessible: %w", err)
	}

	switch info.Mode().Perm() {
	// Matches JMX agent requirements for password file.
	case 0o400, 0o600:
		return nil
	default:
		return fmt.Errorf("`password_file` read access must be restricted to owner-only: %s", c.PasswordFile)
	}
}
