package kubeadm

import (
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/kubeadm/internal/metadata"
)

type Config struct {
	k8sconfig.APIConfig          `mapstructure:",squash"`
	ConfigMapNameFromEnvVar      string                            `mapstructure:"configmap_name_from_env_var"`
	ConfigMapNamespaceFromEnvVar string                            `mapstructure:"configmap_namespace_from_env_var"`
	ResourceAttributes           metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
	configMapName                string
	configMapNamespace           string
}

// UpdateDefaults validates and update the default config with user's provided settings
func (c *Config) UpdateDefaults() error {
	c.configMapName = "kubeadm-config"
	c.configMapNamespace = "kube-system"

	if c.ConfigMapNameFromEnvVar != "" {
		value, envExists := os.LookupEnv(c.ConfigMapNameFromEnvVar)
		if !envExists || value == "" {
			return fmt.Errorf("ConfigMap name can't be found. Check the readme on how to set the required env variable")
		}
		c.configMapName = value
	}

	if c.ConfigMapNamespaceFromEnvVar != "" {
		value, envExists := os.LookupEnv(c.ConfigMapNamespaceFromEnvVar)
		if !envExists || value == "" {
			return fmt.Errorf("ConfigMap namespace can't be found. Check the readme on how to set the required env variable")
		}
		c.configMapNamespace = value
	}

	return nil
}

func CreateDefaultConfig() Config {
	return Config{
		APIConfig:          k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
