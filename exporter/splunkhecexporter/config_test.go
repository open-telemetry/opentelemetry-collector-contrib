// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	// Endpoint and Token do not have a default value so set them directly.
	defaultCfg := createDefaultConfig().(*Config)
	defaultCfg.Token = "00000000-0000-0000-0000-0000000000000"
	defaultCfg.ClientConfig.Endpoint = "https://splunk:8088/services/collector"

	hundred := 100
	idleConnTimeout := 10 * time.Second

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				Token:                   "00000000-0000-0000-0000-0000000000000",
				Source:                  "otel",
				SourceType:              "otel",
				Index:                   "metrics",
				SplunkAppName:           "OpenTelemetry-Collector Splunk Exporter",
				SplunkAppVersion:        "v0.0.1",
				LogDataEnabled:          true,
				ProfilingDataEnabled:    true,
				ExportRaw:               true,
				MaxEventSize:            5 * 1024 * 1024,
				MaxContentLengthLogs:    2 * 1024 * 1024,
				MaxContentLengthMetrics: 2 * 1024 * 1024,
				MaxContentLengthTraces:  2 * 1024 * 1024,
				ClientConfig: confighttp.ClientConfig{
					Timeout:  10 * time.Second,
					Endpoint: "https://splunk:8088/services/collector",
					TLSSetting: configtls.ClientConfig{
						Config: configtls.Config{
							CAFile:   "",
							CertFile: "",
							KeyFile:  "",
						},
						InsecureSkipVerify: false,
					},
					MaxIdleConns:         &hundred,
					MaxIdleConnsPerHost:  &hundred,
					IdleConnTimeout:      &idleConnTimeout,
					HTTP2ReadIdleTimeout: 10 * time.Second,
					HTTP2PingTimeout:     10 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					MaxInterval:         1 * time.Minute,
					MaxElapsedTime:      10 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.QueueConfig{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				BatcherConfig: exporterbatcher.Config{
					Enabled:      true,
					FlushTimeout: time.Second,
					MinSizeConfig: exporterbatcher.MinSizeConfig{
						MinSizeItems: 1,
					},
					MaxSizeConfig: exporterbatcher.MaxSizeConfig{
						MaxSizeItems: 10,
					},
				},
				HecToOtelAttrs: splunk.HecToOtelAttrs{
					Source:     "mysource",
					SourceType: "mysourcetype",
					Index:      "myindex",
					Host:       "myhost",
				},
				HecFields: OtelToHecFields{
					SeverityText:   "myseverityfield",
					SeverityNumber: "myseveritynumfield",
				},
				HealthPath:            "/services/collector/health",
				HecHealthCheckEnabled: false,
				Heartbeat: HecHeartbeat{
					Interval: 30 * time.Second,
				},
				Telemetry: HecTelemetry{
					Enabled: true,
					OverrideMetricsNames: map[string]string{
						"otelcol_exporter_splunkhec_heartbeats_sent":   "app_heartbeats_success_total",
						"otelcol_exporter_splunkhec_heartbeats_failed": "app_heartbeats_failed_total",
					},
					ExtraAttributes: map[string]string{
						"customKey": "customVal",
					},
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

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name:    "default",
			cfg:     createDefaultConfig().(*Config),
			wantErr: "requires a non-empty \"endpoint\"",
		},
		{
			name: "bad url",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = "cache_object:foo/bar"
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "invalid \"endpoint\": parse \"cache_object:foo/bar\": first path segment in URL cannot contain colon",
		},
		{
			name: "missing token",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = "http://example.com"
				return cfg
			}(),
			wantErr: "requires a non-empty \"token\"",
		},
		{
			name: "max default content-length for logs",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = "http://foo_bar.com"
				cfg.MaxContentLengthLogs = maxContentLengthLogsLimit + 1
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "requires \"max_content_length_logs\" <= 838860800",
		},
		{
			name: "max default content-length for metrics",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = "http://foo_bar.com"
				cfg.MaxContentLengthMetrics = maxContentLengthMetricsLimit + 1
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "requires \"max_content_length_metrics\" <= 838860800",
		},
		{
			name: "max default content-length for traces",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = "http://foo_bar.com"
				cfg.MaxContentLengthTraces = maxContentLengthTracesLimit + 1
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "requires \"max_content_length_traces\" <= 838860800",
		},
		{
			name: "max default event-size",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = "http://foo_bar.com"
				cfg.MaxEventSize = maxMaxEventSize + 1
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "requires \"max_event_size\" <= 838860800",
		},
		{
			name: "negative queue size",
			cfg: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = "http://foo_bar.com"
				cfg.QueueSettings.Enabled = true
				cfg.QueueSettings.QueueSize = -5
				cfg.Token = "foo"
				return cfg
			}(),
			wantErr: "queue size must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := component.ValidateConfig(tt.cfg)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
