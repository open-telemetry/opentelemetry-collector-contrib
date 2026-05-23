// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cvm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/tencent/cvm"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/tencent/cvm/internal/metadata"
)

// Config defines user-specified configurations unique to the Tencent Cloud CVM detector.
type Config struct {
	// ResourceAttributes controls which standard resource attributes are enabled
	// (e.g., host.id, host.name, cloud.provider, etc.).
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`

	// FailOnMissingMetadata, if true, causes the detector to return an error
	// when the Tencent Cloud CVM metadata service is unavailable or any required field is missing.
	// If false (default), the detector logs the failure and returns an empty resource (no attributes set).
	FailOnMissingMetadata bool `mapstructure:"fail_on_missing_metadata"`
}

// CreateDefaultConfig returns the default configuration for the Tencent Cloud CVM detector.
func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes:    metadata.DefaultResourceAttributesConfig(),
		FailOnMissingMetadata: false,
	}
}
