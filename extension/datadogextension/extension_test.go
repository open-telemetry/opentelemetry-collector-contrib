// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/metrics/event"
	"github.com/DataDog/datadog-agent/pkg/metrics/servicecheck"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/DataDog/datadog-agent/pkg/serializer/marshaler"
	"github.com/DataDog/datadog-agent/pkg/serializer/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		require.NotNil(t, ext)
		assert.Equal(t, "test-host", ext.info.host.Identifier)
		assert.Equal(t, "test-uuid", ext.info.uuid)
		assert.NotNil(t, ext.GetSerializer(), "serializer should be initialized")
	})

	t.Run("host provider error", func(t *testing.T) {
		hostProvider := &mockSourceProvider{err: errors.New("host error")}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		_, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		mockSerializer := &mockSerializer{}
		ext.serializer = mockSerializer

		host := &mockHost{
			Host: componenttest.NewNopHost(),
			modules: service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
				component.MustNewType("mockreceiver"): {BuilderRef: "gomod.example/receiver v1.0.0"},
			}},
		}
		err = ext.Start(t.Context(), host)
		require.NoError(t, err)
		assert.True(t, mockSerializer.startCalled, "serializer.Start should be called")
		assert.NotEmpty(t, ext.info.modules.Receiver, "module infos should be populated")

		// NotifyConfig will create and start the http server
		err = ext.NotifyConfig(t.Context(), confmap.New())
		require.NoError(t, err)
		require.NotNil(t, ext.httpServer, "httpServer should be created")

		// Shutdown
		err = ext.Shutdown(t.Context())
		require.NoError(t, err)
		assert.True(t, mockSerializer.stopCalled, "serializer.Stop should be called")
	})

	t.Run("start/shutdown without serializer", func(t *testing.T) {
		set := extension.Settings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"}}
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = nil // Test with no serializer

		err = ext.Start(t.Context(), componenttest.NewNopHost())
		require.NoError(t, err)
		err = ext.Shutdown(t.Context())
		require.NoError(t, err)
	})

	t.Run("start without ModuleInfo host capability", func(t *testing.T) {
		set := extension.Settings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}
		hostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		uuidProvider := &mockUUIDProvider{mockUUID: "test-uuid"}
		cfg := &Config{API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"}}
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// NopHost does not implement hostcapabilities.ModuleInfo
		err = ext.Start(t.Context(), componenttest.NewNopHost())
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		err = ext.NotifyConfig(t.Context(), conf)
		require.NoError(t, err)
		assert.NotNil(t, ext.configs.collector)
		assert.NotNil(t, ext.otelCollectorMetadata)
		assert.Equal(t, "1.2.3", ext.otelCollectorMetadata.CollectorVersion)
		assert.NotEmpty(t, ext.otelCollectorMetadata.FullComponents)
		assert.NotEmpty(t, ext.otelCollectorMetadata.ActiveComponents)
		assert.NotNil(t, ext.httpServer, "http server should be created and started")

		// Ensure shutdown works cleanly
		assert.NoError(t, ext.Shutdown(t.Context()))
	})
}

func TestCollectorResourceAttributesArePopulated(t *testing.T) {
	// Prepare TelemetrySettings with Resource attributes
	tel := componenttest.NewNopTelemetrySettings()
	res := pcommon.NewResource()
	res.Attributes().PutStr("b_key", "2")
	res.Attributes().PutStr("a_key", "1")
	tel.Resource = res

	set := extension.Settings{
		TelemetrySettings: tel,
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

	ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
	require.NoError(t, err)
	ext.serializer = &mockSerializer{}
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	// Minimal config to trigger NotifyConfig
	conf := confmap.NewFromStringMap(map[string]any{})
	err = ext.NotifyConfig(t.Context(), conf)
	require.NoError(t, err)

	// Expect map with keys and values
	require.NotNil(t, ext.otelCollectorMetadata)
	// Verify the explicitly set attributes
	assert.Equal(t, "1", ext.otelCollectorMetadata.CollectorResourceAttributes["a_key"])
	assert.Equal(t, "2", ext.otelCollectorMetadata.CollectorResourceAttributes["b_key"])
	// host.ip should also be present (automatically collected)
	assert.Contains(t, ext.otelCollectorMetadata.CollectorResourceAttributes, "host.ip")

	// Cleanup
	assert.NoError(t, ext.Shutdown(t.Context()))
}

func TestCollectorResourceAttributesWithMultipleKeys(t *testing.T) {
	// Prepare TelemetrySettings with multiple Resource attributes
	tel := componenttest.NewNopTelemetrySettings()
	res := pcommon.NewResource()
	res.Attributes().PutStr("deployment.environment.name", "prod")
	res.Attributes().PutStr("team.name", "backend")
	res.Attributes().PutStr("cloud.region", "us-east")
	tel.Resource = res

	set := extension.Settings{
		TelemetrySettings: tel,
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

	ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
	require.NoError(t, err)
	ext.serializer = &mockSerializer{}
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	// Minimal config to trigger NotifyConfig
	conf := confmap.NewFromStringMap(map[string]any{})
	err = ext.NotifyConfig(t.Context(), conf)
	require.NoError(t, err)

	// Verify all resource attributes are collected
	require.NotNil(t, ext.otelCollectorMetadata)
	// Verify the explicitly set attributes
	assert.Equal(t, "prod", ext.otelCollectorMetadata.CollectorResourceAttributes["deployment.environment.name"])
	assert.Equal(t, "us-east", ext.otelCollectorMetadata.CollectorResourceAttributes["cloud.region"])
	assert.Equal(t, "backend", ext.otelCollectorMetadata.CollectorResourceAttributes["team.name"])
	// host.ip should also be present (automatically collected)
	assert.Contains(t, ext.otelCollectorMetadata.CollectorResourceAttributes, "host.ip")

	// Cleanup
	assert.NoError(t, ext.Shutdown(t.Context()))
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// Create a mock serializer that will cause SendPayload to fail
		mockSerializer := &mockFailingSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})

		// This should trigger the error path when SendPayload fails
		err = ext.NotifyConfig(t.Context(), conf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "payload send failed")

		// Cleanup
		assert.NoError(t, ext.Shutdown(t.Context()))
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}

		// Set moduleInfos to empty to trigger error path in PopulateFullComponentsJSON
		ext.info.modules = service.ModuleInfos{}

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
		})

		// This should trigger warning but not fail
		err = ext.NotifyConfig(t.Context(), conf)
		require.NoError(t, err) // Should not fail, just log warning

		// Cleanup
		assert.NoError(t, ext.Shutdown(t.Context()))
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

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
		err = ext.NotifyConfig(t.Context(), conf)
		require.NoError(t, err) // Should not fail, just log warning

		// Cleanup
		assert.NoError(t, ext.Shutdown(t.Context()))
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		mockSerializer := &mockSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		// NotifyConfig should start the periodic payload sending
		err = ext.NotifyConfig(t.Context(), conf)
		require.NoError(t, err)

		// Verify periodic sending components are initialized
		assert.NotNil(t, ext.payloadSender.ticker, "payload ticker should be initialized")
		assert.NotNil(t, ext.payloadSender.ctx, "payload context should be initialized")
		assert.NotNil(t, ext.payloadSender.cancel, "payload cancel function should be initialized")
		assert.NotNil(t, ext.payloadSender.channel, "payload send channel should be initialized")

		// Shutdown should stop periodic sending
		err = ext.Shutdown(t.Context())
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// Use a counting mock serializer to track payload sends
		mockSerializer := &countingMockSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		// NotifyConfig will send the initial payload
		err = ext.NotifyConfig(t.Context(), conf)
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
		err = ext.Shutdown(t.Context())
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		// Mock serializer that fails after the first call
		mockSerializer := &failAfterFirstCallSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		conf := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{"otlp": nil},
			"service":   map[string]any{"pipelines": map[string]any{"traces": map[string]any{"receivers": []any{"otlp"}}}},
		})
		ext.info.modules = service.ModuleInfos{Receiver: map[component.Type]service.ModuleInfo{
			component.MustNewType("otlp"): {BuilderRef: "gomod.example/otlp v1.0.0"},
		}}

		// NotifyConfig will succeed with the first payload
		err = ext.NotifyConfig(t.Context(), conf)
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
		err = ext.Shutdown(t.Context())
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
		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)

		mockSerializer := &mockSerializer{}
		ext.serializer = mockSerializer

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

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

		for i := range numGoroutines {
			wg.Add(1)
			go func(confIndex int) {
				defer wg.Done()
				conf := confs[confIndex%len(confs)]
				if err := ext.NotifyConfig(t.Context(), conf); err != nil {
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
		assert.NoError(t, ext.Shutdown(t.Context()))
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

func (*mockSerializer) SendMetadata(marshaler.JSONMarshaler) error            { return nil }
func (*mockSerializer) SendEvents(event.Events) error                         { return nil }
func (*mockSerializer) SendServiceChecks(servicecheck.ServiceChecks) error    { return nil }
func (*mockSerializer) SendIterableSeries(metrics.SerieSource) error          { return nil }
func (*mockSerializer) AreSeriesEnabled() bool                                { return false }
func (*mockSerializer) SendSketch(metrics.SketchesSource) error               { return nil }
func (*mockSerializer) AreSketchesEnabled() bool                              { return false }
func (*mockSerializer) SendHostMetadata(marshaler.JSONMarshaler) error        { return nil }
func (*mockSerializer) SendProcessesMetadata(any) error                       { return nil }
func (*mockSerializer) SendAgentchecksMetadata(marshaler.JSONMarshaler) error { return nil }
func (*mockSerializer) SendOrchestratorMetadata([]types.ProcessMessageBody, string, string, int) error {
	return nil
}

func (*mockSerializer) SendOrchestratorManifests([]types.ProcessMessageBody, string, string) error {
	return nil
}

// Mock serializer that fails SendPayload for testing error paths
type mockFailingSerializer struct {
	mockSerializer
}

func (*mockFailingSerializer) SendMetadata(marshaler.JSONMarshaler) error {
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

func TestHostIPCollection(t *testing.T) {
	t.Run("host.ip is collected as JSON array per OTel semantic conventions", func(t *testing.T) {
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

		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		// Verify that host.ip was collected
		require.NotNil(t, ext.info.resourceAttributes)
		hostIP, hasHostIP := ext.info.resourceAttributes["host.ip"]
		assert.True(t, hasHostIP, "host.ip should be present in resource attributes")
		assert.NotEmpty(t, hostIP, "host.ip should not be empty")
		t.Logf("Collected host.ip: %s", hostIP)

		// Verify it's a valid JSON array
		var ips []string
		err = json.Unmarshal([]byte(hostIP), &ips)
		require.NoError(t, err, "host.ip should be a valid JSON array")
		assert.NotEmpty(t, ips, "host.ip array should contain at least one IP")
		t.Logf("Parsed IPs: %v", ips)

		// Verify it's included in the payload
		conf := confmap.NewFromStringMap(map[string]any{})
		err = ext.NotifyConfig(t.Context(), conf)
		require.NoError(t, err)

		require.NotNil(t, ext.otelCollectorMetadata)
		assert.Contains(t, ext.otelCollectorMetadata.CollectorResourceAttributes, "host.ip")

		// Verify the payload also has valid JSON array
		payloadHostIP := ext.otelCollectorMetadata.CollectorResourceAttributes["host.ip"]
		var payloadIPs []string
		err = json.Unmarshal([]byte(payloadHostIP), &payloadIPs)
		require.NoError(t, err, "payload host.ip should be a valid JSON array")
		assert.Equal(t, ips, payloadIPs, "payload IPs should match collected IPs")

		// Cleanup
		assert.NoError(t, ext.Shutdown(t.Context()))
	})

	t.Run("host.ip combines with existing resource attributes", func(t *testing.T) {
		// Prepare TelemetrySettings with existing Resource attributes
		tel := componenttest.NewNopTelemetrySettings()
		res := pcommon.NewResource()
		res.Attributes().PutStr("service.name", "test-service")
		res.Attributes().PutStr("deployment.environment", "staging")
		tel.Resource = res

		set := extension.Settings{
			TelemetrySettings: tel,
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

		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		// Verify all attributes are present
		require.NotNil(t, ext.info.resourceAttributes)
		assert.Contains(t, ext.info.resourceAttributes, "service.name")
		assert.Contains(t, ext.info.resourceAttributes, "deployment.environment")
		assert.Contains(t, ext.info.resourceAttributes, "host.ip")
		assert.Equal(t, "test-service", ext.info.resourceAttributes["service.name"])
		assert.Equal(t, "staging", ext.info.resourceAttributes["deployment.environment"])

		// Verify host.ip is a valid JSON array
		hostIP := ext.info.resourceAttributes["host.ip"]
		assert.NotEmpty(t, hostIP)
		var ips []string
		err = json.Unmarshal([]byte(hostIP), &ips)
		require.NoError(t, err, "host.ip should be a valid JSON array")
		assert.NotEmpty(t, ips, "host.ip array should contain at least one IP")

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})

	t.Run("host.ip values are sorted for deterministic output", func(t *testing.T) {
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

		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		// Verify host.ip is present and sorted
		require.NotNil(t, ext.info.resourceAttributes)
		hostIP := ext.info.resourceAttributes["host.ip"]
		require.NotEmpty(t, hostIP)

		// Parse the JSON array
		var ips []string
		err = json.Unmarshal([]byte(hostIP), &ips)
		require.NoError(t, err, "host.ip should be a valid JSON array")
		require.NotEmpty(t, ips, "host.ip array should contain at least one IP")

		// Verify the IPs are sorted
		sortedIPs := make([]string, len(ips))
		copy(sortedIPs, ips)
		sort.Strings(sortedIPs)
		assert.Equal(t, sortedIPs, ips, "IPs should be in sorted order")

		t.Logf("IPs are properly sorted: %v", ips)

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})

	t.Run("host.ip merges with existing resource attribute (JSON array)", func(t *testing.T) {
		// Prepare TelemetrySettings with an existing host.ip as JSON array
		tel := componenttest.NewNopTelemetrySettings()
		res := pcommon.NewResource()
		// Set a pre-configured IP that might not be detected (e.g., external IP)
		existingIPs := []string{"203.0.113.5", "2001:db8::1"}
		existingJSON, _ := json.Marshal(existingIPs)
		res.Attributes().PutStr("host.ip", string(existingJSON))
		tel.Resource = res

		set := extension.Settings{
			TelemetrySettings: tel,
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

		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		// Verify host.ip contains both existing and detected IPs
		require.NotNil(t, ext.info.resourceAttributes)
		hostIP := ext.info.resourceAttributes["host.ip"]
		require.NotEmpty(t, hostIP)

		var ips []string
		err = json.Unmarshal([]byte(hostIP), &ips)
		require.NoError(t, err)

		// Should contain the pre-configured IPs
		assert.Contains(t, ips, "203.0.113.5", "Should contain pre-configured IPv4")
		assert.Contains(t, ips, "2001:db8::1", "Should contain pre-configured IPv6")

		// Should also contain at least some detected IPs (we don't know exact ones)
		assert.GreaterOrEqual(t, len(ips), len(existingIPs), "Should have at least the pre-configured IPs plus detected ones")

		// Verify still sorted
		sortedIPs := make([]string, len(ips))
		copy(sortedIPs, ips)
		sort.Strings(sortedIPs)
		assert.Equal(t, sortedIPs, ips, "Merged IPs should be sorted")

		t.Logf("Merged IPs (existing + detected): %v", ips)

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})

	t.Run("host.ip merges with existing resource attribute (single string - backwards compat)", func(t *testing.T) {
		// Prepare TelemetrySettings with an existing host.ip as a plain string (backwards compatibility)
		tel := componenttest.NewNopTelemetrySettings()
		res := pcommon.NewResource()
		res.Attributes().PutStr("host.ip", "203.0.113.5")
		tel.Resource = res

		set := extension.Settings{
			TelemetrySettings: tel,
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

		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		// Verify host.ip contains both existing string and detected IPs, converted to JSON array
		require.NotNil(t, ext.info.resourceAttributes)
		hostIP := ext.info.resourceAttributes["host.ip"]
		require.NotEmpty(t, hostIP)

		var ips []string
		err = json.Unmarshal([]byte(hostIP), &ips)
		require.NoError(t, err, "Should be converted to JSON array format")

		// Should contain the pre-configured IP
		assert.Contains(t, ips, "203.0.113.5", "Should contain pre-configured IP")

		// Should have multiple IPs (the string one plus detected ones)
		assert.Greater(t, len(ips), 1, "Should have the pre-configured IP plus detected ones")

		t.Logf("Merged IPs (string converted + detected): %v", ips)

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})

	t.Run("host.ip deduplicates overlapping IPs", func(t *testing.T) {
		// Get one of the actual detected IPs first by creating a temp extension
		tempSet := extension.Settings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			BuildInfo:         component.BuildInfo{Version: "1.2.3"},
		}
		tempHostProvider := &mockSourceProvider{source: source.Source{Kind: source.HostnameKind, Identifier: "test-host"}}
		tempUUIDProvider := &mockUUIDProvider{mockUUID: "temp-uuid"}
		tempCfg := &Config{
			API: datadogconfig.APIConfig{Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Site: "datadoghq.com"},
			HTTPConfig: &httpserver.Config{
				ServerConfig: confighttp.ServerConfig{Endpoint: "localhost:0"},
				Path:         "/test-path",
			},
		}
		tempExt, _ := newExtension(t.Context(), tempCfg, tempSet, tempHostProvider, tempUUIDProvider)
		tempExt.serializer = &mockSerializer{}
		_ = tempExt.Start(t.Context(), componenttest.NewNopHost())

		var detectedIPs []string
		if hostIP, exists := tempExt.info.resourceAttributes["host.ip"]; exists {
			_ = json.Unmarshal([]byte(hostIP), &detectedIPs)
		}
		_ = tempExt.Shutdown(context.Background())

		if len(detectedIPs) == 0 {
			t.Skip("No detected IPs available for deduplication test")
		}

		// Now create actual test with overlapping IPs
		tel := componenttest.NewNopTelemetrySettings()
		res := pcommon.NewResource()
		// Include one detected IP and one external IP
		overlappingIPs := []string{detectedIPs[0], "203.0.113.5"}
		existingJSON, _ := json.Marshal(overlappingIPs)
		res.Attributes().PutStr("host.ip", string(existingJSON))
		tel.Resource = res

		set := extension.Settings{
			TelemetrySettings: tel,
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

		ext, err := newExtension(t.Context(), cfg, set, hostProvider, uuidProvider)
		require.NoError(t, err)
		ext.serializer = &mockSerializer{}
		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		// Verify host.ip is deduplicated
		require.NotNil(t, ext.info.resourceAttributes)
		hostIP := ext.info.resourceAttributes["host.ip"]
		require.NotEmpty(t, hostIP)

		var ips []string
		err = json.Unmarshal([]byte(hostIP), &ips)
		require.NoError(t, err)

		// Count occurrences of the overlapping IP
		count := 0
		for _, ip := range ips {
			if ip == detectedIPs[0] {
				count++
			}
		}
		assert.Equal(t, 1, count, "Overlapping IP should appear exactly once (deduplicated)")
		assert.Contains(t, ips, "203.0.113.5", "Should contain the external IP")

		t.Logf("Deduplicated IPs: %v", ips)

		// Cleanup
		assert.NoError(t, ext.Shutdown(context.Background()))
	})
}
