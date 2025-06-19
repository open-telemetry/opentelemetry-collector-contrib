// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/metrics/event"
	"github.com/DataDog/datadog-agent/pkg/metrics/servicecheck"
	"github.com/DataDog/datadog-agent/pkg/serializer/marshaler"
	"github.com/DataDog/datadog-agent/pkg/serializer/types"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/service"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// mockHost implements component.Host and hostcapabilities.ModuleInfo for testing
type mockHost struct {
	component.Host
	modules service.ModuleInfos
}

func (m *mockHost) GetModuleInfos() service.ModuleInfos {
	return m.modules
}

func TestNewExtension(t *testing.T) {
	cfg := &Config{API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"}}
	set := extension.Settings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}

	t.Run("success", func(t *testing.T) {
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		require.NotNil(t, ext)
		assert.Equal(t, "test-host", ext.info.host.Identifier)
		assert.Equal(t, "test-uuid", ext.info.uuid)
		assert.NotNil(t, ext.GetSerializer(), "serializer should be initialized")
	})

	t.Run("host provider error", func(t *testing.T) {
		hostProvider := &mockSourceProvider{err: errors.New("host error")}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		_, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.Error(t, err)
		assert.Equal(t, "host error", err.Error())
	})
}

func TestExtensionLifecycle(t *testing.T) {
	t.Run("start/shutdown with serializer and http server", func(t *testing.T) {
		set := extension.Settings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		mockSerializer := &mockSerializer{}
		ext.serializer = mockSerializer

		host := &mockHost{
			Host: componenttest.NewNopHost(),
			modules: service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
				component.MustNewType("mockreceiver"): {BuilderRef: "gomod.example/receiver v1.0.0"},
			}},
		}
		err = ext.Start(context.Background(), host)
		require.NoError(t, err)
		assert.True(t, mockSerializer.startCalled, "serializer.Start should be called")
		assert.NotEmpty(t, ext.info.modules.Receiver, "module infos should be populated")

		// NotifyConfig will create and start the http server
		err = ext.NotifyConfig(context.Background(), confmap.New())
		require.NoError(t, err)
		require.NotNil(t, ext.httpServer, "httpServer should be created")

		// Shutdown
		err = ext.Shutdown(context.Background())
		require.NoError(t, err)
		assert.True(t, mockSerializer.stopCalled, "serializer.Stop should be called")
	})

	t.Run("start/shutdown without serializer", func(t *testing.T) {
		set := extension.Settings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"}}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = nil // Test with no serializer

		err = ext.Start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		err = ext.Shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("start without ModuleInfo host capability", func(t *testing.T) {
		set := extension.Settings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"}}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// NopHost does not implement hostcapabilities.ModuleInfo
		err = ext.Start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		assert.Empty(t, ext.info.modules, "module infos should be empty")
	})
}

func TestNotifyConfig(t *testing.T) {
	t.Run("success with http server", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		err = ext.NotifyConfig(context.Background(), conf)
		require.NoError(t, err)
		assert.NotNil(t, ext.configs.collector)
		assert.NotNil(t, ext.otelCollectorMetadata)
		assert.Equal(t, "1.2.3", ext.otelCollectorMetadata.CollectorVersion)
		assert.NotEmpty(t, ext.otelCollectorMetadata.FullComponents)
		assert.NotEmpty(t, ext.otelCollectorMetadata.ActiveComponents)
		assert.NotNil(t, ext.httpServer, "http server should be created and started")

		// Ensure shutdown works cleanly
		assert.NoError(t, ext.Shutdown(context.Background()))
	})
}

func TestComponentStatusChanged(t *testing.T) {
	ext := &datadogExtension{}

	assert.NotPanics(t, func() {
		ext.ComponentStatusChanged(nil, nil)
	})

	instanceID := &componentstatus.InstanceID{}
	event := &componentstatus.Event{}
	assert.NotPanics(t, func() {
		ext.ComponentStatusChanged(instanceID, event)
	})
}

func TestNotifyConfigErrorPaths(t *testing.T) {
	t.Run("http server SendPayload error", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// Create a mock serializer that will cause SendPayload to fail
		mockSerializer := &mockFailingSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})

		// This should trigger the error path when SendPayload fails
		err = ext.NotifyConfig(context.Background(), conf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "payload send failed")

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})

	t.Run("PopulateFullComponentsJSON error", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}

		// Set moduleInfos to empty to trigger error path in PopulateFullComponentsJSON
		ext.info.modules = service.ModuleInfos{}

		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
		})

		// This should trigger warning but not fail
		err = ext.NotifyConfig(context.Background(), conf)
		require.NoError(t, err) // Should not fail, just log warning

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})

	t.Run("PopulateActiveComponents error", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}

		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		// Create an invalid configuration that will cause PopulateActiveComponents to fail
		conf := confmap.NewFromStringMap(map[string]any{
			"service": map[string]any{
				"pipelines": map[string]any{
					"traces": map[string]any{
						"receivers": "invalid-not-a-list", // This should cause an error
					},
				},
			},
		})
		ext.info.modules = service.ModuleInfos{
			Receiver: map[component.Type]service.ModuleInfo{
				component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
			},
		}

		// This should trigger warning but not fail
		err = ext.NotifyConfig(context.Background(), conf)
		require.NoError(t, err) // Should not fail, just log warning

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})
}

func TestSimpleInterfaceMethods(t *testing.T) {
	ext := &datadogExtension{}
	assert.NoError(t, ext.Ready())
	assert.NoError(t, ext.NotReady())
	assert.NotPanics(t, func() { ext.ComponentStatusChanged(nil, nil) })
}

func TestRealUUIDProvider(t *testing.T) {
	p := &realUUIDProvider{}
	uuid := p.NewString()
	assert.NotEmpty(t, uuid)
	assert.Regexp(t, `^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`, uuid, "should be a valid UUID")
}

func TestPeriodicPayloadSending(t *testing.T) {
	t.Run("periodic sending starts and stops properly", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		mockSerializer := &mockSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		// NotifyConfig should start the periodic payload sending
		err = ext.NotifyConfig(context.Background(), conf)
		require.NoError(t, err)

		// Verify periodic sending components are initialized
		assert.NotNil(t, ext.payloadSender.ticker, "payload ticker should be initialized")
		assert.NotNil(t, ext.payloadSender.ctx, "payload context should be initialized")
		assert.NotNil(t, ext.payloadSender.cancel, "payload cancel function should be initialized")
		assert.NotNil(t, ext.payloadSender.channel, "payload send channel should be initialized")

		// Shutdown should stop periodic sending
		err = ext.Shutdown(context.Background())
		require.NoError(t, err)

		// Verify context is cancelled
		select {
		case <-ext.payloadSender.ctx.Done():
			// Context should be cancelled
		default:
			t.Fatal("payload context should be cancelled after shutdown")
		}
	})

	t.Run("manual payload trigger works", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// Use a counting mock serializer to track payload sends
		mockSerializer := &countingMockSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		// NotifyConfig will send the initial payload
		err = ext.NotifyConfig(context.Background(), conf)
		require.NoError(t, err)

		initialCount := mockSerializer.GetSendCount()

		// Trigger manual payload send
		select {
		case ext.payloadSender.channel <- struct{}{}:
			// Successfully sent trigger
		default:
			t.Fatal("unable to send manual trigger")
		}

		// Give some time for the goroutine to process
		require.Eventually(t, func() bool {
			return mockSerializer.GetSendCount() > initialCount
		}, time.Second, 10*time.Millisecond, "manual payload should be sent")

		// Cleanup
		err = ext.Shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("periodic sending handles errors gracefully", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// Mock serializer that fails after the first call
		mockSerializer := &failAfterFirstCallSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		// NotifyConfig will succeed with the first payload
		err = ext.NotifyConfig(context.Background(), conf)
		require.NoError(t, err)

		// Trigger manual payload send which should fail but not crash
		select {
		case ext.payloadSender.channel <- struct{}{}:
			// Successfully sent trigger
		default:
			t.Fatal("unable to send manual trigger")
		}

		// Give some time for the error to be logged, but the extension should remain stable
		time.Sleep(100 * time.Millisecond)

		// Verify the extension is still functional by shutting down cleanly
		err = ext.Shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("shutdown handles nil fields gracefully", func(t *testing.T) {
		ext := &datadogExtension{}

		// Should not panic even with nil fields
		assert.NotPanics(t, func() {
			ext.stopPeriodicPayloadSending()
		})
	})
}

func TestNotifyConfigConcurrentAccess(t *testing.T) {
	t.Run("concurrent NotifyConfig calls are synchronized", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		mockSerializer := &mockSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		// Create multiple configurations to test concurrent access
		confs := []*confmap.Conf{
			confmap.NewFromStringMap(map[string]any{
				"receivers": map[string]any{"otlp": nil},
				"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
			}),
			confmap.NewFromStringMap(map[string]any{
				"receivers": map[string]any{"jaeger": nil},
				"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"jaeger"}}}},
			}),
			confmap.NewFromStringMap(map[string]any{
				"receivers": map[string]any{"zipkin": nil},
				"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"zipkin"}}}},
			}),
		}

		// Run concurrent NotifyConfig calls
		const numGoroutines = 10
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(confIndex int) {
				defer wg.Done()
				conf := confs[confIndex%len(confs)]
				if err := ext.NotifyConfig(context.Background(), conf); err != nil {
					// First call might succeed, subsequent calls will fail due to HTTP server already running
					// But they should not race condition or panic
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Verify no race conditions occurred - the test should complete without panic
		// and the collector config should be set to one of the configurations
		ext.configs.mutex.RLock()
		assert.NotNil(t, ext.configs.collector, "collector config should be set")
		ext.configs.mutex.RUnlock()

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})
}

// Mock providers for testing
var _ source.Provider = (*mockSourceProvider)(nil)

type mockSourceProvider struct {
	source source.Source
	err    error
}

func (m *mockSourceProvider) Source(_ context.Context) (source.Source, error) {
	if m.err != nil {
		return source.Source{}, m.err
	}
	return m.source, nil
}

var _ uuidProvider = (*mockUUIDProvider)(nil)

type mockUUIDProvider struct {
	mockUUID string
}

func (p *mockUUIDProvider) NewString() string {
	return p.mockUUID
}

var _ agentcomponents.SerializerWithForwarder = (*mockSerializer)(nil)

type mockSerializer struct {
	mu          sync.RWMutex
	startCalled bool
	stopCalled  bool
	state       uint32
}

func (m *mockSerializer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = true
	m.state = defaultforwarder.Started
	return nil
}

func (m *mockSerializer) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled = true
	m.state = defaultforwarder.Stopped
}

func (m *mockSerializer) State() uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *mockSerializer) SendMetadata(marshaler.JSONMarshaler) error            { return nil }
func (m *mockSerializer) SendEvents(event.Events) error                         { return nil }
func (m *mockSerializer) SendServiceChecks(servicecheck.ServiceChecks) error    { return nil }
func (m *mockSerializer) SendIterableSeries(metrics.SerieSource) error          { return nil }
func (m *mockSerializer) AreSeriesEnabled() bool                                { return false }
func (m *mockSerializer) SendSketch(metrics.SketchesSource) error               { return nil }
func (m *mockSerializer) AreSketchesEnabled() bool                              { return false }
func (m *mockSerializer) SendHostMetadata(marshaler.JSONMarshaler) error        { return nil }
func (m *mockSerializer) SendProcessesMetadata(any) error                       { return nil }
func (m *mockSerializer) SendAgentchecksMetadata(marshaler.JSONMarshaler) error { return nil }
func (m *mockSerializer) SendOrchestratorMetadata([]types.ProcessMessageBody, string, string, int) error {
	return nil
}

func (m *mockSerializer) SendOrchestratorManifests([]types.ProcessMessageBody, string, string) error {
	return nil
}

// Mock serializer that fails SendPayload for testing error paths
type mockFailingSerializer struct {
	mockSerializer
}

func (m *mockFailingSerializer) SendMetadata(marshaler.JSONMarshaler) error {
	return errors.New("payload send failed")
}

// Mock serializer that counts the number of payload sends for testing periodic functionality
type countingMockSerializer struct {
	mockSerializer
	sendCount int
}

func (m *countingMockSerializer) SendMetadata(marshaler.JSONMarshaler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCount++
	return nil
}

func (m *countingMockSerializer) GetSendCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sendCount
}

// Mock serializer that succeeds on the first call but fails on subsequent calls
type failAfterFirstCallSerializer struct {
	mockSerializer
	callCount int
}

func (m *failAfterFirstCallSerializer) SendMetadata(marshaler.JSONMarshaler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.callCount > 1 {
		return errors.New("payload send failed after first call")
	}
	return nil
}

func (m *failAfterFirstCallSerializer) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}
