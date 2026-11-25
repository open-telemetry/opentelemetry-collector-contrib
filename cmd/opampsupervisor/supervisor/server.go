// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"net"
	"net/http"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
)

type flattenedSettings struct {
	onMessage         func(conn serverTypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent
	onConnecting      func(request *http.Request) (shouldConnect bool, rejectStatusCode int)
	onConnectionClose func(conn serverTypes.Connection)
	// onConnected, if set, is invoked once per accepted connection before any
	// messages are processed. The supervisor uses it to authenticate the peer
	// of a Unix domain socket connection (see authenticateAgentPeer).
	onConnected func(conn serverTypes.Connection)
	endpoint    string
	// listener, if set, is used to serve the OpAMP server instead of opening a
	// TCP listener on endpoint (e.g. a Unix domain socket). The caller owns the
	// listener's lifecycle.
	listener net.Listener
}

func (fs flattenedSettings) toServerSettings() server.StartSettings {
	settings := server.StartSettings{
		Settings: server.Settings{
			Callbacks: serverTypes.Callbacks{
				OnConnecting: fs.OnConnecting,
			},
		},
	}
	// A provided listener (Unix domain socket) takes precedence; opamp-go ignores
	// ListenEndpoint when Listener is set.
	if fs.listener != nil {
		settings.Listener = fs.listener
	} else {
		settings.ListenEndpoint = fs.endpoint
	}
	return settings
}

func (fs flattenedSettings) OnConnecting(request *http.Request) serverTypes.ConnectionResponse {
	if fs.onConnecting != nil {
		shouldConnect, rejectStatusCode := fs.onConnecting(request)
		if !shouldConnect {
			return serverTypes.ConnectionResponse{
				Accept:         false,
				HTTPStatusCode: rejectStatusCode,
			}
		}
	}

	return serverTypes.ConnectionResponse{
		Accept: true,
		ConnectionCallbacks: serverTypes.ConnectionCallbacks{
			OnConnected:       fs.OnConnected,
			OnMessage:         fs.OnMessage,
			OnConnectionClose: fs.OnConnectionClose,
		},
	}
}

func (fs flattenedSettings) OnConnected(_ context.Context, conn serverTypes.Connection) {
	if fs.onConnected != nil {
		fs.onConnected(conn)
	}
}

func (fs flattenedSettings) OnMessage(_ context.Context, conn serverTypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	if fs.onMessage != nil {
		return fs.onMessage(conn, message)
	}

	return &protobufs.ServerToAgent{}
}

func (fs flattenedSettings) OnConnectionClose(conn serverTypes.Connection) {
	if fs.onConnectionClose != nil {
		fs.onConnectionClose(conn)
	}
}
