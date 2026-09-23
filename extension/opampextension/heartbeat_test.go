// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	servertypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

// heartbeatTestServer offers a heartbeat interval in its first response and
// counts the messages it receives after that.
type heartbeatTestServer struct {
	srv server.OpAMPServer

	mu           sync.Mutex
	offered      bool
	afterOffer   int
	offeredCh    chan struct{}
	intervalSecs uint64
}

func startHeartbeatTestServer(t *testing.T, intervalSecs uint64) *heartbeatTestServer {
	t.Helper()
	h := &heartbeatTestServer{
		srv:          server.New(nil),
		offeredCh:    make(chan struct{}),
		intervalSecs: intervalSecs,
	}

	settings := server.StartSettings{
		Settings: server.Settings{
			Callbacks: servertypes.Callbacks{
				OnConnecting: func(_ *http.Request) servertypes.ConnectionResponse {
					return servertypes.ConnectionResponse{
						Accept: true,
						ConnectionCallbacks: servertypes.ConnectionCallbacks{
							OnMessage: h.onMessage,
						},
					}
				},
			},
		},
		ListenEndpoint: "127.0.0.1:0",
		ListenPath:     "/v1/opamp",
	}
	require.NoError(t, h.srv.Start(settings))
	return h
}

// stop is deferred rather than registered with t.Cleanup because t.Context()
// is already cancelled when cleanups run.
func (h *heartbeatTestServer) stop(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	assert.NoError(t, h.srv.Stop(ctx))
}

func (h *heartbeatTestServer) endpoint() string {
	return "ws://" + h.srv.Addr().String() + "/v1/opamp"
}

func (h *heartbeatTestServer) onMessage(_ context.Context, _ servertypes.Connection, msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
	h.mu.Lock()
	defer h.mu.Unlock()

	resp := &protobufs.ServerToAgent{InstanceUid: msg.GetInstanceUid()}
	if !h.offered {
		h.offered = true
		resp.ConnectionSettings = &protobufs.ConnectionSettingsOffers{
			Hash: []byte("heartbeat-offer"),
			Opamp: &protobufs.OpAMPConnectionSettings{
				HeartbeatIntervalSeconds: h.intervalSecs,
			},
		}
		close(h.offeredCh)
		return resp
	}
	h.afterOffer++
	return resp
}

func (h *heartbeatTestServer) messagesAfterOffer() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.afterOffer
}

// TestReportsHeartbeatNegotiatedInterval checks that the offered heartbeat
// interval is applied only when reports_heartbeat is enabled.
func TestReportsHeartbeatNegotiatedInterval(t *testing.T) {
	const offeredInterval = 1 // seconds

	for _, tc := range []struct {
		name             string
		reportsHeartbeat bool
	}{
		{name: "enabled", reportsHeartbeat: true},
		{name: "disabled", reportsHeartbeat: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := startHeartbeatTestServer(t, offeredInterval)
			defer h.stop(t)

			cfg := createDefaultConfig().(*Config)
			cfg.Server.WS = &commonFields{Endpoint: h.endpoint()}
			cfg.Capabilities.ReportsHeartbeat = tc.reportsHeartbeat
			cfg.Capabilities.ReportsHealth = false
			cfg.Capabilities.ReportsAvailableComponents = false

			set := extensiontest.NewNopSettings(extensiontest.NopType)
			o, err := newOpampAgent(cfg, set)
			require.NoError(t, err)
			require.NoError(t, o.Start(t.Context(), componenttest.NewNopHost()))
			defer func() { assert.NoError(t, o.Shutdown(t.Context())) }()

			select {
			case <-h.offeredCh:
			case <-time.After(10 * time.Second):
				t.Fatal("agent never connected to the test server")
			}

			if tc.reportsHeartbeat {
				require.Eventually(t, func() bool {
					return h.messagesAfterOffer() >= 2
				}, 10*time.Second, 100*time.Millisecond,
					"expected heartbeats at the offered interval")
				return
			}

			// The default interval is 30s, so nothing should arrive in this window.
			require.Never(t, func() bool {
				return h.messagesAfterOffer() > 0
			}, 3*time.Second, 100*time.Millisecond,
				"offered interval applied without reports_heartbeat")
		})
	}
}
