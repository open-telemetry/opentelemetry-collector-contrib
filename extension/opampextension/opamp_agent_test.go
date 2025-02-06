// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status/testhelpers"
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
	assert.NoError(t, o.Shutdown(context.Background()))
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
	assert.NoError(t, o.Shutdown(context.Background()))
}

func TestCreateAgentDescription(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	description := getOSDescription(zap.NewNop())

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
					stringKeyValue(semconv.AttributeOSDescription, description),
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
					stringKeyValue(semconv.AttributeOSDescription, description),
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
					stringKeyValue(semconv.AttributeOSDescription, description),
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
			assert.NoError(t, o.Shutdown(context.Background()))
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
	assert.NoError(t, o.Shutdown(context.Background()))
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
	assert.Equal(t, "text/yaml", ec.ConfigMap.ConfigMap[""].ContentType)

	assert.NoError(t, o.Shutdown(context.Background()))
}

func TestShutdown(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	// Shutdown with no OpAMP client
	assert.NoError(t, o.Shutdown(context.Background()))
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	assert.NoError(t, o.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, o.Shutdown(context.Background()))
}

func TestHealthReportingReceiveUpdateFromAggregator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	set := extensiontest.NewNopSettings()

	statusUpdateChannel := make(chan *status.AggregateStatus)

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

	mockOpampClient := &mockOpAMPClient{
		setHealthFunc: func(health *protobufs.ComponentHealth) error {
			mtx.Lock()
			defer mtx.Unlock()
			require.Equal(t, expectedHealthUpdates[receivedHealthUpdates], health)
			receivedHealthUpdates++
			return nil
		},
	}

	sa := &mockStatusAggregator{
		statusChan: statusUpdateChannel,
	}

	o := newTestOpampAgent(cfg, set, mockOpampClient, sa)

	o.initHealthReporting()

	assert.NoError(t, o.Start(context.Background(), componenttest.NewNopHost()))

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

	assert.NoError(t, o.Shutdown(context.Background()))
	require.True(t, sa.unsubscribed)
}

func TestHealthReportingForwardComponentHealthToAggregator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	set := extensiontest.NewNopSettings()

	mtx := &sync.RWMutex{}

	sa := &mockStatusAggregator{
		mtx: mtx,
	}

	o := newTestOpampAgent(
		cfg,
		set,
		&mockOpAMPClient{
			setHealthFunc: func(_ *protobufs.ComponentHealth) error {
				return nil
			},
		}, sa)

	o.initHealthReporting()

	assert.NoError(t, o.Start(context.Background(), componenttest.NewNopHost()))

	traces := testhelpers.NewPipelineMetadata("traces")

	// StatusStarting will be sent immediately.
	for _, id := range traces.InstanceIDs() {
		o.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusStarting))
	}

	// StatusOK will be queued until the PipelineWatcher Ready method is called.
	for _, id := range traces.InstanceIDs() {
		o.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusOK))
	}

	// verify we have received the StatusStarting events
	require.Eventually(t, func() bool {
		mtx.RLock()
		defer mtx.RUnlock()
		return len(sa.receivedEvents) == len(traces.InstanceIDs())
	}, 5*time.Second, 100*time.Millisecond)

	for _, event := range sa.receivedEvents {
		require.Equal(t, componentstatus.NewEvent(componentstatus.StatusStarting).Status(), event.event.Status())
	}

	// clean the received events of the mocked status aggregator
	sa.receivedEvents = nil

	err := o.Ready()
	require.NoError(t, err)

	// verify we have received the StatusOK events that have been queued while the agent has not been ready
	require.Eventually(t, func() bool {
		mtx.RLock()
		defer mtx.RUnlock()
		return len(sa.receivedEvents) == len(traces.InstanceIDs())
	}, 5*time.Second, 100*time.Millisecond)

	for _, event := range sa.receivedEvents {
		require.Equal(t, componentstatus.NewEvent(componentstatus.StatusOK).Status(), event.event.Status())
	}

	// clean the received events of the mocked status aggregator
	sa.receivedEvents = nil

	// send another set of events - these should be passed through immediately
	for _, id := range traces.InstanceIDs() {
		o.ComponentStatusChanged(id, componentstatus.NewEvent(componentstatus.StatusStopping))
	}

	require.Eventually(t, func() bool {
		mtx.RLock()
		defer mtx.RUnlock()
		return len(sa.receivedEvents) == len(traces.InstanceIDs())
	}, 5*time.Second, 100*time.Millisecond)

	for _, event := range sa.receivedEvents {
		require.Equal(t, componentstatus.NewEvent(componentstatus.StatusStopping).Status(), event.event.Status())
	}

	assert.NoError(t, o.Shutdown(context.Background()))
	require.True(t, sa.unsubscribed)
}

func TestHealthReportingExitsOnClosedContext(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	set := extensiontest.NewNopSettings()

	statusUpdateChannel := make(chan *status.AggregateStatus)
	sa := &mockStatusAggregator{
		statusChan: statusUpdateChannel,
	}

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

	mockOpampClient := &mockOpAMPClient{
		setHealthFunc: func(health *protobufs.ComponentHealth) error {
			mtx.Lock()
			defer mtx.Unlock()
			require.Equal(t, expectedHealthUpdates[receivedHealthUpdates], health)
			receivedHealthUpdates++
			return nil
		},
	}

	o := newTestOpampAgent(cfg, set, mockOpampClient, sa)

	o.initHealthReporting()

	assert.NoError(t, o.Start(context.Background(), componenttest.NewNopHost()))

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
	assert.NoError(t, o.Shutdown(context.Background()))
	require.True(t, sa.unsubscribed)
}

func TestHealthReportingDisabled(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	o.capabilities.ReportsHealth = false
	o.opampClient = &mockOpAMPClient{
		setHealthFunc: func(_ *protobufs.ComponentHealth) error {
			t.Errorf("setHealth is not supposed to be called with deactivated ReportsHealth capability")
			return nil
		},
	}

	assert.NoError(t, o.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, o.Shutdown(context.Background()))
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
	statusChan     chan *status.AggregateStatus
	receivedEvents []eventSourcePair
	unsubscribed   bool
	mtx            *sync.RWMutex
}

func (m *mockStatusAggregator) Subscribe(_ status.Scope, _ status.Verbosity) (<-chan *status.AggregateStatus, status.UnsubscribeFunc) {
	return m.statusChan, func() {
		m.unsubscribed = true
	}
}

func (m *mockStatusAggregator) RecordStatus(source *componentstatus.InstanceID, event *componentstatus.Event) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.receivedEvents = append(m.receivedEvents, eventSourcePair{
		source: source,
		event:  event,
	})
}

type mockOpAMPClient struct {
	setHealthFunc func(health *protobufs.ComponentHealth) error
}

func (m mockOpAMPClient) Start(_ context.Context, _ types.StartSettings) error {
	return nil
}

func (m mockOpAMPClient) Stop(_ context.Context) error {
	return nil
}

func (m mockOpAMPClient) SetAgentDescription(_ *protobufs.AgentDescription) error {
	return nil
}

func (m mockOpAMPClient) AgentDescription() *protobufs.AgentDescription {
	return nil
}

func (m mockOpAMPClient) SetHealth(health *protobufs.ComponentHealth) error {
	return m.setHealthFunc(health)
}

func (m mockOpAMPClient) UpdateEffectiveConfig(_ context.Context) error {
	return nil
}

func (m mockOpAMPClient) SetRemoteConfigStatus(_ *protobufs.RemoteConfigStatus) error {
	return nil
}

func (m mockOpAMPClient) SetPackageStatuses(_ *protobufs.PackageStatuses) error {
	return nil
}

func (m mockOpAMPClient) RequestConnectionSettings(_ *protobufs.ConnectionSettingsRequest) error {
	return nil
}

func (m mockOpAMPClient) SetCustomCapabilities(_ *protobufs.CustomCapabilities) error {
	return nil
}

func (m mockOpAMPClient) SendCustomMessage(_ *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	return nil, nil
}

func (m mockOpAMPClient) SetFlags(_ protobufs.AgentToServerFlags) {}

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

func newTestOpampAgent(cfg *Config, set extension.Settings, mockOpampClient *mockOpAMPClient, sa *mockStatusAggregator) *opampAgent {
	uid := uuid.New()
	o := &opampAgent{
		cfg:                      cfg,
		logger:                   set.Logger,
		agentType:                set.BuildInfo.Command,
		agentVersion:             set.BuildInfo.Version,
		instanceID:               uid,
		capabilities:             cfg.Capabilities,
		opampClient:              mockOpampClient,
		statusSubscriptionWg:     &sync.WaitGroup{},
		componentHealthWg:        &sync.WaitGroup{},
		readyCh:                  make(chan struct{}),
		customCapabilityRegistry: newCustomCapabilityRegistry(set.Logger, mockOpampClient),
		statusAggregator:         sa,
	}

	o.lifetimeCtx, o.lifetimeCtxCancel = context.WithCancel(context.Background())
	return o
}
