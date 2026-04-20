// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snodemetadataprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config defines the configuration for the k8snodemetadata processor.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	// LocalMode restricts the processor to only watch the local node.
	// The node name is resolved from K8S_NODE_NAME env var or os.Hostname().
	// Use this in DaemonSet deployments to avoid watching all nodes.
	LocalMode bool `mapstructure:"local_mode"`
}

func (cfg *Config) Validate() error {
	return cfg.APIConfig.Validate()
}
