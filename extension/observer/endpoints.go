// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package observer

import "fmt"

type EndpointID string

// Endpoint is a service that can be contacted remotely.
type Endpoint struct {
	// ID uniquely identifies this endpoint.
	ID EndpointID
	// Target is an IP address or hostname of the endpoint.
	Target string
	// Details contains additional context about the endpoint such as a Pod or Port.
	Details interface{}
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("Endpoint{ID: %v, Target: %v, Details: %T%+v}", e.ID, e.Target, e.Details, e.Details)
}

// Pod is a discovered k8s pod.
type Pod struct {
	// Name of the pod.
	Name string
	// Labels is a map of user-specified metadata.
	Labels map[string]string
	// Annotations is a map of user-specified metadata.
	Annotations map[string]string
}

// Port is an endpoint that has a target as well as a port.
type Port struct {
	// Name is the name of the container port.
	Name string
	// Pod is the k8s pod in which the container is running.
	Pod Pod
	// Port number of the endpoint.
	Port uint16
	// Transport is the transport protocol used by the Endpoint. (TCP or UDP).
	Transport Transport
}

// HostPort is an endpoint discovered on a host.
type HostPort struct {
	// Name of the process associated to Endpoint.  If host_observer
	// is unable to collect information about process using the
	// Port, this value is an empty string.
	Name string
	// Command used to invoke the process using the Endpoint.
	Command string
	// Port number of the endpoint.
	Port uint16
	// Transport is the transport protocol used by the Endpoint. (TCP or UDP).
	Transport Transport
	// IsIPv6 indicates whether or not the Endpoint is IPv6.
	IsIPv6 bool
}

type EndpointEnv map[string]interface{}

// EndpointToEnv converts an endpoint into a map suitable for expr evaluation.
func EndpointToEnv(endpoint Endpoint) (EndpointEnv, error) {
	ruleTypes := map[string]interface{}{
		"port": false,
		"pod":  false,
	}

	switch o := endpoint.Details.(type) {
	case Pod:
		ruleTypes["pod"] = true
		return map[string]interface{}{
			"type":        ruleTypes,
			"endpoint":    endpoint.Target,
			"name":        o.Name,
			"labels":      o.Labels,
			"annotations": o.Annotations,
		}, nil
	case Port:
		ruleTypes["port"] = true
		return map[string]interface{}{
			"type":     ruleTypes,
			"endpoint": endpoint.Target,
			"name":     o.Name,
			"port":     o.Port,
			"pod": map[string]interface{}{
				"name":        o.Pod.Name,
				"labels":      o.Pod.Labels,
				"annotations": o.Pod.Annotations,
			},
			"transport": o.Transport,
		}, nil
	case HostPort:
		ruleTypes["port"] = true
		return map[string]interface{}{
			"type":      ruleTypes,
			"endpoint":  endpoint.Target,
			"name":      o.Name,
			"command":   o.Command,
			"is_ipv6":   o.IsIPv6,
			"port":      o.Port,
			"transport": o.Transport,
		}, nil

	default:
		return nil, fmt.Errorf("unknown endpoint details type %T", endpoint.Details)
	}
}
