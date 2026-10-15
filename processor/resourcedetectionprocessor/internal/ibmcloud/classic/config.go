// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package classic // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic/internal/metadata"
)

// Config defines user-specified configurations unique to the IBM Cloud Classic detector.
type Config struct {
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

// CreateDefaultConfig returns the default configuration for the IBM Cloud Classic detector.
func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
