// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"context"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func Test_composeEffectiveConfig(t *testing.T) {
	acceptsRemoteConfig := true
	s := Supervisor{
		logger:                       zap.NewNop(),
		persistentState:              &persistentState{},
		config:                       config.Supervisor{Capabilities: config.Capabilities{AcceptsRemoteConfig: acceptsRemoteConfig}},
		pidProvider:                  staticPIDProvider(1234),
		hasNewConfig:                 make(chan struct{}, 1),
		agentConfigOwnMetricsSection: &atomic.Value{},
		mergedConfig:                 &atomic.Value{},
		agentHealthCheckEndpoint:     "localhost:8000",
	}

	agentDesc := &atomic.Value{}
	agentDesc.Store(&protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "service.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "otelcol",
					},
				},
			},
		},
	})

	s.agentDescription = agentDesc

	fileLogConfig := `
receivers:
  filelog:
    include: ['/test/logs/input.log']
    start_at: "beginning"

exporters:
  file:
    path: '/test/logs/output.log'

service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [file]`

	require.NoError(t, s.createTemplates())
	require.NoError(t, s.loadAndWriteInitialMergedConfig())

	configChanged, err := s.composeMergedConfig(&protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {
					Body: []byte(fileLogConfig),
				},
			},
		},
	})
	require.NoError(t, err)

	expectedConfig, err := os.ReadFile("../testdata/collector/effective_config.yaml")
	require.NoError(t, err)
	expectedConfig = bytes.ReplaceAll(expectedConfig, []byte("\r\n"), []byte("\n"))

	require.True(t, configChanged)
	require.Equal(t, string(expectedConfig), s.mergedConfig.Load().(string))
}

func Test_onMessage(t *testing.T) {
	t.Run("AgentIdentification - New instance ID is valid", func(t *testing.T) {
		agentDesc := &atomic.Value{}
		agentDesc.Store(&protobufs.AgentDescription{})
		initialID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		newID := uuid.MustParse("018fef3f-14a8-73ef-b63e-3b96b146ea38")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: initialID},
			agentDescription:             agentDesc,
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			opampClient:                  client.NewHTTP(newLoggerFromZap(zap.NewNop())),
		}

		s.onMessage(context.Background(), &types.MessageData{
			AgentIdentification: &protobufs.AgentIdentification{
				NewInstanceUid: newID[:],
			},
		})

		require.Equal(t, newID, s.persistentState.InstanceID)
	})

	t.Run("AgentIdentification - New instance ID is invalid", func(t *testing.T) {
		agentDesc := &atomic.Value{}
		agentDesc.Store(&protobufs.AgentDescription{})

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentDescription:             agentDesc,
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
		}

		s.onMessage(context.Background(), &types.MessageData{
			AgentIdentification: &protobufs.AgentIdentification{
				NewInstanceUid: []byte("invalid-value"),
			},
		})

		require.Equal(t, testUUID, s.persistentState.InstanceID)
	})

	t.Run("CustomMessage - Custom message from server is forwarded to agent", func(t *testing.T) {
		customMessage := &protobufs.CustomMessage{
			Capability: "teapot",
			Type:       "brew",
			Data:       []byte("chamomile"),
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		gotMessage := false
		var agentConn serverTypes.Connection = &mockConn{
			sendFunc: func(_ context.Context, message *protobufs.ServerToAgent) error {
				require.Equal(t, &protobufs.ServerToAgent{
					InstanceUid:   testUUID[:],
					CustomMessage: customMessage,
				}, message)
				gotMessage = true

				return nil
			},
		}

		agentConnAtomic := &atomic.Value{}
		agentConnAtomic.Store(agentConn)

		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    agentConnAtomic,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.onMessage(context.Background(), &types.MessageData{
			CustomMessage: customMessage,
		})

		require.True(t, gotMessage, "Message was not sent to agent")
	})

	t.Run("CustomCapabilities - Custom capabilities from server are forwarded to agent", func(t *testing.T) {
		customCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"coffeemaker", "teapot"},
		}
		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		gotMessage := false
		var agentConn serverTypes.Connection = &mockConn{
			sendFunc: func(_ context.Context, message *protobufs.ServerToAgent) error {
				require.Equal(t, &protobufs.ServerToAgent{
					InstanceUid:        testUUID[:],
					CustomCapabilities: customCapabilities,
				}, message)
				gotMessage = true

				return nil
			},
		}

		agentConnAtomic := &atomic.Value{}
		agentConnAtomic.Store(agentConn)

		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    agentConnAtomic,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.onMessage(context.Background(), &types.MessageData{
			CustomCapabilities: customCapabilities,
		})

		require.True(t, gotMessage, "Message was not sent to agent")
	})

}

func Test_handleAgentOpAMPMessage(t *testing.T) {
	t.Run("CustomMessage - Custom message from agent is forwarded to server", func(t *testing.T) {
		customMessage := &protobufs.CustomMessage{
			Capability: "teapot",
			Type:       "brew",
			Data:       []byte("chamomile"),
		}

		gotMessageChan := make(chan struct{})
		client := &mockOpAMPClient{
			sendCustomMessageFunc: func(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
				require.Equal(t, customMessage, message)

				close(gotMessageChan)
				msgChan := make(chan struct{}, 1)
				msgChan <- struct{}{}
				return msgChan, nil
			},
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  client,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		loopDoneChan := make(chan struct{})
		go func() {
			defer close(loopDoneChan)
			s.forwardCustomMessagesToServerLoop()
		}()

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			CustomMessage: customMessage,
		})

		select {
		case <-gotMessageChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for custom message to send")
		}

		close(s.doneChan)

		select {
		case <-loopDoneChan:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for forward loop to stop")
		}
	})

	t.Run("CustomCapabilities - Custom capabilities from agent are forwarded to server", func(t *testing.T) {
		customCapabilities := &protobufs.CustomCapabilities{
			Capabilities: []string{"coffeemaker", "teapot"},
		}

		client := &mockOpAMPClient{
			setCustomCapabilitiesFunc: func(caps *protobufs.CustomCapabilities) error {
				require.Equal(t, customCapabilities, caps)
				return nil
			},
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			logger:                       zap.NewNop(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  client,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			CustomCapabilities: customCapabilities,
		})
	})
}

type staticPIDProvider int

func (s staticPIDProvider) PID() int {
	return int(s)
}

type mockOpAMPClient struct {
	agentDesc                 *protobufs.AgentDescription
	sendCustomMessageFunc     func(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error)
	setCustomCapabilitiesFunc func(customCapabilities *protobufs.CustomCapabilities) error
}

func (mockOpAMPClient) Start(_ context.Context, _ types.StartSettings) error {
	return nil
}

func (mockOpAMPClient) Stop(_ context.Context) error {
	return nil
}

func (m *mockOpAMPClient) SetAgentDescription(descr *protobufs.AgentDescription) error {
	m.agentDesc = descr
	return nil
}

func (m mockOpAMPClient) AgentDescription() *protobufs.AgentDescription {
	return m.agentDesc
}

func (mockOpAMPClient) SetHealth(_ *protobufs.ComponentHealth) error {
	return nil
}

func (mockOpAMPClient) UpdateEffectiveConfig(_ context.Context) error {
	return nil
}

func (mockOpAMPClient) SetRemoteConfigStatus(_ *protobufs.RemoteConfigStatus) error {
	return nil
}

func (mockOpAMPClient) SetPackageStatuses(_ *protobufs.PackageStatuses) error {
	return nil
}

func (mockOpAMPClient) RequestConnectionSettings(_ *protobufs.ConnectionSettingsRequest) error {
	return nil
}

func (m mockOpAMPClient) SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error {
	if m.setCustomCapabilitiesFunc != nil {
		return m.setCustomCapabilitiesFunc(customCapabilities)
	}
	return nil
}

func (m mockOpAMPClient) SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	if m.sendCustomMessageFunc != nil {
		return m.sendCustomMessageFunc(message)
	}

	msgChan := make(chan struct{}, 1)
	msgChan <- struct{}{}
	return msgChan, nil
}

type mockConn struct {
	sendFunc func(ctx context.Context, message *protobufs.ServerToAgent) error
}

func (mockConn) Connection() net.Conn {
	return nil
}
func (m mockConn) Send(ctx context.Context, message *protobufs.ServerToAgent) error {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}
func (mockConn) Disconnect() error {
	return nil
}
