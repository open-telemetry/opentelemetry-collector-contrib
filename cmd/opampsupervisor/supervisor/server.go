// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"net/http"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
)

type flattenedSettings struct {
	onMessageFunc    func(conn serverTypes.Connection, message *protobufs.AgentToServer)
	onConnectingFunc func(request *http.Request)
	endpoint         string
}

func newServerSettings(fs flattenedSettings) server.StartSettings {
	return server.StartSettings{
		Settings: server.Settings{
			Callbacks: server.CallbacksStruct{
				OnConnectingFunc: func(request *http.Request) serverTypes.ConnectionResponse {
					if fs.onConnectingFunc != nil {
						fs.onConnectingFunc(request)
					}
					return serverTypes.ConnectionResponse{
						Accept: true,
						ConnectionCallbacks: server.ConnectionCallbacksStruct{
							OnMessageFunc: func(conn serverTypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
								if fs.onMessageFunc != nil {
									fs.onMessageFunc(conn, message)
								}

								return &protobufs.ServerToAgent{}
							},
						},
					}
				},
			},
		},
		ListenEndpoint: fs.endpoint,
	}
}
