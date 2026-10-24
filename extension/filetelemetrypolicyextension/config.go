// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filetelemetrypolicyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/filetelemetrypolicyextension"

import (
	"go.opentelemetry.io/collector/component"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for the file_telemetry_policy extension.
type Config struct {
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate checks if the configuration is valid.
func (*Config) Validate() error {
	return nil
}
