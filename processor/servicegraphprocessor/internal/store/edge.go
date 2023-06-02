// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor/internal/store"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type ConnectionType string

const (
	Unknown         ConnectionType = ""
	MessagingSystem ConnectionType = "messaging_system"
	Database        ConnectionType = "database"
	VirtualNode     ConnectionType = "virtual_node"
)

// Edge is an Edge between two nodes in the graph
type Edge struct {
	Key Key

	TraceID                            pcommon.TraceID
	ConnectionType                     ConnectionType
	ServerService, ClientService       string
	ServerLatencySec, ClientLatencySec float64

	// If either the client or the server spans have status code error,
	// the Edge will be considered as failed.
	Failed bool

	// Additional dimension to add to the metrics
	Dimensions map[string]string

	// expiration is the time at which the Edge expires, expressed as Unix time
	expiration time.Time

	Peer map[string]string
}

func newEdge(key Key, ttl time.Duration) *Edge {
	return &Edge{
		Key:        key,
		Dimensions: make(map[string]string),
		expiration: time.Now().Add(ttl),
		Peer:       make(map[string]string),
	}
}

// isComplete returns true if the corresponding client and server
// pair spans have been processed for the given Edge
func (e *Edge) isComplete() bool {
	return len(e.ClientService) != 0 && len(e.ServerService) != 0
}

func (e *Edge) isExpired() bool {
	return time.Now().After(e.expiration)
}
