// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

// Config wraps the shared healthcheck.Config to add extension-specific validation.
type Config struct {
	Config healthcheck.Config `mapstructure:",squash"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate checks if the extension configuration is valid, including feature gate checks.
func (c *Config) Validate() error {
	if !metadata.ExtensionHealthcheckUseComponentStatusFeatureGate.IsEnabled() && (c.Config.HTTPConfig != nil || c.Config.GRPCConfig != nil) {
		return errors.New(
			"v2 healthcheck configuration (http/grpc fields) detected but feature gate is disabled. " +
				"Either remove the v2 config fields or enable the feature gate with: " +
				"--feature-gates=+extension.healthcheck.useComponentStatus",
		)
	}

	return nil
}

// Type alias for backward compatibility
type ResponseBodySettings = healthcheck.ResponseBodyConfig
