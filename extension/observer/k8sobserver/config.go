// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config defines configuration for k8s attributes processor.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	// Node is the node name to limit the discovery of pod, port, and node endpoints.
	// Providing no value (the default) results in discovering endpoints for all available nodes.
	// For example, node name can be set using the downward API inside the collector
	// pod spec as follows:
	//
	// env:
	//   - name: K8S_NODE_NAME
	//     valueFrom:
	//       fieldRef:
	//         fieldPath: spec.nodeName
	//
	// Then set this value to ${env:K8S_NODE_NAME} in the configuration.
	Node string `mapstructure:"node"`
	// ObservePods determines whether to report observer pod and port endpoints. If `true` and Node is specified
	// it will only discover pod and port endpoints whose `spec.nodeName` matches the provided node name. If `true` and
	// Node isn't specified, it will discover all available pod and port endpoints. `true` by default.
	ObservePods bool `mapstructure:"observe_pods"`
	// ObserveNodes determines whether to report observer k8s.node endpoints. If `true` and Node is specified
	// it will only discover node endpoints whose `metadata.name` matches the provided node name. If `true` and
	// Node isn't specified, it will discover all available node endpoints. `false` by default.
	ObserveNodes bool `mapstructure:"observe_nodes"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if !cfg.ObservePods && !cfg.ObserveNodes {
		return fmt.Errorf("one of observe_pods and observe_nodes must be true")
	}
	return nil
}
