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

// Transport defines protocol for ports.
type Transport string

const (
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP Transport = "TCP"
	// ProtocolTCP4 is the TCP4 protocol.
	ProtocolTCP4 Transport = "TCP4"
	// ProtocolTCP6 is the TCP6 protocol.
	ProtocolTCP6 Transport = "TCP6"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP Transport = "UDP"
	// ProtocolUDP4 is the UDP4 protocol.
	ProtocolUDP4 Transport = "UDP4"
	// ProtocolUDP6 is the UDP6 protocol.
	ProtocolUDP6 Transport = "UDP6"
	// ProtocolUnknown is some other protocol or it is unknown.
	ProtocolUnknown Transport = "Unknown"
)

// Observable is an interface that provides notification of endpoint changes.
type Observable interface {
	// ListAndWatch provides initial state sync as well as change notification.
	// notify. OnAdd will be called one or more times if there are endpoints discovered.
	// (It would not be called if there are no endpoints present.) The endpoint synchronization
	// happens asynchronously to this call.
	ListAndWatch(notify Notify)

	// Unsubscribe stops the previously registered Notify from receiving callback invocations.
	Unsubscribe(notify Notify)
}

type NotifyID string

// Notify is the callback for Observer events.
type Notify interface {
	// ID must be unique for each Notify implementer instance and is for registration purposes.
	ID() NotifyID
	// OnAdd is called once or more initially for state sync as well as when further endpoints are added.
	OnAdd(added []Endpoint)
	// OnRemove is called when one or more endpoints are removed.
	OnRemove(removed []Endpoint)
	// OnChange is called when one or more endpoints are modified but the identity is not changed
	// (e.g. labels).
	OnChange(changed []Endpoint)
}
