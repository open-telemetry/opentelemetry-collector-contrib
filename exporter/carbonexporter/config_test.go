// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "localhost:8080",
				},
				MaxIdleConns: 15,
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 10 * time.Second,
				},
				RetryConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					RandomizationFactor: 0.7,
					Multiplier:          3.14,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
				},
				QueueConfig: exporterhelper.QueueBatchConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
				},
				ResourceToTelemetryConfig: resourcetotelemetry.Settings{
					Enabled: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:   "default_config",
			config: createDefaultConfig().(*Config),
		},
		{
			name: "invalid_tcp_addr",
			config: &Config{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "http://localhost:2003",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_timeout",
			config: &Config{
				TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: defaultEndpoint},
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: -5 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid_max_idle_conns",
			config: &Config{
				TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: defaultEndpoint},
				MaxIdleConns:  -1,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				assert.Error(t, tt.config.Validate())
			} else {
				assert.NoError(t, tt.config.Validate())
			}
		})
	}
}
