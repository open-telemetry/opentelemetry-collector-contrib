// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"errors"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// DefaultInitContainerTerminatedTTL is the default time-to-live for terminated init container endpoints.
const DefaultInitContainerTerminatedTTL = 15 * time.Minute

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
	// Namespaces limits the namespaces for the observed resources. By default, all namespaces will be observed.
	Namespaces []string `mapstructure:"namespaces"`
	// ObservePodPhases specifies which pod phases to observe. Only pods in the listed phases
	// will have endpoints created. Valid values are: Pending, Running, Succeeded, Failed, Unknown.
	// Default is ["Running"] to maintain backward compatibility.
	// To observe init containers, include "Pending" since init containers run during that phase.
	ObservePodPhases []string `mapstructure:"observe_pod_phases"`
	// ObserveInitContainers determines whether to report init container endpoints.
	// Only effective when ObservePods is true. To observe running init containers,
	// "Pending" must be included in ObservePodPhases. `false` by default.
	ObserveInitContainers bool `mapstructure:"observe_init_containers"`
	// InitContainerTerminatedTTL controls how long after termination an init container
	// endpoint remains observable. Running init containers are always observed.
	// Terminated init containers are only observed if they terminated within this duration.
	// This prevents keeping receivers open indefinitely for completed init containers.
	// Default is 15 minutes.
	InitContainerTerminatedTTL time.Duration `mapstructure:"init_container_terminated_ttl"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if !cfg.ObservePods && !cfg.ObserveNodes && !cfg.ObserveServices && !cfg.ObserveIngresses {
		return errors.New("one of observe_pods, observe_nodes, observe_services and observe_ingresses must be true")
	}
	return nil
}
