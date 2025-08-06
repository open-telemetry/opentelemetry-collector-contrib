//go:build !linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"fmt"
	"os"
)

// validatePasswordFilePermissions checks that the password file is readable on non-Linux platforms
// On Windows and other platforms, we rely on the OS-level file permissions rather than Unix-style octal permissions
func (c *Config) validatePasswordFilePermissions() error {
	// Check if file exists and is accessible
	if _, err := os.Stat(c.PasswordFile); err != nil {
		return fmt.Errorf("`password_file` is inaccessible: %w", err)
	}

	// Test that we can actually read the file
	file, err := os.Open(c.PasswordFile)
	if err != nil {
		return fmt.Errorf("`password_file` cannot be read: %w", err)
	}
	defer file.Close()

	return nil
}
