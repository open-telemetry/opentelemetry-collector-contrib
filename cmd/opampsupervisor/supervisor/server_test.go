// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"net"
	"net/http"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/require"
)

func Test_flattenedSettings_toServerSettings(t *testing.T) {
	fs := flattenedSettings{
		endpoint: "localhost",
	}

	serverSettings := fs.toServerSettings()

	require.Equal(t, "localhost", serverSettings.ListenEndpoint)
	require.Nil(t, serverSettings.Listener)
	require.NotNil(t, serverSettings.Callbacks)
}

func Test_flattenedSettings_toServerSettings_listener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	fs := flattenedSettings{
		// endpoint is ignored when a listener is provided.
		endpoint: "localhost:1234",
		listener: ln,
	}

	serverSettings := fs.toServerSettings()

	require.Same(t, ln, serverSettings.Listener)
	require.Empty(t, serverSettings.ListenEndpoint)
}

func Test_flattenedSettings_OnConnected(t *testing.T) {
	onConnectedFuncCalled := false
	fs := flattenedSettings{
		onConnected: func(serverTypes.Connection) {
			onConnectedFuncCalled = true
		},
	}

	fs.OnConnected(t.Context(), &mockConn{})

	require.True(t, onConnectedFuncCalled)
}

func Test_flattenedSettings_OnConnecting(t *testing.T) {
	t.Run("accept connection", func(t *testing.T) {
		onConnectingFuncCalled := false
		fs := flattenedSettings{
			onConnecting: func(*http.Request) (shouldConnect bool, rejectStatusCode int) {
				onConnectingFuncCalled = true
				return true, 0
			},
		}

		cr := fs.OnConnecting(&http.Request{})

		require.True(t, onConnectingFuncCalled)
		require.True(t, cr.Accept)
		require.NotNil(t, cr.ConnectionCallbacks)
	})
	t.Run("do not accept connection", func(t *testing.T) {
		onConnectingFuncCalled := false
		fs := flattenedSettings{
			onConnecting: func(*http.Request) (shouldConnect bool, rejectStatusCode int) {
				onConnectingFuncCalled = true
				return false, 500
			},
		}

		cr := fs.OnConnecting(&http.Request{})

		require.True(t, onConnectingFuncCalled)
		require.False(t, cr.Accept)
		require.Equal(t, 500, cr.HTTPStatusCode)
	})
}

func Test_flattenedSettings_OnMessage(t *testing.T) {
	onMessageFuncCalled := false
	fs := flattenedSettings{
		onMessage: func(serverTypes.Connection, *protobufs.AgentToServer) *protobufs.ServerToAgent {
			onMessageFuncCalled = true
			return &protobufs.ServerToAgent{}
		},
	}

	sta := fs.OnMessage(t.Context(), &mockConn{}, &protobufs.AgentToServer{})

	require.True(t, onMessageFuncCalled)
	require.NotNil(t, sta)
}

func Test_flattenedSettings_OnConnectionClose(t *testing.T) {
	onConnectionCloseFuncCalled := false
	fs := flattenedSettings{
		onConnectionClose: func(serverTypes.Connection) {
			onConnectionCloseFuncCalled = true
		},
	}

	fs.OnConnectionClose(&mockConn{})

	require.True(t, onConnectionCloseFuncCalled)
}
