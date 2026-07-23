// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"
)

// mockOpAMPClient implements the minimal client.OpAMPClient interface for testing.
type mockOpAMPClient struct {
	mu              sync.Mutex
	sentMessages    []*protobufs.CustomMessage
	sendErr         error
	sendingChanOpen bool
}

func (m *mockOpAMPClient) SendCustomMessage(message *protobufs.CustomMessage) (chan struct{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.sentMessages = append(m.sentMessages, message)
	ch := make(chan struct{})
	close(ch)
	return ch, nil
}

// Satisfy the rest of the client.OpAMPClient interface with no-ops.
func (m *mockOpAMPClient) Start(_ context.Context, _ interface{}) error { return nil }
func (m *mockOpAMPClient) Stop(_ context.Context) error                 { return nil }
func (m *mockOpAMPClient) SetAgentDescription(_ *protobufs.AgentDescription) error {
	return nil
}
func (m *mockOpAMPClient) AgentDescription() *protobufs.AgentDescription { return nil }
func (m *mockOpAMPClient) SetHealth(_ *protobufs.ComponentHealth) error   { return nil }
func (m *mockOpAMPClient) UpdateEffectiveConfig(_ context.Context) error  { return nil }
func (m *mockOpAMPClient) SetRemoteConfigStatus(_ *protobufs.RemoteConfigStatus) error {
	return nil
}
func (m *mockOpAMPClient) SetPackageStatuses(_ *protobufs.PackageStatuses) error       { return nil }
func (m *mockOpAMPClient) SetCapabilities(_ *protobufs.AgentCapabilities) error        { return nil }
func (m *mockOpAMPClient) SetCustomCapabilities(_ *protobufs.CustomCapabilities) error { return nil }
func (m *mockOpAMPClient) SetFlags(_ protobufs.AgentToServerFlags) error               { return nil }
func (m *mockOpAMPClient) SetAvailableComponents(_ *protobufs.AvailableComponents) error {
	return nil
}

// mockConnection implements serverTypes.Connection for testing.
type mockConnection struct {
	mu       sync.Mutex
	messages []*protobufs.ServerToAgent
}

func (c *mockConnection) Send(_ context.Context, msg *protobufs.ServerToAgent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
	return nil
}

func (c *mockConnection) Disconnect() error { return nil }

func (c *mockConnection) Connection() serverTypes.Connection { return c }

func TestGateway_OnMessage_ForwardsUpstream(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockClient := &mockOpAMPClient{}

	gw := NewGateway(logger, GatewayConfig{
		Enabled:        true,
		ListenEndpoint: "localhost:0",
		MaxAgents:      10,
	}, mockClient)

	conn := &mockConnection{}
	instanceUID := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}

	msg := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		Health: &protobufs.ComponentHealth{
			Healthy: true,
		},
	}

	resp := gw.onMessage(context.Background(), conn, msg)
	assert.NotNil(t, resp)

	// Verify the message was forwarded upstream
	mockClient.mu.Lock()
	require.Len(t, mockClient.sentMessages, 1)
	sent := mockClient.sentMessages[0]
	mockClient.mu.Unlock()

	assert.Equal(t, gatewayCapability, sent.Capability)
	// Payload = instanceUID (16 bytes) + proto-marshaled AgentToServer
	assert.True(t, len(sent.Data) > 16)
	assert.Equal(t, instanceUID, sent.Data[:16])

	// Verify the agent was registered
	assert.Equal(t, 1, gw.AgentCount())
}

func TestGateway_OnConnectionClose_RemovesAgent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockClient := &mockOpAMPClient{}

	gw := NewGateway(logger, GatewayConfig{
		Enabled:        true,
		ListenEndpoint: "localhost:0",
		MaxAgents:      10,
	}, mockClient)

	conn := &mockConnection{}
	instanceUID := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}

	msg := &protobufs.AgentToServer{InstanceUid: instanceUID}
	gw.onMessage(context.Background(), conn, msg)
	assert.Equal(t, 1, gw.AgentCount())

	gw.onConnectionClose(conn)
	assert.Equal(t, 0, gw.AgentCount())
}

func TestGateway_MaxAgents_RejectsOverLimit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockClient := &mockOpAMPClient{}

	gw := NewGateway(logger, GatewayConfig{
		Enabled:        true,
		ListenEndpoint: "localhost:0",
		MaxAgents:      2,
	}, mockClient)

	// Connect two agents
	for i := range 2 {
		conn := &mockConnection{}
		uid := make([]byte, 16)
		uid[0] = byte(i)
		gw.onMessage(context.Background(), conn, &protobufs.AgentToServer{InstanceUid: uid})
	}
	assert.Equal(t, 2, gw.AgentCount())

	// Third connection should be rejected
	resp := gw.onConnecting(nil)
	assert.False(t, resp.Accept)
	assert.Equal(t, 503, resp.HTTPStatusCode)
}

func TestGateway_OnCustomMessageFromServer_RoutesToAgent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockClient := &mockOpAMPClient{}

	gw := NewGateway(logger, GatewayConfig{
		Enabled:        true,
		ListenEndpoint: "localhost:0",
		MaxAgents:      10,
	}, mockClient)

	conn := &mockConnection{}
	instanceUID := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}

	// Register the agent
	gw.onMessage(context.Background(), conn, &protobufs.AgentToServer{InstanceUid: instanceUID})

	// Simulate a response from the upstream server
	serverResponse := &protobufs.ServerToAgent{
		InstanceUid: instanceUID,
	}
	responseData, err := proto.Marshal(serverResponse)
	require.NoError(t, err)

	// Format: 16-byte hex-decoded UID + proto payload
	// But OnCustomMessageFromServer expects raw bytes (not hex), so use raw UID
	payload := append(instanceUID, responseData...)

	gw.OnCustomMessageFromServer(&protobufs.CustomMessage{
		Capability: gatewayCapability,
		Data:       payload,
	})

	// Verify the response was sent to the downstream agent
	conn.mu.Lock()
	require.Len(t, conn.messages, 1)
	assert.Equal(t, instanceUID, conn.messages[0].InstanceUid)
	conn.mu.Unlock()
}

func TestGateway_OnCustomMessageFromServer_UnknownAgent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockClient := &mockOpAMPClient{}

	gw := NewGateway(logger, GatewayConfig{
		Enabled:        true,
		ListenEndpoint: "localhost:0",
		MaxAgents:      10,
	}, mockClient)

	unknownUID := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11,
		0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}

	serverResponse := &protobufs.ServerToAgent{InstanceUid: unknownUID}
	responseData, _ := proto.Marshal(serverResponse)
	payload := append(unknownUID, responseData...)

	// Should not panic, just log a warning
	gw.OnCustomMessageFromServer(&protobufs.CustomMessage{
		Capability: gatewayCapability,
		Data:       payload,
	})

	_ = hex.EncodeToString(unknownUID) // just to use the import
}

func TestGateway_DefaultMaxAgents(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockClient := &mockOpAMPClient{}

	gw := NewGateway(logger, GatewayConfig{
		Enabled:        true,
		ListenEndpoint: "localhost:0",
		MaxAgents:      0, // should default to 100
	}, mockClient)

	assert.Equal(t, 100, gw.maxAgents)
}
