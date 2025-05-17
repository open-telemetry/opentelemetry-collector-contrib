// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	exp "go.opentelemetry.io/collector/exporter"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
)

func TestSignalConfigWrapper(t *testing.T) {
	config := &configgrpc.ClientConfig{
		Endpoint:     "test-endpoint:4317",
		WaitForReady: true,
	}
	wrapper := &signalConfigWrapper{config: config}

	t.Run("GetEndpoint", func(t *testing.T) {
		assert.Equal(t, "test-endpoint:4317", wrapper.GetEndpoint())
	})

	t.Run("GetWaitForReady", func(t *testing.T) {
		assert.True(t, wrapper.GetWaitForReady())
	})

	t.Run("ToClientConn", func(_ *testing.T) {
		ctx := context.Background()
		host := &mockHost{}
		settings := component.TelemetrySettings{}

		conn, _ := wrapper.ToClientConn(ctx, host, settings)
		if conn != nil {
			_ = conn.Close()
			// Wait for the connection to be fully closed
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn.WaitForStateChange(ctx, connectivity.Ready)
		}
		// No assertion on error, just ensure call is safe
	})
}

func TestNewSignalExporter(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		settings       exp.Settings
		signalEndpoint string
		headers        map[string]configopaque.String
		wantErr        bool
	}{
		{
			name:           "valid config with domain",
			config:         &Config{Domain: "test-domain"},
			settings:       exp.Settings{},
			signalEndpoint: "",
			headers:        nil,
			wantErr:        false,
		},
		{
			name:           "valid config with signal endpoint",
			config:         &Config{},
			settings:       exp.Settings{},
			signalEndpoint: "test-endpoint:4317",
			headers:        nil,
			wantErr:        false,
		},
		{
			name:           "invalid config - no domain or endpoint",
			config:         &Config{},
			settings:       exp.Settings{},
			signalEndpoint: "",
			headers:        nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := newSignalExporter(tt.config, tt.settings, tt.signalEndpoint, tt.headers)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, exporter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exporter)
				assert.Equal(t, tt.config, exporter.config)
			}
		})
	}
}

func TestSignalExporter_Shutdown(t *testing.T) {
	tests := []struct {
		name     string
		exporter *signalExporter
		wantErr  bool
	}{
		{
			name:     "nil client connection",
			exporter: &signalExporter{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.exporter.shutdown(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSignalExporter_EnhanceContext(t *testing.T) {
	tests := []struct {
		name     string
		exporter *signalExporter
		wantMD   metadata.MD
	}{
		{
			name:     "empty metadata",
			exporter: &signalExporter{},
			wantMD:   nil,
		},
		{
			name: "with metadata",
			exporter: &signalExporter{
				metadata: metadata.New(map[string]string{
					"key": "value",
				}),
			},
			wantMD: metadata.New(map[string]string{
				"key": "value",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			enhancedCtx := tt.exporter.enhanceContext(ctx)

			if tt.wantMD == nil {
				md, ok := metadata.FromOutgoingContext(enhancedCtx)
				assert.False(t, ok)
				assert.Nil(t, md)
			} else {
				md, ok := metadata.FromOutgoingContext(enhancedCtx)
				assert.True(t, ok)
				assert.Equal(t, tt.wantMD, md)
			}
		})
	}
}

func TestSignalExporter_StartSignalExporter(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		signalConfig signalConfig
	}{
		{
			name:   "valid signal endpoint",
			config: &Config{},
			signalConfig: &signalConfigWrapper{
				config: &configgrpc.ClientConfig{
					Endpoint: "test-endpoint:4317",
				},
			},
		},
		{
			name: "valid domain",
			config: &Config{
				Domain:     "test-domain",
				PrivateKey: configopaque.String("test-key"),
			},
			signalConfig: &signalConfigWrapper{
				config: &configgrpc.ClientConfig{},
			},
		},
		{
			name:   "no endpoint or domain",
			config: &Config{},
			signalConfig: &signalConfigWrapper{
				config: &configgrpc.ClientConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			exporter := &signalExporter{
				config:   tt.config,
				settings: component.TelemetrySettings{},
			}

			_ = exporter.startSignalExporter(context.Background(), &mockHost{}, tt.signalConfig)
			if exporter.clientConn != nil {
				_ = exporter.clientConn.Close()
				// Wait for the connection to be fully closed
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				exporter.clientConn.WaitForStateChange(ctx, connectivity.Ready)
			}
			// No assertion on error, just ensure call is safe and close connection to avoid goroutine leaks
		})
	}
}

// mockHost implements component.Host interface for testing
type mockHost struct{}

func (m *mockHost) ReportFatalError(_ error) {}
func (m *mockHost) GetFactory(_ component.Kind, _ component.Type) component.Factory {
	return nil
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (m *mockHost) GetExporters() map[component.Type]map[component.ID]component.Component {
	return nil
}
