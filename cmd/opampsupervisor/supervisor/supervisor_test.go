// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
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
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	serverTypes "github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

const configTemplate = `
server:
  endpoint: ws://localhost/v1/opamp
  tls:
    insecure: true

capabilities:
  reports_effective_config: true
  reports_own_metrics: true
  reports_own_logs: true
  reports_own_traces: true
  reports_health: true
  accepts_remote_config: true
  reports_remote_config: true
  accepts_restart_command: true

storage:
  directory: %s

agent:
  executable: %s
`

const configTemplateWithTelemetrySettings = `
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

telemetry:
  traces:
    processors:
      - batch:
          exporter:
            otlp:
              protocol: http/protobuf
              endpoint: https://backend:4317
  metrics:
    readers:
      - periodic:
          exporter:
            otlp:
              protocol: http/protobuf
              endpoint: http://localhost:14317
  logs:
    level: info
    processors:
      - batch:
          exporter:
            otlp:
              protocol: http/protobuf
              endpoint: https://backend:4317
`

func setupSupervisorConfig(t *testing.T, configuration string) config.Supervisor {
	t.Helper()

	tmpDir := t.TempDir()

	executablePath := filepath.Join(tmpDir, "binary")
	err := os.WriteFile(executablePath, []byte{}, 0o600)
	require.NoError(t, err)
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

func newNopTelemetrySettings() telemetrySettings {
	return telemetrySettings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}
}

func Test_NewSupervisor(t *testing.T) {
	cfg := setupSupervisorConfig(t, configTemplate)
	supervisor, err := NewSupervisor(zap.L(), cfg)
	require.NoError(t, err)
	require.NotNil(t, supervisor)
}

func Test_NewSupervisorWithTelemetrySettings(t *testing.T) {
	cfg := setupSupervisorConfig(t, configTemplateWithTelemetrySettings)
	supervisor, err := NewSupervisor(zap.L(), cfg)
	require.NoError(t, err)
	require.NotNil(t, supervisor)
	require.NotEmpty(t, supervisor.telemetrySettings)
	require.NotNil(t, supervisor.telemetrySettings.MeterProvider)
	require.NotNil(t, supervisor.telemetrySettings.TracerProvider)
	require.NotNil(t, supervisor.telemetrySettings.Logger)
	require.NotNil(t, supervisor.telemetrySettings.loggerProvider)

	supervisor.Shutdown()
}

func Test_NewSupervisorFailedStorageCreation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows because chmod doesn't affect permissions on Windows, so this test won't work.")
	}
	cfg := setupSupervisorConfig(t, configTemplate)

	dir := filepath.Dir(cfg.Storage.Directory)
	require.NoError(t, os.Chmod(dir, 0o500))

	supervisor, err := NewSupervisor(zap.L(), cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "error creating storage dir")
	require.Nil(t, supervisor)
}

func Test_composeEffectiveConfig(t *testing.T) {
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

	// load expected effective config bytes once
	effectiveConfig, err := os.ReadFile("../testdata/collector/effective_config.yaml")
	require.NoError(t, err)

	mergedEffectiveConfig, err := os.ReadFile("./testdata/merged_effective_config.yaml")
	require.NoError(t, err)

	mergedLocalConfig, err := os.ReadFile("./testdata/merged_local_config.yaml")
	require.NoError(t, err)

	tests := []struct {
		name                string
		configFiles         []string
		acceptsRemoteConfig bool
		remoteConfig        *protobufs.AgentRemoteConfig
		wantErr             bool
		wantChanged         bool
		wantConfig          []byte
	}{
		{
			name:                "can accept remote config, receives one",
			configFiles:         []string{"testdata/local_config1.yaml", "testdata/local_config2.yaml"},
			acceptsRemoteConfig: true,
			remoteConfig: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {Body: []byte(fileLogConfig)},
					},
				},
			},
			wantErr:     false,
			wantChanged: true,
			wantConfig:  effectiveConfig,
		},
		{
			name:                "can accept remote config, receives none",
			configFiles:         []string{"testdata/local_config1.yaml", "testdata/local_config2.yaml"},
			acceptsRemoteConfig: true,
			remoteConfig:        nil,
			wantErr:             false,
			wantChanged:         true,
			wantConfig:          mergedLocalConfig,
		},
		{
			name:                "cannot accept remote config, receives one",
			configFiles:         []string{"../testdata/collector/effective_config_without_extensions.yaml"},
			acceptsRemoteConfig: false,
			remoteConfig: &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {Body: []byte(fileLogConfig)},
					},
				},
			},
			wantErr:     false,
			wantChanged: true,
			wantConfig:  effectiveConfig,
		},
		{
			name:                "cannot accept remote config, receives none",
			configFiles:         []string{"../testdata/collector/effective_config_without_extensions.yaml"},
			acceptsRemoteConfig: false,
			remoteConfig:        nil,
			wantErr:             false,
			wantChanged:         false,
			wantConfig:          mergedEffectiveConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Supervisor{
				telemetrySettings:            newNopTelemetrySettings(),
				persistentState:              &persistentState{},
				config:                       config.Supervisor{Capabilities: config.Capabilities{AcceptsRemoteConfig: tt.acceptsRemoteConfig}, Agent: config.Agent{ConfigFiles: tt.configFiles}},
				pidProvider:                  staticPIDProvider(1234),
				hasNewConfig:                 make(chan struct{}, 1),
				agentConfigOwnMetricsSection: &atomic.Value{},
				cfgState:                     &atomic.Value{},
			}
			agentDesc := &atomic.Value{}
			agentDesc.Store(&protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					{
						Key: "service.name",
						Value: &protobufs.AnyValue{
							Value: &protobufs.AnyValue_StringValue{StringValue: "otelcol"},
						},
					},
				},
			})
			s.agentDescription = agentDesc

			s.loadRemoteConfig()
			require.NoError(t, s.createTemplates())
			require.NoError(t, s.loadAndWriteInitialMergedConfig())

			changed, err := s.composeMergedConfig(tt.remoteConfig)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantChanged, changed)
			got := s.cfgState.Load().(*configState).mergedConfig

			k := koanf.New("::")
			err = k.Load(rawbytes.Provider(tt.wantConfig), yaml.Parser(), koanf.WithMergeFunc(configMergeFunc))
			require.NoError(t, err)

			gotParsed, err := k.Marshal(yaml.Parser())

			require.NoError(t, err)
			require.Equal(t, string(gotParsed), got)
		})
	}
}

func Test_onMessage(t *testing.T) {
	t.Run("AgentIdentification - New instance ID is valid", func(t *testing.T) {
		agentDesc := &atomic.Value{}
		agentDesc.Store(&protobufs.AgentDescription{})
		initialID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		newID := uuid.MustParse("018fef3f-14a8-73ef-b63e-3b96b146ea38")
		s := Supervisor{
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: initialID},
			agentDescription:             agentDesc,
			agentConfigOwnMetricsSection: &atomic.Value{},
			cfgState:                     &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			opampClient:                  client.NewHTTP(newLoggerFromZap(zap.NewNop(), "opamp-client")),
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentDescription:             agentDesc,
			agentConfigOwnMetricsSection: &atomic.Value{},
			cfgState:                     &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			cfgState:                     &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    agentConnAtomic,
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    agentConnAtomic,
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: initialID},
			agentDescription:             agentDesc,
			agentConfigOwnMetricsSection: &atomic.Value{},
			cfgState:                     &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			opampClient:                  client.NewHTTP(newLoggerFromZap(zap.NewNop(), "opamp-client")),
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
				DestinationEndpoint: "http://127.0.0.1:4318",
				Headers: &protobufs.Headers{
					Headers: []*protobufs.Header{
						{Key: "testkey", Value: "testval"},
						{Key: "testkey2", Value: "testval2"},
					},
				},
			},
		})

		require.Equal(t, newID, s.persistentState.InstanceID)
		t.Log(s.cfgState.Load())
		mergedCfg := s.cfgState.Load().(*configState).mergedConfig
		require.Contains(t, mergedCfg, newID.String())
		require.Contains(t, mergedCfg, "runtime.type: test")
	})
	t.Run("RemoteConfig - Remote Config message is processed and merged into local config", func(t *testing.T) {
		const testConfigMessage = `receivers:
  debug:`

		const expectedMergedConfig = `extensions:
    opamp:
        capabilities:
            reports_available_components: false
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
			telemetrySettings: newNopTelemetrySettings(),
			pidProvider:       staticPIDProvider(88888),
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
    opamp:
        capabilities:
            reports_available_components: false
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
				return errors.New("unexpected error")
			},
			updateEffectiveConfigFunc: func(_ context.Context) error {
				return nil
			},
		}

		configStorageDir := t.TempDir()

		s := Supervisor{
			telemetrySettings: newNopTelemetrySettings(),
			pidProvider:       staticPIDProvider(88888),
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
			telemetrySettings: newNopTelemetrySettings(),
			pidProvider:       defaultPIDProvider{},
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
	t.Run("RemoteConfig - Don't report status if config is not changed", func(t *testing.T) {
		const testConfigMessage = `receivers:
  debug:`

		const expectedMergedConfig = `extensions:
    opamp:
        capabilities:
            reports_available_components: false
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
        - opamp
    telemetry:
        logs:
            encoding: json
        resource: null
`

		// the remote config message we will send that will get merged and compared with the initial config
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

		remoteConfigStatusUpdated := false
		mc := &mockOpAMPClient{
			setRemoteConfigStatusFunc: func(_ *protobufs.RemoteConfigStatus) error {
				remoteConfigStatusUpdated = true
				return nil
			},
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		configStorageDir := t.TempDir()
		s := Supervisor{
			telemetrySettings: newNopTelemetrySettings(),
			pidProvider:       staticPIDProvider(88888),
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
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		require.NoError(t, s.createTemplates())

		// need to set the agent description as part of initialization
		s.agentDescription.Store(&protobufs.AgentDescription{
			IdentifyingAttributes:    []*protobufs.KeyValue{},
			NonIdentifyingAttributes: []*protobufs.KeyValue{},
		})

		// initially write & store config so that we have the same config when we send the remote config message
		err := os.WriteFile(filepath.Join(configStorageDir, lastRecvRemoteConfigFile), []byte(testConfigMessage), 0o600)
		require.NoError(t, err)

		s.cfgState.Store(&configState{
			mergedConfig:     expectedMergedConfig,
			configMapIsEmpty: false,
		})

		s.onMessage(context.Background(), &types.MessageData{
			RemoteConfig: remoteConfig,
		})

		// assert the remote config status callback was not called
		assert.False(t, remoteConfigStatusUpdated)
		// assert the config file and stored data are still the same
		fileContent, err := os.ReadFile(filepath.Join(configStorageDir, lastRecvRemoteConfigFile))
		require.NoError(t, err)
		assert.Contains(t, string(fileContent), testConfigMessage)
		assert.Equal(t, expectedMergedConfig, s.cfgState.Load().(*configState).mergedConfig)
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  client,
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  client,
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  mc,
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
				return errors.New("unexpected error")
			},
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  mc,
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
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  mc,
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

	t.Run("ComponentHealth - Component health from agent is set in OpAmpClient", func(t *testing.T) {
		healthSet := false
		mc := &mockOpAMPClient{
			setHealthFunc: func(_ *protobufs.ComponentHealth) {
				healthSet = true
			},
		}

		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			telemetrySettings:            newNopTelemetrySettings(),
			pidProvider:                  defaultPIDProvider{},
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentConn:                    &atomic.Value{},
			opampClient:                  mc,
			customMessageToServer:        make(chan *protobufs.CustomMessage, 10),
			doneChan:                     make(chan struct{}),
		}

		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			Health: &protobufs.ComponentHealth{
				Healthy: true,
			},
		})

		assert.True(t, healthSet)
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
	setHealthFunc             func(health *protobufs.ComponentHealth)
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

func (m mockOpAMPClient) SetHealth(h *protobufs.ComponentHealth) error {
	m.setHealthFunc(h)
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

func (m mockOpAMPClient) SetAvailableComponents(_ *protobufs.AvailableComponents) (err error) {
	return nil
}

func (m mockOpAMPClient) SetFlags(_ protobufs.AgentToServerFlags) {}

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

func TestSupervisor_setupOwnTelemetry(t *testing.T) {
	testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
	t.Run("No DestinationEndpoint set", func(t *testing.T) {
		s := Supervisor{
			telemetrySettings:            newNopTelemetrySettings(),
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

		configChanged := s.setupOwnTelemetry(context.Background(), &protobufs.ConnectionSettingsOffers{OwnMetrics: &protobufs.TelemetryConnectionSettings{
			DestinationEndpoint: "",
		}})

		assert.True(t, configChanged)
		assert.Empty(t, s.agentConfigOwnMetricsSection.Load().(string))
	})
	t.Run("DestinationEndpoint set - enable own metrics", func(t *testing.T) {
		s := Supervisor{
			telemetrySettings:            newNopTelemetrySettings(),
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

		configChanged := s.setupOwnTelemetry(context.Background(), &protobufs.ConnectionSettingsOffers{OwnMetrics: &protobufs.TelemetryConnectionSettings{
			DestinationEndpoint: "http://127.0.0.1:4318",
			Headers: &protobufs.Headers{
				Headers: []*protobufs.Header{
					{Key: "testkey", Value: "testval"},
					{Key: "testkey2", Value: "testval2"},
				},
			},
		}})

		expectedOwnMetricsSection := `
service:
  telemetry:
    metrics:
      readers:
        - periodic:
            exporter:
              otlp:
                protocol: http/protobuf
                endpoint: http://127.0.0.1:4318
                headers:
                  "testkey": "testval"
                  "testkey2": "testval2"
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

		const expectedMergedConfig = `extensions:
    opamp:
        capabilities:
            reports_available_components: false
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
service:
    extensions:
        - opamp
    telemetry:
        logs:
            encoding: json
            processors:
                - batch:
                    exporter:
                        otlp:
                            endpoint: localhost-logs
                            protocol: http/protobuf
        metrics:
            readers:
                - periodic:
                    exporter:
                        otlp:
                            endpoint: localhost-metrics
                            protocol: http/protobuf
        resource:
            service.name: otelcol
        traces:
            processors:
                - batch:
                    exporter:
                        otlp:
                            endpoint: localhost-traces
                            protocol: http/protobuf
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

		ownTelemetryCfg := &protobufs.ConnectionSettingsOffers{
			OwnMetrics: &protobufs.TelemetryConnectionSettings{
				DestinationEndpoint: "localhost-metrics",
			},
			OwnLogs: &protobufs.TelemetryConnectionSettings{
				DestinationEndpoint: "localhost-logs",
			},
			OwnTraces: &protobufs.TelemetryConnectionSettings{
				DestinationEndpoint: "localhost-traces",
			},
		}

		marshalledOwnTelemetryCfg, err := proto.Marshal(ownTelemetryCfg)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(configDir, lastRecvRemoteConfigFile), marshalledRemoteCfg, 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(configDir, lastRecvOwnTelemetryConfigFile), marshalledOwnTelemetryCfg, 0o600))

		s := Supervisor{
			telemetrySettings: newNopTelemetrySettings(),
			config: config.Supervisor{
				Capabilities: config.Capabilities{
					AcceptsRemoteConfig: true,
					ReportsOwnMetrics:   true,
					ReportsOwnLogs:      true,
					ReportsOwnTraces:    true,
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

		s.loadRemoteConfig()
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
        capabilities:
            reports_available_components: false
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

func TestSupervisor_composeNoopConfigReportAvailableComponents(t *testing.T) {
	const expectedConfig = `exporters:
    nop: null
extensions:
    opamp:
        capabilities:
            reports_available_components: true
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
		config: config.Supervisor{
			Capabilities: config.Capabilities{
				ReportsAvailableComponents: true,
			},
		},
	}

	require.NoError(t, s.createTemplates())

	noopConfigBytes, err := s.composeNoopConfig()
	noopConfig := strings.ReplaceAll(string(noopConfigBytes), "\r\n", "\n")

	require.NoError(t, err)
	require.Equal(t, expectedConfig, noopConfig)
}

func TestSupervisor_configStrictUnmarshal(t *testing.T) {
	tmpDir := t.TempDir()

	configuration := `
server:
  endpoint: ws://localhost/v1/opamp
  tls:
    insecure: true

capabilities:
  reports_effective_config: true
  invalid_key: invalid_value
`

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(configuration), 0o600)
	require.NoError(t, err)

	_, err = config.Load(cfgPath)
	require.Error(t, err)
	require.ErrorContains(t, err, "decoding failed")

	t.Cleanup(func() {
		require.NoError(t, os.Chmod(tmpDir, 0o700))
		require.NoError(t, os.RemoveAll(tmpDir))
	})
}

func TestSupervisor_exportLogsWithSDK(t *testing.T) {
	template := `
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

telemetry:
  logs:
    level: info
    processors:
      - batch:
          exporter:
            otlp:
              protocol: http/protobuf
              endpoint: http://127.0.0.1:4318
`

	cfg := setupSupervisorConfig(t, template)
	supervisor, err := NewSupervisor(zap.NewNop(), cfg)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "output.txt")
	backend := testbed.NewOTLPHTTPDataReceiver(4318)
	mockBackend := testbed.NewMockBackend(path, backend)
	err = mockBackend.Start()
	require.NoError(t, err)

	mockBackend.EnableRecording()
	supervisor.telemetrySettings.Logger.Info("test log")

	require.Eventually(t, func() bool {
		return len(mockBackend.GetReceivedLogs()) > 0
	}, 5*time.Second, 1*time.Second)

	mockBackend.Stop()

	receivedLogs := mockBackend.GetReceivedLogs()
	require.Len(t, receivedLogs, 1)
	l := mockBackend.ReceivedLogs[0]
	require.Equal(t, 1, l.ResourceLogs().Len())
	l.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		require.Equal(t, 1, rl.ScopeLogs().Len())
		rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
			require.Equal(t, 1, sl.LogRecords().Len())
			sl.LogRecords().RemoveIf(func(lr plog.LogRecord) bool {
				assert.Equal(t, "test log", lr.Body().Str())
				return false
			})
			return false
		})
		return false
	})

	supervisor.Shutdown()
}
