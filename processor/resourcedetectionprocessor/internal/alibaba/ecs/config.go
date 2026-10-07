// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/alibaba/ecs"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/alibaba/ecs/internal/metadata"
)

// Config defines user-specified configurations unique to the Alibaba Cloud ECS detector.
type Config struct {
	// ResourceAttributes controls which standard resource attributes are enabled
	// (e.g., host.id, host.name, cloud.provider, etc.).
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`

	// Deprecated: Use the top-level fail_on_missing_metadata in the processor config instead.
	// This field will be removed in a future release.
	FailOnMissingMetadata bool `mapstructure:"fail_on_missing_metadata"`
}

// CreateDefaultConfig returns the default configuration for the Alibaba Cloud ECS detector.
func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes:    metadata.DefaultResourceAttributesConfig(),
		FailOnMissingMetadata: false,
	}
}
