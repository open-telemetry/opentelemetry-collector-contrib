// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"net/url"
	"testing"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

func TestReceiver(t *testing.T) {
	f := NewFactory()
	tests := []struct {
		name              string
		useServiceAccount bool
		wantError         bool
	}{
		{
			name: "success",
		},
		{
			name:              "fails to get prometheus config",
			useServiceAccount: true,
			wantError:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.UseServiceAccount = tt.useServiceAccount

			r, err := f.CreateMetrics(
				t.Context(),
				receivertest.NewNopSettings(metadata.Type),
				cfg,
				consumertest.NewNop(),
			)

			if !tt.wantError {
				require.NoError(t, err)
				require.NotNil(t, r)

				require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(t.Context()))
				return
			}

			require.Error(t, r.Start(t.Context(), componenttest.NewNopHost()))
		})
	}
}

func TestGetPrometheusConfig(t *testing.T) {
	clientConfigTLS := confighttp.NewDefaultClientConfig()
	clientConfigTLS.Endpoint = "localhost:1234"
	clientConfigTLS.TLS = configtls.ClientConfig{
		Insecure: true,
	}

	clientConfigCA := confighttp.NewDefaultClientConfig()
	clientConfigCA.Endpoint = "localhost:1234"
	clientConfigCA.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "./testdata/test_cert.pem",
		},
		InsecureSkipVerify: true,
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"

	clientConfigJobName := confighttp.NewDefaultClientConfig()
	clientConfigJobName.Endpoint = "localhost:1234"

	clientConfigWithTimeout := func(timeout time.Duration) confighttp.ClientConfig {
		c := confighttp.NewDefaultClientConfig()
		c.Endpoint = "localhost:1234"
		c.Timeout = timeout
		return c
	}

	wantWithScrapeTimeout := func(scrapeTimeout time.Duration) *prometheusreceiver.Config {
		return &prometheusreceiver.Config{
			PrometheusConfig: &prometheusreceiver.PromConfig{
				GlobalConfig: config.DefaultGlobalConfig,
				ScrapeConfigs: []*config.ScrapeConfig{
					{
						JobName:         "prometheus_simple/localhost:1234",
						HonorTimestamps: true,
						ScrapeInterval:  model.Duration(10 * time.Second),
						ScrapeTimeout:   model.Duration(scrapeTimeout),
						MetricsPath:     "/metrics",
						Scheme:          "https",
						ServiceDiscoveryConfigs: discovery.Configs{
							discovery.StaticConfig{
								{
									Targets: []model.LabelSet{
										{model.AddressLabel: model.LabelValue("localhost:1234")},
									},
									Labels: model.LabelSet{},
								},
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name     string
		config   *Config
		want     *prometheusreceiver.Config
		wantWarn bool
	}{
		{
			name: "Test without TLS",
			config: &Config{
				ClientConfig:       clientConfigTLS,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				Params:             url.Values{"foo": []string{"bar", "foobar"}},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							Scheme:          "http",
							MetricsPath:     "/metric",
							Params:          url.Values{"foo": []string{"bar", "foobar"}},
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
										Labels: map[model.LabelName]model.LabelValue{},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with job name",
			config: &Config{
				ClientConfig:       clientConfigJobName,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				JobName:            "job123",
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "job123",
							HonorTimestamps: true,
							Scheme:          "https",
							MetricsPath:     "/metric",
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
										Labels: map[model.LabelName]model.LabelValue{},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with TLS",
			config: &Config{
				ClientConfig:       clientConfigCA,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							MetricsPath:     "/metrics",
							Scheme:          "https",
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
										Labels: map[model.LabelName]model.LabelValue{},
									},
								},
							},
							HTTPClientConfig: configutil.HTTPClientConfig{
								TLSConfig: configutil.TLSConfig{
									CAFile:             "./testdata/test_cert.pem",
									InsecureSkipVerify: true,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with TLS - default CA",
			config: &Config{
				ClientConfig:       clientConfig,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
				Labels: map[string]string{
					"key": "value",
				},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							MetricsPath:     "/metrics",
							Scheme:          "https",
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: model.LabelValue("localhost:1234"),
											},
										},
										Labels: model.LabelSet{
											model.LabelName("key"): model.LabelValue("value"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with timeout unset",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(0),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithScrapeTimeout(10 * time.Second),
		},
		{
			name: "Test with timeout less than collection_interval",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(5 * time.Second),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithScrapeTimeout(5 * time.Second),
		},
		{
			name: "Test with timeout equal to collection_interval",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(10 * time.Second),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithScrapeTimeout(10 * time.Second),
		},
		{
			name: "Test with timeout greater than collection_interval",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(30 * time.Second),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want:     wantWithScrapeTimeout(10 * time.Second),
			wantWarn: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.WarnLevel)
			got, err := getPrometheusConfig(tt.config, zap.New(core))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if tt.wantWarn {
				require.Equal(t, 1, logs.Len())
				assert.Equal(t, tt.config.ClientConfig.Timeout, logs.All()[0].ContextMap()["timeout"])
				assert.Equal(t, tt.config.CollectionInterval, logs.All()[0].ContextMap()["collection_interval"])
			} else {
				assert.Equal(t, 0, logs.Len())
			}

			// The prometheus receiver re-marshals its config through YAML on
			// start; the scrape timeout must survive that round trip.
			wantTimeout := got.PrometheusConfig.ScrapeConfigs[0].ScrapeTimeout
			require.NoError(t, got.PrometheusConfig.Reload())
			require.Len(t, got.PrometheusConfig.ScrapeConfigs, 1)
			assert.Equal(t, wantTimeout, got.PrometheusConfig.ScrapeConfigs[0].ScrapeTimeout)
		})
	}
}
