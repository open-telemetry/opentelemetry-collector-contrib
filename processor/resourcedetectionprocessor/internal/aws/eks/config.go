// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

type Config struct {
	APIConfig          k8sconfig.APIConfig               `mapstructure:",squash"`
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
	NodeFromEnvVar     string                            `mapstructure:"node_from_env_var"`
}

func CreateDefaultConfig() Config {
	return Config{
		APIConfig:          k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
