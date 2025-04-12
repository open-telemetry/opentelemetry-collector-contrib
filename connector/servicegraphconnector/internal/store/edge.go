// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/store"

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

type VirtualNodeLabel string

const (
	UnknownVirtualNode VirtualNodeLabel = ""
	ClientVirtualNode  VirtualNodeLabel = "client"
	ServerVirtualNode  VirtualNodeLabel = "server"
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

	// Peer is a map of peer attributes to be used for virtual node matching
	Peer map[string]string

	// VirtualNodeLabel is an optional label to be added to the spans
	VirtualNodeLabel VirtualNodeLabel
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
