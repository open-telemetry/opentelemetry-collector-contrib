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
	"fmt"
)

// Protocol defines network protocol for container ports.
type Protocol string

const (
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP Protocol = "TCP"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP Protocol = "UDP"
)

// Endpoint is a service that can be contacted remotely.
type Endpoint interface {
	// ID uniquely identifies this endpoint.
	ID() string
	// Target is an address or hostname of the endpoint.
	Target() string
	// String pretty formats the endpoint.
	String() string
	// Labels is a map of arbitrary metadata.
	Labels() map[string]string
}

// endpointBase is common endpoint data used across all endpoint types.
type endpointBase struct {
	id     string
	target string
	labels map[string]string
}

func (e *endpointBase) ID() string {
	return e.id
}

func (e *endpointBase) Target() string {
	return e.target
}

func (e *endpointBase) Labels() map[string]string {
	return e.labels
}

// HostEndpoint is an endpoint that just has a target but no identifying port information.
type HostEndpoint struct {
	endpointBase
}

func (h *HostEndpoint) String() string {
	return fmt.Sprintf("HostEndpoint{id: %v, Target: %v, Labels: %v}", h.ID(), h.target, h.labels)
}

// NewHostEndpoint creates a HostEndpoint
func NewHostEndpoint(id string, target string, labels map[string]string) *HostEndpoint {
	return &HostEndpoint{endpointBase{
		id:     id,
		target: target,
		labels: labels,
	}}
}

var _ Endpoint = (*HostEndpoint)(nil)

// PortEndpoint is an endpoint that has a target as well as a port.
type PortEndpoint struct {
	endpointBase
	Port int32
}

func (p *PortEndpoint) String() string {
	return fmt.Sprintf("PortEndpoint{ID: %v, Target: %v, Port: %d, Labels: %v}", p.ID(), p.target, p.Port, p.labels)
}

// NewPortEndpoint creates a PortEndpoint.
func NewPortEndpoint(id string, target string, port int32, labels map[string]string) *PortEndpoint {
	return &PortEndpoint{endpointBase: endpointBase{
		id:     id,
		target: target,
		labels: labels,
	}, Port: port}
}

var _ Endpoint = (*PortEndpoint)(nil)

// Observable is an interface that provides notification of endpoint changes.
type Observable interface {
	// TODO: Stopping
	// ListAndWatch provides initial state sync as well as change notification.
	ListAndWatch(notify Notify)
}

// Notify is the callback for Observer events.
type Notify interface {
	// OnAdd is called once or more initially for state sync as well as when further endpoints are added.
	OnAdd(added []Endpoint)
	// OnRemove is called when one or more endpoints are removed.
	OnRemove(removed []Endpoint)
	// OnChange is called when one ore more endpoints are modified.
	OnChange(changed []Endpoint)
}
