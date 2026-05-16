// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nova // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openstack/nova/internal/metadata"
)

// Config defines user-specified configurations unique to the Nova detector.
type Config struct {
	// Labels is a list of regex patterns to match Nova instance metadata keys
	// (from the "meta" map) that should be added as resource attributes.
	// Matched keys are emitted as "openstack.nova.meta.<key>: <value>".
	Labels []string `mapstructure:"labels"`

	// ResourceAttributes controls which standard resource attributes are enabled
	// (e.g., host.id, host.name, cloud.provider, etc.).
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`

	// Deprecated: Use the top-level fail_on_missing_metadata in the processor config instead.
	// This field will be removed in a future release.
	FailOnMissingMetadata bool `mapstructure:"fail_on_missing_metadata"`
}

// CreateDefaultConfig returns the default configuration for the Nova detector.
func CreateDefaultConfig() Config {
	return Config{
		Labels:                []string{},
		ResourceAttributes:    metadata.DefaultResourceAttributesConfig(),
		FailOnMissingMetadata: false,
	}
}
