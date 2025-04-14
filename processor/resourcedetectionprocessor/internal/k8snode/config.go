// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snode // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8snode"

import (
	"errors"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/k8snode/internal/metadata"
)

type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`
	// NodeFromEnv can be used to extract the node name from an environment
	// variable. The value must be the name of the environment variable.
	// This is useful when the node a Otel agent will run on cannot be
	// predicted. In such cases, the Kubernetes downward API can be used to
	// add the node name to each pod as an environment variable. K8s tagger
	// can then read this value and filter pods by it.
	// NodeFromEnv defaults to K8S_NODE_NAME
	//
	// For example, node name can be passed to each agent with the downward API as follows
	//
	// env:
	//   - name: K8S_NODE_NAME
	//     valueFrom:
	//       fieldRef:
	//         fieldPath: spec.nodeName
	//
	// Then the NodeFromEnv field can be set to `K8S_NODE_NAME` to filter all pods by the node that
	// the agent is running on.
	//
	// More on downward API here: https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/
	NodeFromEnvVar     string                            `mapstructure:"node_from_env_var"`
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

// UpdateDefaults validates and update the default config with user's provided settings
func (c *Config) UpdateDefaults() error {
	if c.NodeFromEnvVar == "" {
		c.NodeFromEnvVar = "K8S_NODE_NAME"
	}
	if value, envExists := os.LookupEnv(c.NodeFromEnvVar); !envExists || value == "" {
		return errors.New("node name can't be found. Check the readme on how to set the required env variable")
	}
	return nil
}

func CreateDefaultConfig() Config {
	return Config{
		APIConfig:          k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
