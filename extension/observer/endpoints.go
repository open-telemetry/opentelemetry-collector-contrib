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

import (
	"errors"
	"fmt"
)

type (
	// EndpointID unique identifies an endpoint per-observer instance.
	EndpointID string
	// EndpointEnv is a map of endpoint attributes.
	EndpointEnv map[string]interface{}
	// EndpointType is a type of an endpoint like a port or pod.
	EndpointType string
)

const (
	// PortType is a port endpoint.
	PortType EndpointType = "port"
	// PodType is a pod endpoint.
	PodType EndpointType = "pod"
	// HostPortType is a hostport endpoint.
	HostPortType EndpointType = "hostport"
)

var (
	// EndpointTypes is a map of all known endpoint types.
	EndpointTypes = map[EndpointType]bool{
		PortType:     true,
		PodType:      true,
		HostPortType: true,
	}

	_ EndpointDetails = (*Pod)(nil)
	_ EndpointDetails = (*Port)(nil)
	_ EndpointDetails = (*HostPort)(nil)
)

// EndpointDetails provides additional context about an endpoint such as a Pod or Port.
type EndpointDetails interface {
	Env() EndpointEnv
	Type() EndpointType
}

// Endpoint is a service that can be contacted remotely.
type Endpoint struct {
	// ID uniquely identifies this endpoint.
	ID EndpointID
	// Target is an IP address or hostname of the endpoint.
	Target string
	// Details contains additional context about the endpoint such as a Pod or Port.
	Details EndpointDetails
}

// Env converts an endpoint into a map suitable for expr evaluation.
func (e *Endpoint) Env() (EndpointEnv, error) {
	if e.Details == nil {
		return nil, errors.New("endpoint is missing details")
	}
	env := e.Details.Env()
	env["endpoint"] = e.Target

	// Populate type field for evaluating rules with `type.port && ...`.
	// Use string instead of EndpointType for rule evaluation.
	types := map[string]bool{}
	for endpointType := range EndpointTypes {
		types[string(endpointType)] = false
	}
	types[string(e.Details.Type())] = true
	env["type"] = types
	return env, nil
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("Endpoint{ID: %v, Target: %v, Details: %T%+v}", e.ID, e.Target, e.Details, e.Details)
}

// Pod is a discovered k8s pod.
type Pod struct {
	// Name of the pod.
	Name string
	// UID is the unique ID in the cluster for the pod.
	UID string
	// Labels is a map of user-specified metadata.
	Labels map[string]string
	// Annotations is a map of user-specified metadata.
	Annotations map[string]string
	// Namespace must be unique for pods with same name.
	Namespace string
}

func (p *Pod) Env() EndpointEnv {
	return map[string]interface{}{
		"uid":         p.UID,
		"name":        p.Name,
		"labels":      p.Labels,
		"annotations": p.Annotations,
		"namespace":   p.Namespace,
	}
}

func (p *Pod) Type() EndpointType {
	return PodType
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

func (p *Port) Env() EndpointEnv {
	return map[string]interface{}{
		"name":      p.Name,
		"port":      p.Port,
		"pod":       p.Pod.Env(),
		"transport": p.Transport,
	}
}

func (p *Port) Type() EndpointType {
	return PortType
}

// HostPort is an endpoint discovered on a host.
type HostPort struct {
	// ProcessName of the process associated to Endpoint.  If host_observer
	// is unable to collect information about process using the
	// Port, this value is an empty string.
	ProcessName string
	// Command used to invoke the process using the Endpoint.
	Command string
	// Port number of the endpoint.
	Port uint16
	// Transport is the transport protocol used by the Endpoint. (TCP or UDP).
	Transport Transport
	// IsIPv6 indicates whether or not the Endpoint is IPv6.
	IsIPv6 bool
}

func (h *HostPort) Env() EndpointEnv {
	return map[string]interface{}{
		"process_name": h.ProcessName,
		"command":      h.Command,
		"is_ipv6":      h.IsIPv6,
		"port":         h.Port,
		"transport":    h.Transport,
	}
}

func (h *HostPort) Type() EndpointType {
	return HostPortType
}
