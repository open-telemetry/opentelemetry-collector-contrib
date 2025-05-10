// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func setupSupervisorConfig(t *testing.T) config.Supervisor {
	t.Helper()

	tmpDir, err := os.MkdirTemp(os.TempDir(), "*")
	require.NoError(t, err)

	executablePath := filepath.Join(tmpDir, "binary")
	err = os.WriteFile(executablePath, []byte{}, 0o600)
	require.NoError(t, err)

	configuration := `
server:
  endpoint: ws://localhost/v1/opamp
  tls:
    insecure: true

capabilities:
  reports_effective_config: true
  reports_own_metrics: true
  reports_health: true
  accepts_remote_config: true
  reports_remote_config: true
  accepts_restart_command: true

storage:
  directory: %s

agent:
  executable: %s
`
	configuration = fmt.Sprintf(configuration, filepath.Join(tmpDir, "storage"), executablePath)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(cfgPath, []byte(configuration), 0o600)
	require.NoError(t, err)

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, os.Chmod(tmpDir, 0o700))
		require.NoError(t, os.RemoveAll(tmpDir))
	})

	return cfg
}

func Test_NewSupervisor(t *testing.T) {
	cfg := setupSupervisorConfig(t)
	supervisor, err := NewSupervisor(zap.L(), cfg)
	require.NoError(t, err)
	require.NotNil(t, supervisor)
}

func Test_NewSupervisorFailedStorageCreation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows because chmod doesn't affect permissions on Windows, so this test won't work.")
	}
	cfg := setupSupervisorConfig(t)

	dir := filepath.Dir(cfg.Storage.Directory)
	require.NoError(t, os.Chmod(dir, 0o500))

	supervisor, err := NewSupervisor(zap.L(), cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "error creating storage dir")
	require.Nil(t, supervisor)
}

func Test_composeEffectiveConfig(t *testing.T) {
	acceptsRemoteConfig := true
	s := Supervisor{
		logger:                       zap.NewNop(),
		persistentState:              &persistentState{},
		config:                       config.Supervisor{Capabilities: config.Capabilities{AcceptsRemoteConfig: acceptsRemoteConfig}},
		pidProvider:                  staticPIDProvider(1234),
		hasNewConfig:                 make(chan struct{}, 1),
		agentConfigOwnMetricsSection: &atomic.Value{},
		cfgState:                     &atomic.Value{},
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
	require.Equal(t, string(expectedConfig), s.cfgState.Load().(*configState).mergedConfig)
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
			cfgState:                     &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			opampClient:                  client.NewHTTP(newLoggerFromZap(zap.NewNop())),
		}
		require.NoError(t, s.createTemplates())

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
			cfgState:                     &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
		}
		require.NoError(t, s.createTemplates())

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
			cfgState:                     &atomic.Value{},
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

	t.Run("Processes all ServerToAgent fields", func(t *testing.T) {
		agentDesc := &atomic.Value{}
		agentDesc.Store(&protobufs.AgentDescription{
			NonIdentifyingAttributes: []*protobufs.KeyValue{
				{
					Key: "runtime.type",
					Value: &protobufs.AnyValue{
						Value: &protobufs.AnyValue_StringValue{
							StringValue: "test",
						},
					},
				},
			},
		})
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
			cfgState:                     &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			opampClient:                  client.NewHTTP(newLoggerFromZap(zap.NewNop())),
		}
		require.NoError(t, s.createTemplates())

		s.onMessage(context.Background(), &types.MessageData{
			AgentIdentification: &protobufs.AgentIdentification{
				NewInstanceUid: newID[:],
			},
			RemoteConfig: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {
							Body: []byte(""),
						},
					},
				},
			},
			OwnMetricsConnSettings: &protobufs.TelemetryConnectionSettings{
				DestinationEndpoint: "http://localhost:4318",
			},
		})

		require.Equal(t, newID, s.persistentState.InstanceID)
		t.Log(s.cfgState.Load())
		mergedCfg := s.cfgState.Load().(*configState).mergedConfig
		require.Contains(t, mergedCfg, "prometheus/own_metrics")
		require.Contains(t, mergedCfg, newID.String())
		require.Contains(t, mergedCfg, "runtime.type: test")
	})
	t.Run("RemoteConfig - Remote Config message is processed and merged into local config", func(t *testing.T) {

		const testConfigMessage = `receivers:
  debug:`

		const expectedMergedConfig = `extensions:
    health_check:
        endpoint: localhost:8000
    opamp:
        instance_uid: 018fee23-4a51-7303-a441-73faed7d9deb
        ppid: 88888
        ppid_poll_interval: 5s
        server:
            ws:
                endpoint: ws://127.0.0.1:0/v1/opamp
                tls:
                    insecure: true
receivers:
    debug: null
service:
    extensions:
        - health_check
        - opamp
    telemetry:
        logs:
            encoding: json
        resource: null
`

		remoteConfig := &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {
						Body: []byte(testConfigMessage),
					},
				},
			},
			ConfigHash: []byte("hash"),
		}
		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")

		remoteConfigStatusUpdated := false
		mc := &mockOpAMPClient{
			setRemoteConfigStatusFunc: func(rcs *protobufs.RemoteConfigStatus) error {
				remoteConfigStatusUpdated = true
				assert.Equal(
					t,
					&protobufs.RemoteConfigStatus{
						LastRemoteConfigHash: remoteConfig.ConfigHash,
						Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING,
					},
					rcs,
				)
				return nil
			},
			updateEffectiveConfigFunc: func(_ context.Context) error {
				return nil
			},
		}

		configStorageDir := t.TempDir()

		s := Supervisor{
			logger:      zap.NewNop(),
			pidProvider: staticPIDProvider(88888),
			config: config.Supervisor{
				Storage: config.Storage{
					Directory: configStorageDir,
				},
			},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			opampClient:                  mc,
			agentDescription:             &atomic.Value{},
			cfgState:                     &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		require.NoError(t, s.createTemplates())

		s.agentDescription.Store(&protobufs.AgentDescription{
			IdentifyingAttributes:    []*protobufs.KeyValue{},
			NonIdentifyingAttributes: []*protobufs.KeyValue{},
		})

		s.onMessage(context.Background(), &types.MessageData{
			RemoteConfig: remoteConfig,
		})

		fileContent, err := os.ReadFile(filepath.Join(configStorageDir, lastRecvRemoteConfigFile))
		require.NoError(t, err)
		assert.Contains(t, string(fileContent), testConfigMessage)
		assert.Equal(t, expectedMergedConfig, s.cfgState.Load().(*configState).mergedConfig)
		assert.True(t, remoteConfigStatusUpdated)
	})
	t.Run("RemoteConfig - Remote Config message is processed but OpAmp Client fails", func(t *testing.T) {

		const testConfigMessage = `receivers:
  debug:`

		const expectedMergedConfig = `extensions:
    health_check:
        endpoint: localhost:8000
    opamp:
        instance_uid: 018fee23-4a51-7303-a441-73faed7d9deb
        ppid: 88888
        ppid_poll_interval: 5s
        server:
            ws:
                endpoint: ws://127.0.0.1:0/v1/opamp
                tls:
                    insecure: true
receivers:
    debug: null
service:
    extensions:
        - health_check
        - opamp
    telemetry:
        logs:
            encoding: json
        resource: null
`

		remoteConfig := &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {
						Body: []byte(testConfigMessage),
					},
				},
			},
			ConfigHash: []byte("hash"),
		}
		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")

		remoteConfigStatusUpdated := false
		mc := &mockOpAMPClient{
			setRemoteConfigStatusFunc: func(rcs *protobufs.RemoteConfigStatus) error {
				remoteConfigStatusUpdated = true
				assert.Equal(
					t,
					&protobufs.RemoteConfigStatus{
						LastRemoteConfigHash: remoteConfig.ConfigHash,
						Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING,
					},
					rcs,
				)
				return fmt.Errorf("unexpected error")
			},
			updateEffectiveConfigFunc: func(_ context.Context) error {
				return nil
			},
		}

		configStorageDir := t.TempDir()

		s := Supervisor{
			logger:      zap.NewNop(),
			pidProvider: staticPIDProvider(88888),
			config: config.Supervisor{
				Storage: config.Storage{
					Directory: configStorageDir,
				},
			},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			opampClient:                  mc,
			agentDescription:             &atomic.Value{},
			cfgState:                     &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		require.NoError(t, s.createTemplates())

		s.agentDescription.Store(&protobufs.AgentDescription{
			IdentifyingAttributes:    []*protobufs.KeyValue{},
			NonIdentifyingAttributes: []*protobufs.KeyValue{},
		})

		s.onMessage(context.Background(), &types.MessageData{
			RemoteConfig: remoteConfig,
		})

		fileContent, err := os.ReadFile(filepath.Join(configStorageDir, lastRecvRemoteConfigFile))
		require.NoError(t, err)
		assert.Contains(t, string(fileContent), testConfigMessage)
		assert.Equal(t, expectedMergedConfig, s.cfgState.Load().(*configState).mergedConfig)
		assert.True(t, remoteConfigStatusUpdated)
	})
	t.Run("RemoteConfig - Invalid Remote Config message is detected and status is set appropriately", func(t *testing.T) {

		const testConfigMessage = `invalid`

		remoteConfig := &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {
						Body: []byte(testConfigMessage),
					},
				},
			},
			ConfigHash: []byte("hash"),
		}
		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")

		remoteConfigStatusUpdated := false
		mc := &mockOpAMPClient{
			setRemoteConfigStatusFunc: func(rcs *protobufs.RemoteConfigStatus) error {
				remoteConfigStatusUpdated = true
				assert.Equal(t, remoteConfig.ConfigHash, rcs.LastRemoteConfigHash)
				assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, rcs.Status)
				assert.NotEmpty(t, rcs.ErrorMessage)
				return nil
			},
			updateEffectiveConfigFunc: func(_ context.Context) error {
				return nil
			},
		}

		configStorageDir := t.TempDir()

		s := Supervisor{
			logger:      zap.NewNop(),
			pidProvider: defaultPIDProvider{},
			config: config.Supervisor{
				Storage: config.Storage{
					Directory: configStorageDir,
				},
			},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			opampClient:                  mc,
			agentDescription:             &atomic.Value{},
			cfgState:                     &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		require.NoError(t, s.createTemplates())

		s.agentDescription.Store(&protobufs.AgentDescription{
			IdentifyingAttributes:    []*protobufs.KeyValue{},
			NonIdentifyingAttributes: []*protobufs.KeyValue{},
		})

		s.onMessage(context.Background(), &types.MessageData{
			RemoteConfig: remoteConfig,
		})

		fileContent, err := os.ReadFile(filepath.Join(configStorageDir, lastRecvRemoteConfigFile))
		require.NoError(t, err)
		assert.Contains(t, string(fileContent), testConfigMessage)
		assert.Nil(t, s.cfgState.Load())
		assert.True(t, remoteConfigStatusUpdated)
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

	t.Run("EffectiveConfig - Effective config from agent is stored in OpAmpClient", func(t *testing.T) {
		updatedClientEffectiveConfig := false
		mc := &mockOpAMPClient{
			updateEffectiveConfigFunc: func(_ context.Context) error {
				updatedClientEffectiveConfig = true
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
			opampClient:                  mc,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			EffectiveConfig: &protobufs.EffectiveConfig{
				ConfigMap: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {
							Body: []byte("test"),
						},
					},
				},
			},
		})

		assert.Equal(t, "test", s.effectiveConfig.Load())
		assert.True(t, updatedClientEffectiveConfig)
	})
	t.Run("EffectiveConfig - Effective config from agent is stored in OpAmpClient; client returns error", func(t *testing.T) {
		updatedClientEffectiveConfig := false
		mc := &mockOpAMPClient{
			updateEffectiveConfigFunc: func(_ context.Context) error {
				updatedClientEffectiveConfig = true
				return fmt.Errorf("unexpected error")
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
			opampClient:                  mc,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			EffectiveConfig: &protobufs.EffectiveConfig{
				ConfigMap: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {
							Body: []byte("test"),
						},
					},
				},
			},
		})

		assert.Equal(t, "test", s.effectiveConfig.Load())
		assert.True(t, updatedClientEffectiveConfig)
	})
	t.Run("EffectiveConfig - Effective config message contains an empty config", func(t *testing.T) {
		updatedClientEffectiveConfig := false
		mc := &mockOpAMPClient{
			updateEffectiveConfigFunc: func(_ context.Context) error {
				updatedClientEffectiveConfig = true
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
			opampClient:                  mc,
			agentHealthCheckEndpoint:     "localhost:8000",
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			EffectiveConfig: &protobufs.EffectiveConfig{
				ConfigMap: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{},
				},
			},
		})

		assert.Empty(t, s.effectiveConfig.Load())
		assert.False(t, updatedClientEffectiveConfig)
	})
}

func TestSupervisor_setAgentDescription(t *testing.T) {
	s := &Supervisor{
		agentDescription: &atomic.Value{},
		config: config.Supervisor{
			Agent: config.Agent{
				Description: config.AgentDescription{
					IdentifyingAttributes: map[string]string{
						"overriding-attribute": "overridden-value",
						"additional-attribute": "additional-value",
					},
					NonIdentifyingAttributes: map[string]string{
						"overriding-attribute": "overridden-value",
						"additional-attribute": "additional-value",
					},
				},
			},
		},
	}

	ad := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "overriding-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "old-value",
					},
				},
			},
			{
				Key: "other-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "old-value",
					},
				},
			},
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "overriding-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "old-value",
					},
				},
			},
			{
				Key: "other-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "old-value",
					},
				},
			},
		},
	}
	s.setAgentDescription(ad)

	updatedAgentDescription := s.agentDescription.Load()

	expectedAgentDescription := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "additional-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "additional-value",
					},
				},
			},
			{
				Key: "other-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "old-value",
					},
				},
			},
			{
				Key: "overriding-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "overridden-value",
					},
				},
			},
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "additional-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "additional-value",
					},
				},
			},
			{
				Key: "other-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "old-value",
					},
				},
			},
			{
				Key: "overriding-attribute",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "overridden-value",
					},
				},
			},
		},
	}

	assert.Equal(t, expectedAgentDescription, updatedAgentDescription)
}

type staticPIDProvider int

func (s staticPIDProvider) PID() int {
	return int(s)
}

type mockOpAMPClient struct {
	agentDesc                 *protobufs.AgentDescription
	sendCustomMessageFunc     func(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error)
	setCustomCapabilitiesFunc func(customCapabilities *protobufs.CustomCapabilities) error
	updateEffectiveConfigFunc func(ctx context.Context) error
	setRemoteConfigStatusFunc func(rcs *protobufs.RemoteConfigStatus) error
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

func (m mockOpAMPClient) UpdateEffectiveConfig(ctx context.Context) error {
	return m.updateEffectiveConfigFunc(ctx)
}

func (m mockOpAMPClient) SetRemoteConfigStatus(rcs *protobufs.RemoteConfigStatus) error {
	return m.setRemoteConfigStatusFunc(rcs)
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

func TestSupervisor_findRandomPort(t *testing.T) {
	s := Supervisor{}
	port, err := s.findRandomPort()

	require.NoError(t, err)
	require.NotZero(t, port)
}

func TestSupervisor_setupOwnMetrics(t *testing.T) {
	testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
	t.Run("No DestinationEndpoint set", func(t *testing.T) {
		s := Supervisor{
			logger:                       zap.NewNop(),
			agentConfigOwnMetricsSection: &atomic.Value{},
			cfgState:                     &atomic.Value{},
			persistentState:              &persistentState{InstanceID: testUUID},
			pidProvider:                  staticPIDProvider(1234),
		}
		require.NoError(t, s.createTemplates())

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

		configChanged := s.setupOwnMetrics(context.Background(), &protobufs.TelemetryConnectionSettings{
			DestinationEndpoint: "",
		})

		assert.True(t, configChanged)
		assert.Empty(t, s.agentConfigOwnMetricsSection.Load().(string))
	})
	t.Run("DestinationEndpoint set - enable own metrics", func(t *testing.T) {
		s := Supervisor{
			logger:                       zap.NewNop(),
			agentConfigOwnMetricsSection: &atomic.Value{},
			cfgState:                     &atomic.Value{},
			persistentState:              &persistentState{InstanceID: testUUID},
			pidProvider:                  staticPIDProvider(1234),
		}
		err := s.createTemplates()

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

		require.NoError(t, err)

		configChanged := s.setupOwnMetrics(context.Background(), &protobufs.TelemetryConnectionSettings{
			DestinationEndpoint: "localhost",
		})

		expectedOwnMetricsSection := `receivers:
  # Collect own metrics
  prometheus/own_metrics:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 10s
          static_configs:
            - targets: ['0.0.0.0:55555']  
exporters:
  otlphttp/own_metrics:
    metrics_endpoint: "localhost"

service:
  telemetry:
    metrics:
      address: ":55555"
  pipelines:
    metrics/own_metrics:
      receivers: [prometheus/own_metrics]
      exporters: [otlphttp/own_metrics]
`

		assert.True(t, configChanged)

		got := s.agentConfigOwnMetricsSection.Load().(string)
		got = strings.ReplaceAll(got, "\r\n", "\n")

		// replace the port because that changes on each run
		portRegex := regexp.MustCompile(":[0-9]{5}")
		replaced := portRegex.ReplaceAll([]byte(got), []byte(":55555"))
		assert.Equal(t, expectedOwnMetricsSection, string(replaced))
	})
}

func TestSupervisor_createEffectiveConfigMsg(t *testing.T) {

	t.Run("empty config", func(t *testing.T) {
		s := Supervisor{
			effectiveConfig: &atomic.Value{},
			cfgState:        &atomic.Value{},
		}
		got := s.createEffectiveConfigMsg()

		assert.Empty(t, got.ConfigMap.ConfigMap[""].Body)
	})
	t.Run("effective and merged config set - prefer effective config", func(t *testing.T) {
		s := Supervisor{
			effectiveConfig: &atomic.Value{},
			cfgState:        &atomic.Value{},
		}

		s.effectiveConfig.Store("effective")
		s.cfgState.Store("merged")

		got := s.createEffectiveConfigMsg()

		assert.Equal(t, []byte("effective"), got.ConfigMap.ConfigMap[""].Body)
	})
	t.Run("only merged config set", func(t *testing.T) {
		s := Supervisor{
			effectiveConfig: &atomic.Value{},
			cfgState:        &atomic.Value{},
		}

		s.cfgState.Store(&configState{mergedConfig: "merged"})

		got := s.createEffectiveConfigMsg()

		assert.Equal(t, []byte("merged"), got.ConfigMap.ConfigMap[""].Body)
	})

}

func TestSupervisor_loadAndWriteInitialMergedConfig(t *testing.T) {

	t.Run("load initial config", func(t *testing.T) {

		configDir := t.TempDir()

		const testLastReceivedRemoteConfig = `receiver:
  debug/remote:
`

		const expectedMergedConfig = `exporters:
    otlphttp/own_metrics:
        metrics_endpoint: localhost
extensions:
    health_check:
        endpoint: ""
    opamp:
        instance_uid: 018fee23-4a51-7303-a441-73faed7d9deb
        ppid: 1234
        ppid_poll_interval: 5s
        server:
            ws:
                endpoint: ws://127.0.0.1:0/v1/opamp
                tls:
                    insecure: true
receiver:
    debug/remote: null
receivers:
    prometheus/own_metrics:
        config:
            scrape_configs:
                - job_name: otel-collector
                  scrape_interval: 10s
                  static_configs:
                    - targets:
                        - 0.0.0.0:55555
service:
    extensions:
        - health_check
        - opamp
    pipelines:
        metrics/own_metrics:
            exporters:
                - otlphttp/own_metrics
            receivers:
                - prometheus/own_metrics
    telemetry:
        logs:
            encoding: json
        metrics:
            address: :55555
        resource:
            service.name: otelcol
`

		remoteCfg := &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {
						Body: []byte(testLastReceivedRemoteConfig),
					},
				},
			},
			ConfigHash: []byte("hash"),
		}

		marshalledRemoteCfg, err := proto.Marshal(remoteCfg)
		require.NoError(t, err)

		ownMetricsCfg := &protobufs.TelemetryConnectionSettings{
			DestinationEndpoint: "localhost",
		}

		marshalledOwnMetricsCfg, err := proto.Marshal(ownMetricsCfg)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(configDir, lastRecvRemoteConfigFile), marshalledRemoteCfg, 0600))
		require.NoError(t, os.WriteFile(filepath.Join(configDir, lastRecvOwnMetricsConfigFile), marshalledOwnMetricsCfg, 0600))

		s := Supervisor{
			logger: zap.NewNop(),
			config: config.Supervisor{
				Capabilities: config.Capabilities{
					AcceptsRemoteConfig: true,
					ReportsOwnMetrics:   true,
				},
				Storage: config.Storage{
					Directory: configDir,
				},
			},
			agentConfigOwnMetricsSection: &atomic.Value{},
			cfgState:                     &atomic.Value{},
			persistentState: &persistentState{
				InstanceID: uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb"),
			},
			pidProvider: staticPIDProvider(1234),
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

		require.NoError(t, s.createTemplates())
		require.NoError(t, s.loadAndWriteInitialMergedConfig())

		assert.Equal(t, remoteCfg.String(), s.remoteConfig.String())

		gotMergedConfig := s.cfgState.Load().(*configState).mergedConfig
		gotMergedConfig = strings.ReplaceAll(gotMergedConfig, "\r\n", "\n")
		// replace random port numbers
		portRegex := regexp.MustCompile(":[0-9]{5}")
		replacedMergedConfig := portRegex.ReplaceAll([]byte(gotMergedConfig), []byte(":55555"))
		assert.Equal(t, expectedMergedConfig, string(replacedMergedConfig))
	})

}

func TestSupervisor_composeNoopConfig(t *testing.T) {

	const expectedConfig = `exporters:
    nop: null
extensions:
    opamp:
        instance_uid: 018fee23-4a51-7303-a441-73faed7d9deb
        ppid: 1234
        ppid_poll_interval: 5s
        server:
            ws:
                endpoint: ws://127.0.0.1:0/v1/opamp
                tls:
                    insecure: true
receivers:
    nop: null
service:
    extensions:
        - opamp
    pipelines:
        traces:
            exporters:
                - nop
            receivers:
                - nop
`
	s := Supervisor{
		persistentState: &persistentState{
			InstanceID: uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb"),
		},
		pidProvider: staticPIDProvider(1234),
	}

	require.NoError(t, s.createTemplates())

	noopConfigBytes, err := s.composeNoopConfig()
	noopConfig := strings.ReplaceAll(string(noopConfigBytes), "\r\n", "\n")

	require.NoError(t, err)
	require.Equal(t, expectedConfig, noopConfig)
}
