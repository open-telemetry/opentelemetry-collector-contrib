// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc/internal/metadata"
)

// Config defines user-specified configurations unique to the IBM Cloud VPC detector.
type Config struct {
	// Protocol selects the scheme used to reach the IMDS endpoint.
	// Accepted values are "http" (default) and "https".
	// Both use the host api.metadata.cloud.ibm.com.
	Protocol           string                            `mapstructure:"protocol"`
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

// CreateDefaultConfig returns the default configuration for the IBM Cloud VPC detector.
func CreateDefaultConfig() Config {
	return Config{
		Protocol:           "http",
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
