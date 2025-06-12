// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension

import (
	"context"
	"errors"
	"testing"

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
		assert.Equal(t, "test-host", ext.host.Identifier)
		assert.Equal(t, "test-uuid", ext.uuid)
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
		assert.NotEmpty(t, ext.moduleInfos.Receiver, "module infos should be populated")

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
		assert.Empty(t, ext.moduleInfos, "module infos should be empty")
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
		ext.moduleInfos = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		err = ext.NotifyConfig(context.Background(), conf)
		require.NoError(t, err)
		assert.NotNil(t, ext.collectorConfig)
		assert.NotNil(t, ext.otelCollectorMetadata)
		assert.Equal(t, "1.2.3", ext.otelCollectorMetadata.CollectorVersion)
		assert.NotEmpty(t, ext.otelCollectorMetadata.FullComponents)
		assert.NotEmpty(t, ext.otelCollectorMetadata.ActiveComponents)
		assert.NotNil(t, ext.httpServer, "http server should be created and started")

		// Ensure shutdown works cleanly
		assert.NoError(t, ext.Shutdown(context.Background()))
	})

	t.Run("success without http server", func(t *testing.T) {
		set := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"}, HTTPConfig: nil}
		ext, err := newExtension(context.Background(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		err = ext.NotifyConfig(context.Background(), confmap.New())
		require.NoError(t, err)
		assert.Nil(t, ext.httpServer, "http server should not be created")
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
	startCalled bool
	stopCalled  bool
	state       uint32
}

func (m *mockSerializer) Start() error {
	m.startCalled = true
	m.state = defaultforwarder.Started
	return nil
}

func (m *mockSerializer) Stop() {
	m.stopCalled = true
	m.state = defaultforwarder.Stopped
}

func (m *mockSerializer) State() uint32 {
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
