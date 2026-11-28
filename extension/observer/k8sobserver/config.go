// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"errors"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config defines configuration for k8s attributes processor.
type Config struct {
	APIConfig k8sconfig.APIConfig `mapstructure:",squash"`

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
	// ObserveInitContainers determines whether to also report pod container and port endpoints for init
	// containers, native sidecars included. Requires ObservePods. `false` by default.
	ObserveInitContainers bool `mapstructure:"observe_init_containers"`
	// ObserveEphemeralContainers determines whether to also report pod container endpoints for ephemeral
	// containers, such as the ones `kubectl debug` adds. Requires ObservePods. `false` by default.
	ObserveEphemeralContainers bool `mapstructure:"observe_ephemeral_containers"`
	// ObserveTerminatedContainers determines whether to also report pod container endpoints for containers
	// that ran and are not running now, such as a container that crashed or the container of a finished
	// Job, in a pod of any phase. Port endpoints are still reported for running containers only.
	// Requires ObservePods. `false` by default.
	ObserveTerminatedContainers bool `mapstructure:"observe_terminated_containers"`
	// TerminatedContainerTTL is how long the endpoint of a container that is not running stays reported
	// after the container finished, when ObserveTerminatedContainers is set. Zero keeps it until the pod
	// is deleted. `15m` by default.
	TerminatedContainerTTL time.Duration `mapstructure:"terminated_container_ttl"`
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
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if !cfg.ObservePods && !cfg.ObserveNodes && !cfg.ObserveServices && !cfg.ObserveIngresses {
		return errors.New("one of observe_pods, observe_nodes, observe_services and observe_ingresses must be true")
	}
	if cfg.ObserveInitContainers && !cfg.ObservePods {
		return errors.New("observe_init_containers requires observe_pods")
	}
	if cfg.ObserveEphemeralContainers && !cfg.ObservePods {
		return errors.New("observe_ephemeral_containers requires observe_pods")
	}
	if cfg.ObserveTerminatedContainers && !cfg.ObservePods {
		return errors.New("observe_terminated_containers requires observe_pods")
	}
	if cfg.TerminatedContainerTTL < 0 {
		return errors.New("terminated_container_ttl must not be negative")
	}
	return nil
}
