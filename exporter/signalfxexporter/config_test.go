// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter

import (
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	apmcorrelation "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	seventy := 70
	hundred := 100
	idleConnTimeout := 30 * time.Second

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				AccessToken: "testToken",
				Realm:       "ap0",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:              10 * time.Second,
					Headers:              nil,
					MaxIdleConns:         &hundred,
					MaxIdleConnsPerHost:  &hundred,
					IdleConnTimeout:      &idleConnTimeout,
					HTTP2ReadIdleTimeout: 10 * time.Second,
					HTTP2PingTimeout:     10 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: true,
				},
				LogDimensionUpdates: false,
				DimensionClient: DimensionClientConfig{
					MaxBuffered:         10000,
					SendDelay:           10 * time.Second,
					MaxIdleConns:        20,
					MaxIdleConnsPerHost: 20,
					MaxConnsPerHost:     20,
					IdleConnTimeout:     30 * time.Second,
					Timeout:             10 * time.Second,
				},
				TranslationRules:    nil,
				ExcludeMetrics:      nil,
				IncludeMetrics:      nil,
				DeltaTranslationTTL: 3600,
				ExcludeProperties:   nil,
				Correlation: &correlation.Config{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "",
						Timeout:  5 * time.Second,
					},
					StaleServiceTimeout: 5 * time.Minute,
					SyncAttributes: map[string]string{
						"k8s.pod.uid":  "k8s.pod.uid",
						"container.id": "container.id",
					},
					Config: apmcorrelation.Config{
						MaxRequests:     20,
						MaxBuffered:     10_000,
						MaxRetries:      2,
						LogUpdates:      false,
						RetryDelay:      30 * time.Second,
						CleanupInterval: 1 * time.Minute,
					},
				},
				NonAlphanumericDimensionChars: "_-.",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				AccessToken: "testToken",
				Realm:       "us1",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout: 2 * time.Second,
					Headers: map[string]configopaque.String{
						"added-entry": "added value",
						"dot.test":    "test",
					},
					MaxIdleConns:         &seventy,
					MaxIdleConnsPerHost:  &seventy,
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
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				}, AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: false,
				},
				LogDimensionUpdates: true,
				DimensionClient: DimensionClientConfig{
					MaxBuffered:         1,
					SendDelay:           time.Hour,
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 10,
					MaxConnsPerHost:     10000,
					IdleConnTimeout:     2 * time.Hour,
					Timeout:             20 * time.Second,
				},
				TranslationRules: []translation.Rule{
					{
						Action: translation.ActionRenameDimensionKeys,
						Mapping: map[string]string{
							"k8s.cluster.name": "kubernetes_cluster",
						},
					},
					{
						Action: translation.ActionDropDimensions,
						DimensionPairs: map[string]map[string]bool{
							"foo":  nil,
							"foo1": {"bar": true},
						},
					},
					{
						Action:     translation.ActionDropDimensions,
						MetricName: "metric",
						DimensionPairs: map[string]map[string]bool{
							"foo":  nil,
							"foo1": {"bar": true},
						},
					},
					{
						Action: translation.ActionDropDimensions,
						MetricNames: map[string]bool{
							"metric1": true,
							"metric2": true,
						},
						DimensionPairs: map[string]map[string]bool{
							"foo":  nil,
							"foo1": {"bar": true},
						},
					},
				},
				ExcludeMetrics: []dpfilters.MetricFilter{
					{
						MetricName: "metric1",
					},
					{
						MetricNames: []string{"metric2", "metric3"},
					},
					{
						MetricName: "metric4",
						Dimensions: map[string]any{
							"dimension_key": "dimension_val",
						},
					},
					{
						MetricName: "metric5",
						Dimensions: map[string]any{
							"dimension_key": []any{"dimension_val1", "dimension_val2"},
						},
					},
					{
						MetricName: `/cpu\..*/`,
					},
					{
						MetricNames: []string{"cpu.util*", "memory.util*"},
					},
					{
						MetricName: "cpu.utilization",
						Dimensions: map[string]any{
							"container_name": "/^[A-Z][A-Z]$/",
						},
					},
				},
				IncludeMetrics: []dpfilters.MetricFilter{
					{
						MetricName: "metric1",
					},
					{
						MetricNames: []string{"metric2", "metric3"},
					},
				},
				DeltaTranslationTTL: 3600,
				ExcludeProperties: []dpfilters.PropertyFilter{
					{
						PropertyName: mustStringFilter(t, "globbed*"),
					},
					{
						PropertyValue: mustStringFilter(t, "!globbed*value"),
					},
					{
						DimensionName: mustStringFilter(t, "globbed*"),
					},
					{
						DimensionValue: mustStringFilter(t, "!globbed*value"),
					},
					{
						PropertyName:   mustStringFilter(t, "globbed*"),
						PropertyValue:  mustStringFilter(t, "!globbed*value"),
						DimensionName:  mustStringFilter(t, "globbed*"),
						DimensionValue: mustStringFilter(t, "!globbed*value"),
					},
				},
				Correlation: &correlation.Config{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "",
						Timeout:  5 * time.Second,
					},
					StaleServiceTimeout: 5 * time.Minute,
					SyncAttributes: map[string]string{
						"k8s.pod.uid":  "k8s.pod.uid",
						"container.id": "container.id",
					},
					Config: apmcorrelation.Config{
						MaxRequests:     20,
						MaxBuffered:     10_000,
						MaxRetries:      2,
						LogUpdates:      false,
						RetryDelay:      30 * time.Second,
						CleanupInterval: 1 * time.Minute,
					},
				},
				NonAlphanumericDimensionChars: "_-.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			// We need to add the default exclude rules.
			assert.NoError(t, setDefaultExcludes(tt.expected))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigGetMetricTranslator(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		want    *translation.MetricTranslator
		wantErr bool
	}{
		{
			name: "Test empty config",
			cfg: &Config{
				DeltaTranslationTTL: 3600,
			},
			want: func() *translation.MetricTranslator {
				translator, err := translation.NewMetricTranslator(defaultTranslationRules, 3600)
				require.NoError(t, err)
				return translator
			}(),
		},
		{
			name: "Test empty rules",
			cfg: &Config{
				TranslationRules:    []translation.Rule{},
				DeltaTranslationTTL: 3600,
			},
			want: func() *translation.MetricTranslator {
				translator, err := translation.NewMetricTranslator([]translation.Rule{}, 3600)
				require.NoError(t, err)
				return translator
			}(),
		},
		{
			name: "Test disable rules",
			cfg: &Config{
				DisableDefaultTranslationRules: true,
				DeltaTranslationTTL:            3600,
			},
			want: func() *translation.MetricTranslator {
				translator, err := translation.NewMetricTranslator([]translation.Rule{}, 3600)
				require.NoError(t, err)
				return translator
			}(),
		},
		{
			name: "Test disable rules overrides rules",
			cfg: &Config{
				TranslationRules:               []translation.Rule{{Action: translation.ActionDropDimensions}},
				DisableDefaultTranslationRules: true,
				DeltaTranslationTTL:            3600,
			},
			want: func() *translation.MetricTranslator {
				translator, err := translation.NewMetricTranslator([]translation.Rule{}, 3600)
				require.NoError(t, err)
				return translator
			}(),
		},
		{
			name: "Test invalid translation rules",
			cfg: &Config{
				Realm:       "us0",
				AccessToken: "access_token",
				TranslationRules: []translation.Rule{
					{
						Action: translation.ActionRenameDimensionKeys,
					},
				},
				DeltaTranslationTTL: 3600,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.getMetricTranslator(zap.NewNop())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigGetIngestURL(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		want    *url.URL
		wantErr bool
	}{
		{
			name: "Test URL from Realm",
			cfg: &Config{
				Realm: "us0",
			},
			want: &url.URL{
				Scheme: "https",
				Host:   "ingest.us0.signalfx.com",
				Path:   "",
			},
		},
		{
			name: "Test URL overrides",
			cfg: &Config{
				Realm:     "us0",
				IngestURL: "https://ingest.us1.signalfx.com/",
			},
			want: &url.URL{
				Scheme: "https",
				Host:   "ingest.us1.signalfx.com",
				Path:   "/",
			},
		},
		{
			name: "Test invalid URL",
			cfg: &Config{
				IngestURL: "https://api us1 signalfx com/",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.getIngestURL()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigGetAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		want    *url.URL
		wantErr bool
	}{
		{
			name: "Test URL from Realm",
			cfg: &Config{
				Realm: "us0",
			},
			want: &url.URL{
				Scheme: "https",
				Host:   "api.us0.signalfx.com",
			},
		},
		{
			name: "Test URL overrides",
			cfg: &Config{
				Realm:  "us0",
				APIURL: "https://api.us1.signalfx.com/",
			},
			want: &url.URL{
				Scheme: "https",
				Host:   "api.us1.signalfx.com",
				Path:   "/",
			},
		},
		{
			name: "Test invalid URL",
			cfg: &Config{
				APIURL: "https://api us1 signalfx com/",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.getAPIURL()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "Test empty config",
			cfg:  &Config{},
		},
		{
			name: "Test empty realm and API URL",
			cfg: &Config{
				AccessToken: "access_token",
				IngestURL:   "https://ingest.us1.signalfx.com/",
			},
		},
		{
			name: "Test empty realm and Ingest URL",
			cfg: &Config{
				AccessToken: "access_token",
				APIURL:      "https://api.us1.signalfx.com/",
			},
		},
		{
			name: "Negative Timeout",
			cfg: &Config{
				Realm:              "us0",
				AccessToken:        "access_token",
				HTTPClientSettings: confighttp.HTTPClientSettings{Timeout: -1 * time.Second},
			},
		},
		{
			name: "Negative QueueSize",
			cfg: &Config{
				Realm:       "us0",
				AccessToken: "access_token",
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:   true,
					QueueSize: -1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, component.ValidateConfig(tt.cfg))
		})
	}
}

func TestUnmarshalExcludeMetrics(t *testing.T) {
	tests := []struct {
		name              string
		cfg               *Config
		excludeMetricsLen int
	}{
		{
			name:              "empty config",
			cfg:               &Config{},
			excludeMetricsLen: 12,
		},
		{
			name: "existing exclude config",
			cfg: &Config{
				ExcludeMetrics: []dpfilters.MetricFilter{
					{
						MetricNames: []string{"metric1"},
					},
				},
			},
			excludeMetricsLen: 13,
		},
		{
			name: "existing empty exclude config",
			cfg: &Config{
				ExcludeMetrics: []dpfilters.MetricFilter{},
			},
			excludeMetricsLen: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.cfg.Unmarshal(confmap.NewFromStringMap(map[string]any{})))
			assert.Len(t, tt.cfg.ExcludeMetrics, tt.excludeMetricsLen)
		})
	}
}

func mustStringFilter(t *testing.T, filter string) *dpfilters.StringFilter {
	sf, err := dpfilters.NewStringFilter([]string{filter})
	require.NoError(t, err)
	return sf
}
