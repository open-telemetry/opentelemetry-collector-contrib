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
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/metadata"
)

func TestConfigValidation(t *testing.T) {
	emptyEndpointClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	emptyEndpointClientConfig.MaxIdleConns = 0
	emptyEndpointClientConfig.IdleConnTimeout = 0
	emptyEndpointClientConfig.ForceAttemptHTTP2 = false
	emptyEndpointClientConfig.Endpoint = ""

	missingSchemeClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	missingSchemeClientConfig.MaxIdleConns = 0
	missingSchemeClientConfig.IdleConnTimeout = 0
	missingSchemeClientConfig.ForceAttemptHTTP2 = false
	missingSchemeClientConfig.Endpoint = "test.com/dummy"

	invalidFormatClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	invalidFormatClientConfig.MaxIdleConns = 0
	invalidFormatClientConfig.IdleConnTimeout = 0
	invalidFormatClientConfig.ForceAttemptHTTP2 = false
	invalidFormatClientConfig.Endpoint = "invalid.com@#$%"

	validClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	validClientConfig.MaxIdleConns = 0
	validClientConfig.IdleConnTimeout = 0
	validClientConfig.ForceAttemptHTTP2 = false
	validClientConfig.Endpoint = "http://validurl.com/rest"

	testcases := []struct {
		name         string
		cfg          *Config
		wantErr      bool
		errorMessage string
	}{
		{
			name: "empty endpoint",
			cfg: &Config{
				ClientConfig: emptyEndpointClientConfig,
			},
			wantErr:      true,
			errorMessage: "endpoint should not be empty",
		},
		{
			name: "missing http scheme",
			cfg: &Config{
				ClientConfig: missingSchemeClientConfig,
			},
			wantErr:      true,
			errorMessage: "endpoint must be valid",
		},
		{
			name: "invalid endpoint format",
			cfg: &Config{
				ClientConfig: invalidFormatClientConfig,
			},
			wantErr:      true,
			errorMessage: "endpoint must be valid",
		},
		{
			name: "valid config",
			cfg: &Config{
				ClientConfig: validClientConfig,
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
				if tc.errorMessage != "" {
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

	apitokenClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	apitokenClientConfig.MaxIdleConns = 0
	apitokenClientConfig.IdleConnTimeout = 0
	apitokenClientConfig.ForceAttemptHTTP2 = false
	apitokenClientConfig.Endpoint = "https://company.logicmonitor.com/rest"

	bearertokenClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	bearertokenClientConfig.MaxIdleConns = 0
	bearertokenClientConfig.IdleConnTimeout = 0
	bearertokenClientConfig.ForceAttemptHTTP2 = false
	bearertokenClientConfig.Endpoint = "https://company.logicmonitor.com/rest"
	bearertokenClientConfig.Headers = configopaque.MapList{
		{Name: "Authorization", Value: "Bearer <token>"},
	}

	resourceMappingClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	resourceMappingClientConfig.MaxIdleConns = 0
	resourceMappingClientConfig.IdleConnTimeout = 0
	resourceMappingClientConfig.ForceAttemptHTTP2 = false
	resourceMappingClientConfig.Endpoint = "https://company.logicmonitor.com/rest"
	resourceMappingClientConfig.Headers = configopaque.MapList{
		{Name: "Authorization", Value: "Bearer <token>"},
	}

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "apitoken"),
			expected: &Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:  apitokenClientConfig,
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
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:  bearertokenClientConfig,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "resource-mapping-op"),
			expected: &Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:  resourceMappingClientConfig,
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

			assert.NoError(t, xconfmap.Validate(cfg))
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
			err: "'logs.resource_mapping_op' unsupported mapping operation \"invalid_op\"",
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
