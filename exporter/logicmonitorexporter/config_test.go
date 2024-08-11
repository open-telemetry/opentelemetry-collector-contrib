// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/metadata"
)

func TestConfigValidation(t *testing.T) {
	testcases := []struct {
		name         string
		cfg          *Config
		wantErr      bool
		errorMessage string
	}{
		{
			name: "empty endpoint",
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "",
				},
			},
			wantErr:      true,
			errorMessage: "endpoint should not be empty",
		},
		{
			name: "missing http scheme",
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "test.com/dummy",
				},
			},
			wantErr:      true,
			errorMessage: "endpoint must be valid",
		},
		{
			name: "invalid endpoint format",
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "invalid.com@#$%",
				},
			},
			wantErr:      true,
			errorMessage: "endpoint must be valid",
		},
		{
			name: "valid config",
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://validurl.com/rest",
				},
			},
			wantErr:      false,
			errorMessage: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("config validation failed: error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if tc.wantErr {
				assert.Error(t, err)
				if len(tc.errorMessage) != 0 {
					assert.Equal(t, errors.New(tc.errorMessage), err, "Error messages must match")
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "apitoken"),
			expected: &Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://company.logicmonitor.com/rest",
				},
				APIToken: APIToken{
					AccessID:  "accessid",
					AccessKey: "accesskey",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "bearertoken"),
			expected: &Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://company.logicmonitor.com/rest",
					Headers: map[string]configopaque.String{
						"Authorization": "Bearer <token>",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "resource-mapping-op"),
			expected: &Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://company.logicmonitor.com/rest",
					Headers: map[string]configopaque.String{
						"Authorization": "Bearer <token>",
					},
				},
				Logs: LogsConfig{
					ResourceMappingOperation: "or",
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

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		configMap *confmap.Conf
		cfg       *Config
		err       string
	}{
		{
			name: "invalid resource mapping operation",
			configMap: confmap.NewFromStringMap(map[string]any{
				"logs": map[string]any{
					"resource_mapping_op": "invalid_op",
				},
			}),
			err: "'logs.resource_mapping_op': unsupported mapping operation \"invalid_op\"",
		},
	}

	f := NewFactory()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := f.CreateDefaultConfig().(*Config)
			err := tt.configMap.Unmarshal(cfg)
			if err != nil || tt.err != "" {
				assert.ErrorContains(t, err, tt.err)
			} else {
				assert.Equal(t, tt.cfg, cfg)
			}
		})
	}
}
