// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc/internal/metadata"
)

// Config defines user-specified configurations unique to the IBM Cloud VPC detector.
type Config struct {
	// Secure switches the IMDS endpoint from http:// to https://
	// (both use api.metadata.cloud.ibm.com).
	Secure             bool                              `mapstructure:"secure"`
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

// CreateDefaultConfig returns the default configuration for the IBM Cloud VPC detector.
func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
