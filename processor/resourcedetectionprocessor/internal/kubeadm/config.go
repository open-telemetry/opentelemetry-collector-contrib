// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm/internal/metadata"
)

type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`
	ResourceAttributes  metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
	configMapName       string
	configMapNamespace  string
}

const defaultConfigMapName = "kubeadm-config"
const defaultConfigMapNamespace = "kube-system"

// UpdateDefaults validates and update the default config with user's provided settings
func (c *Config) UpdateDefaults() error {
	c.configMapName = defaultConfigMapName
	c.configMapNamespace = defaultConfigMapNamespace

	return nil
}

func CreateDefaultConfig() Config {
	return Config{
		APIConfig:          k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
