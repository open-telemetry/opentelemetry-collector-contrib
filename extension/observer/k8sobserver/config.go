// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// CRDConfig defines the configuration for observing a specific Custom Resource Definition.
type CRDConfig struct {
	// Group is the API group of the CRD (e.g., "mycompany.io").
	Group string `mapstructure:"group"`
	// Version is the API version of the CRD (e.g., "v1", "v1alpha1").
	Version string `mapstructure:"version"`
	// Kind is the kind of the CRD (e.g., "MyResource").
	Kind string `mapstructure:"kind"`
	// Namespaces limits the namespaces for the observed CRD. If empty, all namespaces will be observed.
	// This overrides the global Namespaces setting for this specific CRD.
	Namespaces []string `mapstructure:"namespaces"`
}

// Validate checks if the CRD configuration is valid.
func (c *CRDConfig) Validate() error {
	if c.Group == "" {
		return errors.New("CRD group is required")
	}
	if c.Version == "" {
		return errors.New("CRD version is required")
	}
	if c.Kind == "" {
		return errors.New("CRD kind is required")
	}
	return nil
}

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
	// ObserveServices determines whether to report observer service and port endpoints. `false` by default.
	ObserveServices bool `mapstructure:"observe_services"`
	// ObserveIngresses determines whether to report observer ingress. `false` by default.
	ObserveIngresses bool `mapstructure:"observe_ingresses"`
	// ObserveCRDs is a list of Custom Resource Definitions to observe. `nil` by default.
	ObserveCRDs []CRDConfig `mapstructure:"observe_crds"`
	// Namespaces limits the namespaces for the observed resources. By default, all namespaces will be observed.
	Namespaces []string `mapstructure:"namespaces"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if !cfg.ObservePods && !cfg.ObserveNodes && !cfg.ObserveServices && !cfg.ObserveIngresses && len(cfg.ObserveCRDs) == 0 {
		return errors.New("one of observe_pods, observe_nodes, observe_services, observe_ingresses must be true, or observe_crds must be specified")
	}
	for i, crd := range cfg.ObserveCRDs {
		if err := crd.Validate(); err != nil {
			return fmt.Errorf("observe_crds[%d]: %w", i, err)
		}
	}
	return nil
}
