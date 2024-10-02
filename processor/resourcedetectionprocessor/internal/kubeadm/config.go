// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm"

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

func CreateDefaultConfig() Config {
	return Config{
		APIConfig:          k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		configMapName:      defaultConfigMapName,
		configMapNamespace: defaultConfigMapNamespace,
	}
}
