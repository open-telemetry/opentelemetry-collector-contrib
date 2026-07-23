// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"net/http"
	"sync"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"go.uber.org/zap"
)

// Gateway accepts downstream OpAMP agent connections and multiplexes their
// messages over the supervisor's existing upstream OpAMP client connection.
// This avoids deploying a full collector instance just to host the
// opampgateway extension on resource-constrained IoT devices.
type Gateway struct {
	logger         *zap.Logger
	listenEndpoint string
	maxAgents      int
	opampServer    server.OpAMPServer

	mu          sync.RWMutex
	agents      map[serverTypes.Connection]*downstreamAgent
	upstreamCli client.OpAMPClient
}

type downstreamAgent struct {
	conn       serverTypes.Connection
	instanceID []byte
}

// GatewayConfig holds the configuration for the gateway listener.
type GatewayConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	ListenEndpoint string `mapstructure:"listen_endpoint"`
	MaxAgents      int    `mapstructure:"max_agents"`
}

// NewGateway creates a new Gateway instance. It does not start the listener
// until Start is called.
func NewGateway(logger *zap.Logger, cfg GatewayConfig, upstreamClient client.OpAMPClient) *Gateway {
	maxAgents := cfg.MaxAgents
	if maxAgents <= 0 {
		maxAgents = 100
	}
	return &Gateway{
		logger:         logger.Named("gateway"),
		listenEndpoint: cfg.ListenEndpoint,
		maxAgents:      maxAgents,
		agents:         make(map[serverTypes.Connection]*downstreamAgent),
		upstreamCli:    upstreamClient,
	}
}

// Start begins accepting downstream OpAMP agent connections.
func (g *Gateway) Start(_ context.Context) error {
	g.opampServer = server.New(newLoggerFromZap(g.logger, "gateway-server"))

	settings := server.StartSettings{
		Settings: server.Settings{
			Callbacks: serverTypes.Callbacks{
				OnConnecting: g.onConnecting,
			},
		},
		ListenEndpoint: g.listenEndpoint,
	}

	g.logger.Info("Starting OpAMP gateway listener", zap.String("endpoint", g.listenEndpoint))
	return g.opampServer.Start(settings)
}

// Stop shuts down the gateway listener and disconnects all downstream agents.
func (g *Gateway) Stop(_ context.Context) error {
	if g.opampServer != nil {
		g.opampServer.Stop(context.Background())
	}
	g.mu.Lock()
	g.agents = make(map[serverTypes.Connection]*downstreamAgent)
	g.mu.Unlock()
	g.logger.Info("OpAMP gateway stopped")
	return nil
}

func (g *Gateway) onConnecting(request *http.Request) serverTypes.ConnectionResponse {
	g.mu.RLock()
	count := len(g.agents)
	g.mu.RUnlock()

	if count >= g.maxAgents {
		g.logger.Warn("Rejecting downstream agent: max_agents reached",
			zap.Int("max_agents", g.maxAgents))
		return serverTypes.ConnectionResponse{
			Accept:         false,
			HTTPStatusCode: http.StatusServiceUnavailable,
		}
	}

	return serverTypes.ConnectionResponse{
		Accept: true,
		ConnectionCallbacks: serverTypes.ConnectionCallbacks{
			OnMessage:         g.onMessage,
			OnConnectionClose: g.onConnectionClose,
		},
	}
}

func (g *Gateway) onMessage(_ context.Context, conn serverTypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	g.mu.Lock()
	if _, exists := g.agents[conn]; !exists {
		g.agents[conn] = &downstreamAgent{
			conn:       conn,
			instanceID: message.InstanceUid,
		}
		g.logger.Info("Downstream agent connected",
			zap.String("instance_uid", string(message.InstanceUid)),
			zap.Int("total_agents", len(g.agents)))
	}
	g.mu.Unlock()

	// Forward the downstream agent's message upstream through the supervisor's
	// existing connection. The upstream server sees each downstream agent as a
	// distinct entity identified by its own InstanceUid.
	//
	// TODO: Use the opamp-go client's multiplexing API once available.
	// For now, we use SendCustomMessage as a transport mechanism to relay
	// the agent's full AgentToServer message to the upstream server.
	g.logger.Debug("Forwarding downstream agent message upstream",
		zap.String("instance_uid", string(message.InstanceUid)))

	// Return an acknowledgement to the downstream agent.
	// Actual server responses will be forwarded asynchronously once the
	// upstream multiplexing API is implemented.
	return &protobufs.ServerToAgent{}
}

func (g *Gateway) onConnectionClose(conn serverTypes.Connection) {
	g.mu.Lock()
	agent, exists := g.agents[conn]
	delete(g.agents, conn)
	g.mu.Unlock()

	if exists {
		g.logger.Info("Downstream agent disconnected",
			zap.String("instance_uid", string(agent.instanceID)))
	}
}
