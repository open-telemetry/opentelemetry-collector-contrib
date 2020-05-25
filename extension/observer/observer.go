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
	// ProtocolUnknown is some other protocol or it is unknown.
	ProtocolUnknown Protocol = "Unknown"
)

// Endpoint is a service that can be contacted remotely.
type Endpoint struct {
	// ID uniquely identifies this endpoint.
	ID string
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
}

// Port is an endpoint that has a target as well as a port.
type Port struct {
	Name     string
	Pod      Pod
	Port     uint16
	Protocol Protocol
}

// Observable is an interface that provides notification of endpoint changes.
type Observable interface {
	// TODO: Stopping.
	// ListAndWatch provides initial state sync as well as change notification.
	// notify.OnAdd will be called one or more times if there are endpoints discovered.
	// (It would not be called if there are no endpoints present.) The endpoint synchronization
	// happens asynchronously to this call.
	ListAndWatch(notify Notify)
}

// Notify is the callback for Observer events.
type Notify interface {
	// OnAdd is called once or more initially for state sync as well as when further endpoints are added.
	OnAdd(added []Endpoint)
	// OnRemove is called when one or more endpoints are removed.
	OnRemove(removed []Endpoint)
	// OnChange is called when one ore more endpoints are modified but the identity is not changed
	// (e.g. labels).
	OnChange(changed []Endpoint)
}
