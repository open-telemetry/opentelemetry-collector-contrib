// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	clientConfigWithoutHeaders := confighttp.NewDefaultClientConfig()
	clientConfigWithoutHeaders.Endpoint = "localhost:8888"
	clientConfigWithoutHeaders.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/var/lib/mycert.pem", // This is subject to change, but currently I have no idea what else to put here lol
		},
		Insecure: false,
	}
	clientConfigWithoutHeaders.ReadBufferSize = 0
	clientConfigWithoutHeaders.WriteBufferSize = 512 * 1024
	clientConfigWithoutHeaders.Timeout = 5 * time.Second

	clientConfigWithHeaders := clientConfigWithoutHeaders
	clientConfigWithHeaders.Headers = configopaque.MapList{
		{Name: "Prometheus-Remote-Write-Version", Value: "0.1.0"},
		{Name: "X-Scope-OrgID", Value: "234"},
	}
	tests := []struct {
		id               component.ID
		expected         component.Config
		errorMessage     string
		enableSendingRW2 bool
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig = confighttp.ClientConfig{}
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				MaxBatchSizeBytes:          3000000,
				MaxBatchRequestParallelism: new(10),
				TimeoutSettings:            exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				RemoteWriteQueue: RemoteWriteQueue{
					Enabled:      true,
					QueueSize:    2000,
					NumConsumers: 10,
				},
				AddMetricSuffixes: false,
				Namespace:         "test-space",
				ExternalLabels:    map[string]string{"key1": "value1", "key2": "value2"},
				ClientConfig:      confighttp.ClientConfig{},
				HTTP:              clientConfigWithHeaders,
				//nolint:staticcheck // test deprecated field
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
				TargetInfo: TargetInfo{
					Enabled: true,
				},
				RemoteWriteProtoMsg: remoteapi.WriteV1MessageType,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "translation_strategy"),
			expected: &Config{
				MaxBatchSizeBytes:          3000000,
				MaxBatchRequestParallelism: nil,
				TimeoutSettings:            exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     50 * time.Millisecond,
					RandomizationFactor: 0.5,
					Multiplier:          1.5,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
				},
				RemoteWriteQueue: RemoteWriteQueue{
					Enabled:      true,
					QueueSize:    10000,
					NumConsumers: 5,
				},
				ExternalLabels:      map[string]string{},
				AddMetricSuffixes:   true,
				TranslationStrategy: "NoTranslation",
				ClientConfig:        confighttp.ClientConfig{},
				HTTP: func() confighttp.ClientConfig {
					cc := confighttp.NewDefaultClientConfig()
					cc.Endpoint = "localhost:8888"
					cc.WriteBufferSize = 512 * 1024
					cc.Timeout = 5 * time.Second
					return cc
				}(),
				RemoteWriteProtoMsg: remoteapi.WriteV2MessageType,
				TargetInfo: TargetInfo{
					Enabled: true,
				},
			},
			enableSendingRW2: true,
		},

		{
			id:           component.NewIDWithName(metadata.Type, "negative_queue_size"),
			errorMessage: "remote write queue size can't be negative",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "negative_num_consumers"),
			errorMessage: "remote write consumer number can't be negative",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "less_than_1_max_batch_request_parallelism"),
			errorMessage: "max_batch_request_parallelism can't be set to below 1",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "non_snappy_compression_type"),
			errorMessage: "compression type must be snappy",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "unknown_protobuf_message"),
			errorMessage: "unknown type for remote write protobuf message io.prometheus.write.v4.Request, supported: prometheus.WriteRequest, io.prometheus.write.v2.Request",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_translation_strategy"),
			errorMessage: "invalid translation_strategy: invalid_strategy",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "v1_no_utf8"),
			errorMessage: "translation strategy NoUTF8EscapingWithSuffixes requires Prometheus Remote Write 2.0",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "v1_no_translation"),
			errorMessage: "translation strategy NoTranslation requires Prometheus Remote Write 2.0",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "reserved_metadata_keys"),
			errorMessage: "include_metadata_keys entry \"content-type\" collides with a reserved remote write header",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "negative_max_batch_size_bytes"),
			errorMessage: "max_batch_size_bytes must be greater than 0",
		},
		{
			id: component.NewIDWithName(metadata.Type, "include_metadata_keys"),
			expected: &Config{
				MaxBatchSizeBytes:          3000000,
				MaxBatchRequestParallelism: nil,
				TimeoutSettings:            exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     50 * time.Millisecond,
					RandomizationFactor: 0.5,
					Multiplier:          1.5,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
				},
				RemoteWriteQueue: RemoteWriteQueue{
					Enabled:      true,
					QueueSize:    1000,
					NumConsumers: 5,
				},
				IncludeMetadataKeys: []string{"target-id", "x-org-id"},
				ExternalLabels:      map[string]string{},
				AddMetricSuffixes:   true,
				ClientConfig:        confighttp.ClientConfig{},
				HTTP: func() confighttp.ClientConfig {
					cc := confighttp.NewDefaultClientConfig()
					cc.Endpoint = "localhost:8888"
					cc.WriteBufferSize = 512 * 1024
					cc.Timeout = 5 * time.Second
					return cc
				}(),
				RemoteWriteProtoMsg: remoteapi.WriteV1MessageType,
				TargetInfo: TargetInfo{
					Enabled: true,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "resource_constant_labels"),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig = confighttp.ClientConfig{}
				cfg.HTTP.Endpoint = "localhost:8888"
				cfg.ResourceConstantLabels = resourcetotelemetry.Settings{
					Included: []string{"service*"},
					Excluded: []string{"service.attr1"},
				}
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			if tt.enableSendingRW2 {
				defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusremotewritexporterEnableSendingRW2FeatureGate, true)()
			}

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.ErrorContains(t, confmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestDisabledQueue(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "disabled_queue").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.False(t, cfg.(*Config).RemoteWriteQueue.Enabled)
}

func TestDisabledTargetInfo(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "disabled_target_info").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.False(t, cfg.(*Config).TargetInfo.Enabled)
}

func TestHTTPOverridesFlatConfig(t *testing.T) {
	flatEndpoint := "http://flat.example.com"
	httpEndpoint := "http://http.example.com"
	flatTimeout := 15 * time.Second
	httpTimeout := 10 * time.Second

	testCases := []struct {
		name                string
		featureGateEnabled  bool
		conf                map[string]any
		wantErr             string
		wantEndpoint        string
		wantTimeout         time.Duration
		wantExporterTimeout time.Duration
		checkDefaults       bool
	}{
		{
			name:               "gate disabled, http block set overrides flat",
			featureGateEnabled: false,
			conf: map[string]any{
				"endpoint": flatEndpoint,
				"timeout":  flatTimeout.String(),
				"http": map[string]any{
					"endpoint": httpEndpoint,
				},
			},
			wantEndpoint: httpEndpoint,
			// This also ensures that if http.timeout is not set, we fallback to correct defaults
			wantTimeout: getDefaultHTTPClientConfig().Timeout,
			// Top-level timeout still configures the exporter helper; http owns the client timeout.
			wantExporterTimeout: flatTimeout,
			checkDefaults:       true,
		},
		{
			name:               "gate disabled, http block unset keeps flat",
			featureGateEnabled: false,
			conf: map[string]any{
				"endpoint": flatEndpoint,
				"timeout":  flatTimeout.String(),
			},
			wantEndpoint: flatEndpoint,
			wantTimeout:  flatTimeout,
		},
		{
			name:               "gate enabled, http block set without flat",
			featureGateEnabled: true,
			conf: map[string]any{
				"http": map[string]any{
					"endpoint": httpEndpoint,
					"timeout":  httpTimeout.String(),
				},
			},
			wantEndpoint:  httpEndpoint,
			wantTimeout:   httpTimeout,
			checkDefaults: true,
		},
		{
			name:               "gate enabled, flat settings rejected even when http is set",
			featureGateEnabled: true,
			conf: map[string]any{
				"endpoint": flatEndpoint,
				"http": map[string]any{
					"endpoint": httpEndpoint,
					"timeout":  httpTimeout.String(),
				},
			},
			wantErr: "top-level HTTP client settings are not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusremotewritexporterRemoveTopLevelHTTPSettingsFeatureGate, tc.featureGateEnabled)()

			cfg := createDefaultConfig().(*Config)
			err := confmap.NewFromStringMap(tc.conf).Unmarshal(cfg)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantEndpoint, cfg.HTTP.Endpoint)
			require.Equal(t, tc.wantTimeout, cfg.HTTP.Timeout)
			if tc.wantExporterTimeout != 0 {
				require.Equal(t, tc.wantExporterTimeout, cfg.TimeoutSettings.Timeout)
			}
			if tc.checkDefaults {
				require.Equal(t, getDefaultHTTPClientConfig().WriteBufferSize, cfg.HTTP.WriteBufferSize)
				require.Equal(t, getDefaultHTTPClientConfig().MaxIdleConns, cfg.HTTP.MaxIdleConns) //nolint:staticcheck // SA1019: MaxIdleConns is deprecated but still in use, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316
			}
		})
	}
}

func TestResourceConstantLabelsValidation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ResourceConstantLabels = resourcetotelemetry.Settings{
		Included: []string{"service*"},
	}
	assert.NoError(t, cfg.Validate())

	invalidCfg := createDefaultConfig().(*Config)
	invalidCfg.ResourceConstantLabels.Enabled = true //nolint:staticcheck // testing deprecated field rejection
	assert.Error(t, invalidCfg.Validate())

	cfg.ResourceToTelemetrySettings.Enabled = true //nolint:staticcheck // ignore deprecated field
	assert.Error(t, cfg.Validate())

	defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusremotewriteDisableResourceToTelemetryConversionFeatureGate, true)()
	assert.Error(t, cfg.Validate())

	cfg.ResourceToTelemetrySettings = resourcetotelemetry.Settings{}
	assert.NoError(t, cfg.Validate())
}
