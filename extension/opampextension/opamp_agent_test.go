// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"go.opentelemetry.io/collector/component/componentstatus"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

func TestNewOpampAgent(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	set.BuildInfo = component.BuildInfo{Version: "test version", Command: "otelcoltest"}
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)
	assert.Equal(t, "otelcoltest", o.agentType)
	assert.Equal(t, "test version", o.agentVersion)
	assert.NotEmpty(t, o.instanceID.String())
	assert.True(t, o.capabilities.ReportsEffectiveConfig)
	assert.True(t, o.capabilities.ReportsHealth)
	assert.Empty(t, o.effectiveConfig)
	assert.Nil(t, o.agentDescription)
}

func TestNewOpampAgentAttributes(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	set.BuildInfo = component.BuildInfo{Version: "test version", Command: "otelcoltest"}
	set.Resource.Attributes().PutStr(semconv.AttributeServiceName, "otelcol-distro")
	set.Resource.Attributes().PutStr(semconv.AttributeServiceVersion, "distro.0")
	set.Resource.Attributes().PutStr(semconv.AttributeServiceInstanceID, "f8999bc1-4c9b-4619-9bae-7f009d2411ec")
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)
	assert.Equal(t, "otelcol-distro", o.agentType)
	assert.Equal(t, "distro.0", o.agentVersion)
	assert.Equal(t, "f8999bc1-4c9b-4619-9bae-7f009d2411ec", o.instanceID.String())
}

func TestCreateAgentDescription(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	serviceName := "otelcol-distrot"
	serviceVersion := "distro.0"
	serviceInstanceUUID := "f8999bc1-4c9b-4619-9bae-7f009d2411ec"

	testCases := []struct {
		name string
		cfg  func(*Config)

		expected *protobufs.AgentDescription
	}{
		{
			name: "No extra attributes",
			cfg:  func(_ *Config) {},
			expected: &protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeServiceInstanceID, serviceInstanceUUID),
					stringKeyValue(semconv.AttributeServiceName, serviceName),
					stringKeyValue(semconv.AttributeServiceVersion, serviceVersion),
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
					stringKeyValue(semconv.AttributeHostName, hostname),
					stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
				},
			},
		},
		{
			name: "Extra attributes specified",
			cfg: func(c *Config) {
				c.AgentDescription.NonIdentifyingAttributes = map[string]string{
					"env":                       "prod",
					semconv.AttributeK8SPodName: "my-very-cool-pod",
				}
			},
			expected: &protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeServiceInstanceID, serviceInstanceUUID),
					stringKeyValue(semconv.AttributeServiceName, serviceName),
					stringKeyValue(semconv.AttributeServiceVersion, serviceVersion),
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue("env", "prod"),
					stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
					stringKeyValue(semconv.AttributeHostName, hostname),
					stringKeyValue(semconv.AttributeK8SPodName, "my-very-cool-pod"),
					stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
				},
			},
		},
		{
			name: "Extra attributes override",
			cfg: func(c *Config) {
				c.AgentDescription.NonIdentifyingAttributes = map[string]string{
					semconv.AttributeHostName: "override-host",
				}
			},
			expected: &protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeServiceInstanceID, serviceInstanceUUID),
					stringKeyValue(semconv.AttributeServiceName, serviceName),
					stringKeyValue(semconv.AttributeServiceVersion, serviceVersion),
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
					stringKeyValue(semconv.AttributeHostName, "override-host"),
					stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			cfg := createDefaultConfig().(*Config)
			tc.cfg(cfg)

			set := extensiontest.NewNopSettings()
			set.Resource.Attributes().PutStr(semconv.AttributeServiceName, serviceName)
			set.Resource.Attributes().PutStr(semconv.AttributeServiceVersion, serviceVersion)
			set.Resource.Attributes().PutStr(semconv.AttributeServiceInstanceID, serviceInstanceUUID)

			o, err := newOpampAgent(cfg, set)
			require.NoError(t, err)
			assert.Nil(t, o.agentDescription)

			err = o.createAgentDescription()
			assert.NoError(t, err)
			require.Equal(t, tc.expected, o.agentDescription)
		})
	}
}

func TestUpdateAgentIdentity(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	olduid := o.instanceID
	assert.NotEmpty(t, olduid.String())

	uid := uuid.Must(uuid.NewV7())
	assert.NotEqual(t, uid, olduid)

	o.updateAgentIdentity(uid)
	assert.Equal(t, o.instanceID, uid)
}

func TestComposeEffectiveConfig(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)
	assert.Empty(t, o.effectiveConfig)

	ec := o.composeEffectiveConfig()
	assert.Nil(t, ec)

	ecFileName := filepath.Join("testdata", "effective.yaml")
	cm, err := confmaptest.LoadConf(ecFileName)
	assert.NoError(t, err)
	expected, err := os.ReadFile(ecFileName)
	assert.NoError(t, err)

	o.updateEffectiveConfig(cm)
	ec = o.composeEffectiveConfig()
	assert.NotNil(t, ec)
	assert.YAMLEq(t, string(expected), string(ec.ConfigMap.ConfigMap[""].Body))
}

func TestShutdown(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	// Shutdown with no OpAMP client
	assert.NoError(t, o.Shutdown(context.TODO()))
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	assert.NoError(t, o.Start(context.TODO(), componenttest.NewNopHost()))
	assert.NoError(t, o.Shutdown(context.TODO()))
}

func TestHealthReporting(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	statusUpdateChannel := make(chan *status.AggregateStatus)
	sa := &mockStatusAggregator{
		statusChan: statusUpdateChannel,
	}
	o.statusAggregator = sa

	mtx := &sync.RWMutex{}
	now := time.Now()
	expectedHealthUpdates := []*protobufs.ComponentHealth{
		{
			Healthy: false,
		},
		{
			Healthy:            true,
			StartTimeUnixNano:  uint64(now.UnixNano()),
			Status:             "StatusOK",
			StatusTimeUnixNano: uint64(now.UnixNano()),
			ComponentHealthMap: map[string]*protobufs.ComponentHealth{
				"test-receiver": {
					Healthy:            true,
					Status:             "StatusOK",
					StatusTimeUnixNano: uint64(now.UnixNano()),
				},
			},
		},
		{
			Healthy:            false,
			Status:             "StatusPermanentError",
			StatusTimeUnixNano: uint64(now.UnixNano()),
			LastError:          "unexpected error",
			ComponentHealthMap: map[string]*protobufs.ComponentHealth{
				"test-receiver": {
					Healthy:            false,
					Status:             "StatusPermanentError",
					StatusTimeUnixNano: uint64(now.UnixNano()),
					LastError:          "unexpected error",
				},
			},
		},
	}
	receivedHealthUpdates := 0

	o.opampClient = &mockOpAMPClient{
		setHealthFunc: func(health *protobufs.ComponentHealth) error {
			mtx.Lock()
			defer mtx.Unlock()
			require.Equal(t, expectedHealthUpdates[receivedHealthUpdates], health)
			receivedHealthUpdates++
			return nil
		},
	}

	assert.NoError(t, o.Start(context.TODO(), componenttest.NewNopHost()))

	statusUpdateChannel <- nil
	statusUpdateChannel <- &status.AggregateStatus{
		Event: &mockStatusEvent{
			status:    componentstatus.StatusOK,
			err:       nil,
			timestamp: now,
		},
		ComponentStatusMap: map[string]*status.AggregateStatus{
			"test-receiver": {
				Event: &mockStatusEvent{
					status:    componentstatus.StatusOK,
					err:       nil,
					timestamp: now,
				},
			},
		},
	}
	statusUpdateChannel <- &status.AggregateStatus{
		Event: &mockStatusEvent{
			status:    componentstatus.StatusPermanentError,
			err:       fmt.Errorf("unexpected error"),
			timestamp: now,
		},
		ComponentStatusMap: map[string]*status.AggregateStatus{
			"test-receiver": {
				Event: &mockStatusEvent{
					status:    componentstatus.StatusPermanentError,
					err:       fmt.Errorf("unexpected error"),
					timestamp: now,
				},
			},
		},
	}

	close(statusUpdateChannel)

	require.Eventually(t, func() bool {
		mtx.RLock()
		defer mtx.RUnlock()
		return receivedHealthUpdates == len(expectedHealthUpdates)
	}, 1*time.Second, 100*time.Millisecond)

	assert.NoError(t, o.Shutdown(context.TODO()))
	require.True(t, sa.unsubscribed)
}

func TestHealthReportingExitsOnClosedContext(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	statusUpdateChannel := make(chan *status.AggregateStatus)
	sa := &mockStatusAggregator{
		statusChan: statusUpdateChannel,
	}
	o.statusAggregator = sa

	mtx := &sync.RWMutex{}
	now := time.Now()
	expectedHealthUpdates := []*protobufs.ComponentHealth{
		{
			Healthy: false,
		},
		{
			Healthy:            true,
			StartTimeUnixNano:  uint64(now.UnixNano()),
			Status:             "StatusOK",
			StatusTimeUnixNano: uint64(now.UnixNano()),
			ComponentHealthMap: map[string]*protobufs.ComponentHealth{
				"test-receiver": {
					Healthy:            true,
					Status:             "StatusOK",
					StatusTimeUnixNano: uint64(now.UnixNano()),
				},
			},
		},
	}
	receivedHealthUpdates := 0

	o.opampClient = &mockOpAMPClient{
		setHealthFunc: func(health *protobufs.ComponentHealth) error {
			mtx.Lock()
			defer mtx.Unlock()
			require.Equal(t, expectedHealthUpdates[receivedHealthUpdates], health)
			receivedHealthUpdates++
			return nil
		},
	}

	assert.NoError(t, o.Start(context.TODO(), componenttest.NewNopHost()))

	statusUpdateChannel <- nil
	statusUpdateChannel <- &status.AggregateStatus{
		Event: &mockStatusEvent{
			status:    componentstatus.StatusOK,
			err:       nil,
			timestamp: now,
		},
		ComponentStatusMap: map[string]*status.AggregateStatus{
			"test-receiver": {
				Event: &mockStatusEvent{
					status:    componentstatus.StatusOK,
					err:       nil,
					timestamp: now,
				},
			},
		},
	}

	require.Eventually(t, func() bool {
		mtx.RLock()
		defer mtx.RUnlock()
		return receivedHealthUpdates == len(expectedHealthUpdates)
	}, 1*time.Second, 100*time.Millisecond)

	// invoke Shutdown before health update channel has been closed
	assert.NoError(t, o.Shutdown(context.TODO()))
	require.True(t, sa.unsubscribed)
}

func TestHealthReportingDisabled(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	o.capabilities.ReportsHealth = false
	o.opampClient = &mockOpAMPClient{
		setHealthFunc: func(health *protobufs.ComponentHealth) error {
			t.Errorf("setHealth is not supposed to be called with deactivated ReportsHealth capability")
			return nil
		},
	}

	assert.NoError(t, o.Start(context.TODO(), componenttest.NewNopHost()))
	assert.NoError(t, o.Shutdown(context.TODO()))
}

func TestParseInstanceIDString(t *testing.T) {
	testCases := []struct {
		name         string
		in           string
		expectedUUID uuid.UUID
		expectedErr  string
	}{
		{
			name:         "Parses ULID",
			in:           "7RK6DW2K4V8RCSQBKZ02EJ84FC",
			expectedUUID: uuid.MustParse("f8999bc1-4c9b-4619-9bae-7f009d2411ec"),
		},
		{
			name:         "Parses UUID",
			in:           "f8999bc1-4c9b-4619-9bae-7f009d2411ec",
			expectedUUID: uuid.MustParse("f8999bc1-4c9b-4619-9bae-7f009d2411ec"),
		},
		{
			name:        "Fails on invalid format",
			in:          "not-a-valid-id",
			expectedErr: "invalid UUID length: 14\nulid: bad data size when unmarshaling",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := parseInstanceIDString(tc.in)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}

			require.Equal(t, tc.expectedUUID, id)
		})
	}
}

func TestOpAMPAgent_Dependencies(t *testing.T) {
	t.Run("No server specified", func(t *testing.T) {
		o := opampAgent{
			cfg: &Config{},
		}

		require.Nil(t, o.Dependencies())
	})

	t.Run("No auth extension specified", func(t *testing.T) {
		o := opampAgent{
			cfg: &Config{
				Server: &OpAMPServer{
					WS: &commonFields{},
				},
			},
		}

		require.Nil(t, o.Dependencies())
	})

	t.Run("auth extension specified", func(t *testing.T) {
		authID := component.MustNewID("basicauth")
		o := opampAgent{
			cfg: &Config{
				Server: &OpAMPServer{
					WS: &commonFields{
						Auth: authID,
					},
				},
			},
		}

		require.Equal(t, []component.ID{authID}, o.Dependencies())
	})
}

type mockStatusAggregator struct {
	statusChan   chan *status.AggregateStatus
	unsubscribed bool
}

func (m *mockStatusAggregator) Subscribe(_ status.Scope, _ status.Verbosity) (<-chan *status.AggregateStatus, status.UnsubscribeFunc) {
	return m.statusChan, func() {
		m.unsubscribed = true
	}
}

type mockOpAMPClient struct {
	setHealthFunc func(health *protobufs.ComponentHealth) error
}

func (mockOpAMPClient) Start(_ context.Context, _ types.StartSettings) error {
	return nil
}

func (mockOpAMPClient) Stop(_ context.Context) error {
	return nil
}

func (mockOpAMPClient) SetAgentDescription(_ *protobufs.AgentDescription) error {
	return nil
}

func (mockOpAMPClient) AgentDescription() *protobufs.AgentDescription {
	return nil
}

func (m mockOpAMPClient) SetHealth(health *protobufs.ComponentHealth) error {
	return m.setHealthFunc(health)
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

func (mockOpAMPClient) SetCustomCapabilities(_ *protobufs.CustomCapabilities) error {
	return nil
}

func (mockOpAMPClient) SendCustomMessage(_ *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	return nil, nil
}

func (mockOpAMPClient) SetFlags(flags protobufs.AgentToServerFlags) {

}

type mockStatusEvent struct {
	status    componentstatus.Status
	err       error
	timestamp time.Time
}

func (m mockStatusEvent) Status() componentstatus.Status {
	return m.status
}

func (m mockStatusEvent) Err() error {
	return m.err
}

func (m mockStatusEvent) Timestamp() time.Time {
	return m.timestamp
}
