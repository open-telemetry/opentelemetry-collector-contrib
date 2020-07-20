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

// TransportProtocol defines network protocol for container ports.
type TransportProtocol string

const (
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP TransportProtocol = "TCP"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP TransportProtocol = "UDP"
	// ProtocolUnknown is some other protocol or it is unknown.
	ProtocolUnknown TransportProtocol = "Unknown"
)

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
