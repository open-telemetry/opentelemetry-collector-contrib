// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package simpleprometheusreceiver

import (
	"context"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
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
			cfg := (f.CreateDefaultConfig()).(*Config)
			cfg.UseServiceAccount = tt.useServiceAccount

			r, err := f.CreateMetricsReceiver(
				context.Background(),
				receivertest.NewNopCreateSettings(),
				cfg,
				consumertest.NewNop(),
			)

			if !tt.wantError {
				require.NoError(t, err)
				require.NotNil(t, r)

				require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(context.Background()))
				return
			}

			require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
		})
	}
}

func TestGetPrometheusConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   *prometheusreceiver.Config
	}{
		{
			name: "Test without TLS",
			config: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:1234",
					TLSSetting: configtls.TLSClientSetting{
						Insecure: true,
					},
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				Params:             url.Values{"foo": []string{"bar", "foobar"}},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
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
								&discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
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
			name: "Test with TLS",
			config: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:1234",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "./testdata/test_cert.pem",
						},
						InsecureSkipVerify: true,
					},
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							MetricsPath:     "/metrics",
							Scheme:          "https",
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
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
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:1234",
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
				Labels: map[string]string{
					"key": "value",
				},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							MetricsPath:     "/metrics",
							Scheme:          "https",
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{
												model.AddressLabel:     model.LabelValue("localhost:1234"),
												model.LabelName("key"): model.LabelValue("value")},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPrometheusConfig(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPrometheusConfigWrapper(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   *prometheusreceiver.Config
	}{
		{
			name: "Test TLSEnable true",
			config: &Config{
				httpConfig: httpConfig{
					TLSEnabled: true,
					TLSConfig: tlsConfig{
						CAFile:             "./testdata/test_cert.pem",
						InsecureSkipVerify: true,
					},
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
					TLSSetting: configtls.TLSClientSetting{
						Insecure: true,
					},
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				Params:             url.Values{"foo": []string{"bar", "foobar"}},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "prometheus_simple/localhost:9090",
							HonorTimestamps: true,
							Scheme:          "https",
							MetricsPath:     "/metric",
							Params:          url.Values{"foo": []string{"bar", "foobar"}},
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue(defaultEndpoint)},
										},
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
			name: "Test TLSEnable false",
			config: &Config{
				httpConfig: httpConfig{
					TLSEnabled: false,
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
					TLSSetting: configtls.TLSClientSetting{
						Insecure: true,
					},
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				Params:             url.Values{"foo": []string{"bar", "foobar"}},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "prometheus_simple/localhost:9090",
							HonorTimestamps: true,
							Scheme:          "http",
							MetricsPath:     "/metric",
							Params:          url.Values{"foo": []string{"bar", "foobar"}},
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue(defaultEndpoint)},
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
			name: "Test TLSEnable false but tls configured",
			config: &Config{
				httpConfig: httpConfig{
					TLSEnabled: false,
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
					TLSSetting: configtls.TLSClientSetting{
						Insecure: false,
					},
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				Params:             url.Values{"foo": []string{"bar", "foobar"}},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "prometheus_simple/localhost:9090",
							HonorTimestamps: true,
							Scheme:          "https",
							MetricsPath:     "/metric",
							Params:          url.Values{"foo": []string{"bar", "foobar"}},
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue(defaultEndpoint)},
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
			name: "Test TLSEnable false with tls configured with ca",
			config: &Config{
				httpConfig: httpConfig{
					TLSEnabled: false,
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
					TLSSetting: configtls.TLSClientSetting{
						Insecure: false,
						TLSSetting: configtls.TLSSetting{
							CAFile: "./testdata/test_cert.pem",
						},
					},
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				Params:             url.Values{"foo": []string{"bar", "foobar"}},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "prometheus_simple/localhost:9090",
							HonorTimestamps: true,
							Scheme:          "https",
							MetricsPath:     "/metric",
							Params:          url.Values{"foo": []string{"bar", "foobar"}},
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue(defaultEndpoint)},
										},
									},
								},
							},
							HTTPClientConfig: configutil.HTTPClientConfig{
								TLSConfig: configutil.TLSConfig{
									CAFile: "./testdata/test_cert.pem",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPrometheusConfigWrapper(tt.config, receivertest.NewNopCreateSettings())
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
