// Copyright The OpenTelemetry Authors
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

package observer // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"

import (
	"errors"
	"fmt"
	"reflect"
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
	// K8sNodeType is a Kubernetes Node endpoint.
	K8sNodeType EndpointType = "k8s.node"
	// HostPortType is a hostport endpoint.
	HostPortType EndpointType = "hostport"
	// ContainerType is a container endpoint.
	ContainerType EndpointType = "container"
)

var (
	_ EndpointDetails = (*Pod)(nil)
	_ EndpointDetails = (*Port)(nil)
	_ EndpointDetails = (*K8sNode)(nil)
	_ EndpointDetails = (*HostPort)(nil)
	_ EndpointDetails = (*Container)(nil)
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
	// Target is an IP address or hostname of the endpoint. It can also be a hostname/ip:port pair.
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
	env["type"] = string(e.Details.Type())
	env["id"] = string(e.ID)

	return env, nil
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("Endpoint{ID: %v, Target: %v, Details: %T%+v}", e.ID, e.Target, e.Details, e.Details)
}

func (e Endpoint) equals(other Endpoint) bool {
	switch {
	case e.ID != other.ID:
		return false
	case e.Target != other.Target:
		return false
	case e.Details == nil && other.Details != nil:
		return false
	case other.Details == nil && e.Details != nil:
		return false
	case e.Details == nil && other.Details == nil:
		return true
	case e.Details.Type() != other.Details.Type():
		return false
	default:
		return reflect.DeepEqual(e.Details.Env(), other.Details.Env())
	}
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

// Container is a discovered container
type Container struct {
	// Name is the primary name of the container
	Name string
	// Image is the name of the container image
	Image string
	// Tag is the container image tag, e.g. '0.1'
	Tag string
	// Port is the exposed port of container
	Port uint16
	// AlternatePort is the exposed port accessed through some kind of redirection,
	// such as Docker port redirection
	AlternatePort uint16
	// Command used to invoke the process using the Endpoint.
	Command string
	// ContainerID is the id of the container exposing the Endpoint.
	ContainerID string
	// Host is the hostname/ip address of the Endpoint.
	Host string
	// Transport is the transport protocol used by the Endpoint. (TCP or UDP).
	Transport Transport
	// Labels is a map of user-specified metadata on the container.
	Labels map[string]string
}

func (c *Container) Env() EndpointEnv {
	return map[string]interface{}{
		"name":           c.Name,
		"image":          c.Image,
		"tag":            c.Tag,
		"port":           c.Port,
		"alternate_port": c.AlternatePort,
		"command":        c.Command,
		"container_id":   c.ContainerID,
		"host":           c.Host,
		"transport":      c.Transport,
		"labels":         c.Labels,
	}
}

func (c *Container) Type() EndpointType {
	return ContainerType
}

// K8sNode represents a Kubernetes Node object:
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/k8s.md#node
type K8sNode struct {
	// Name is the name of the Kubernetes Node.
	Name string
	// UID is the unique ID for the node
	UID string
	// Hostname is the node hostname as reported by the status object
	Hostname string
	// ExternalIP is the node's external IP address as reported by the Status object
	ExternalIP string
	// InternalIP is the node internal IP address as reported by the Status object
	InternalIP string
	// ExternalDNS is the node's external DNS record as reported by the Status object
	ExternalDNS string
	// InternalDNS is the node's internal DNS record as reported by the Status object
	InternalDNS string
	// Annotations is an arbitrary key-value map of non-identifying, user-specified node metadata
	Annotations map[string]string
	// Labels is the map of identifying, user-specified node metadata
	Labels map[string]string
	// KubeletEndpointPort is the node status object's DaemonEndpoints.KubeletEndpoint.Port value
	KubeletEndpointPort uint16
}

func (n *K8sNode) Env() EndpointEnv {
	return map[string]interface{}{
		"name":                  n.Name,
		"uid":                   n.UID,
		"annotations":           n.Annotations,
		"labels":                n.Labels,
		"hostname":              n.Hostname,
		"external_ip":           n.ExternalIP,
		"internal_ip":           n.InternalIP,
		"external_dns":          n.ExternalDNS,
		"internal_dns":          n.InternalDNS,
		"kubelet_endpoint_port": n.KubeletEndpointPort,
	}
}

func (n *K8sNode) Type() EndpointType {
	return K8sNodeType
}
