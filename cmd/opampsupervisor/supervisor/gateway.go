// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"encoding/hex"
	"net/http"
	"sync"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const gatewayCapability = "io.opentelemetry.opamp.gateway"

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
	agentsByUID map[string]*downstreamAgent
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
		agentsByUID:    make(map[string]*downstreamAgent),
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
	g.agentsByUID = make(map[string]*downstreamAgent)
	g.mu.Unlock()
	g.logger.Info("OpAMP gateway stopped")
	return nil
}

// OnCustomMessageFromServer handles responses from the upstream OpAMP server
// intended for downstream agents. The server wraps ServerToAgent messages in
// a CustomMessage with capability "io.opentelemetry.opamp.gateway". The data
// field contains the proto-marshaled ServerToAgent message prefixed with the
// 16-byte instance UID of the target downstream agent.
func (g *Gateway) OnCustomMessageFromServer(message *protobufs.CustomMessage) {
	if message.GetCapability() != gatewayCapability {
		return
	}
	data := message.GetData()
	if len(data) < 16 {
		g.logger.Warn("Gateway received malformed response: data too short")
		return
	}

	uid := hex.EncodeToString(data[:16])
	payload := data[16:]

	var response protobufs.ServerToAgent
	if err := proto.Unmarshal(payload, &response); err != nil {
		g.logger.Error("Failed to unmarshal upstream response for downstream agent",
			zap.String("instance_uid", uid), zap.Error(err))
		return
	}

	g.mu.RLock()
	agent, exists := g.agentsByUID[uid]
	g.mu.RUnlock()

	if !exists {
		g.logger.Warn("Received response for unknown downstream agent",
			zap.String("instance_uid", uid))
		return
	}

	agent.conn.Send(context.Background(), &response)
	g.logger.Debug("Forwarded upstream response to downstream agent",
		zap.String("instance_uid", uid))
}

// AgentCount returns the number of currently connected downstream agents.
func (g *Gateway) AgentCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.agents)
}

func (g *Gateway) onConnecting(_ *http.Request) serverTypes.ConnectionResponse {
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
	uid := hex.EncodeToString(message.InstanceUid)

	g.mu.Lock()
	if _, exists := g.agents[conn]; !exists {
		agent := &downstreamAgent{
			conn:       conn,
			instanceID: message.InstanceUid,
		}
		g.agents[conn] = agent
		g.agentsByUID[uid] = agent
		g.logger.Info("Downstream agent connected",
			zap.String("instance_uid", uid),
			zap.Int("total_agents", len(g.agents)))
	}
	g.mu.Unlock()

	// Marshal the downstream agent's message and forward it upstream as a
	// CustomMessage. The upstream server (e.g. BindPlane) processes the
	// relayed message and responds via a CustomMessage back to this gateway.
	data, err := proto.Marshal(message)
	if err != nil {
		g.logger.Error("Failed to marshal downstream agent message",
			zap.String("instance_uid", uid), zap.Error(err))
		return &protobufs.ServerToAgent{}
	}

	// Prefix with the instance UID so the response can be routed back.
	payload := append(message.InstanceUid, data...)

	customMsg := &protobufs.CustomMessage{
		Capability: gatewayCapability,
		Data:       payload,
	}

	if _, err := g.upstreamCli.SendCustomMessage(customMsg); err != nil {
		g.logger.Error("Failed to forward downstream message upstream",
			zap.String("instance_uid", uid), zap.Error(err))
	} else {
		g.logger.Debug("Forwarded downstream agent message upstream",
			zap.String("instance_uid", uid))
	}

	// Return empty response synchronously. The actual server response will
	// arrive asynchronously via OnCustomMessageFromServer and be pushed to
	// the downstream agent's connection.
	return &protobufs.ServerToAgent{}
}

func (g *Gateway) onConnectionClose(conn serverTypes.Connection) {
	g.mu.Lock()
	agent, exists := g.agents[conn]
	if exists {
		uid := hex.EncodeToString(agent.instanceID)
		delete(g.agents, conn)
		delete(g.agentsByUID, uid)
		g.logger.Info("Downstream agent disconnected",
			zap.String("instance_uid", uid),
			zap.Int("total_agents", len(g.agents)))
	}
	g.mu.Unlock()
}
