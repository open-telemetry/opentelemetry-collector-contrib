// Copyright 2020, OpenTelemetry Authors
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
	"reflect"
	"testing"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"

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
				componenttest.NewNopReceiverCreateSettings(),
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
		name    string
		config  *Config
		want    *prometheusreceiver.Config
		wantErr bool
	}{
		{
			name: "Test without TLS",
			config: &Config{
				TCPAddr: confignet.TCPAddr{
					Endpoint: "localhost:1234",
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
				TCPAddr: confignet.TCPAddr{
					Endpoint: "localhost:1234",
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
				httpConfig: httpConfig{
					TLSEnabled: true,
					TLSConfig: tlsConfig{
						CAFile:             "path1",
						CertFile:           "path2",
						KeyFile:            "path3",
						InsecureSkipVerify: true,
					},
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
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
									},
								},
							},
							HTTPClientConfig: configutil.HTTPClientConfig{
								TLSConfig: configutil.TLSConfig{
									CAFile:             "path1",
									CertFile:           "path2",
									KeyFile:            "path3",
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
				TCPAddr: confignet.TCPAddr{
					Endpoint: "localhost:1234",
				},
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
				httpConfig: httpConfig{
					TLSEnabled: true,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPrometheusConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPrometheusConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPrometheusConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
