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
package prometheusexec

import (
	"reflect"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexec/subprocessmanager"
	subconfig "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexec/subprocessmanager/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	sdconfig "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/receiver/prometheusreceiver"
)

var (
	configTests = []struct {
		name                 string
		customName           string
		config               *Config
		wantReceiverConfig   *prometheusreceiver.Config
		wantSubprocessConfig *subprocessmanager.Process
		wantErr              bool
	}{
		{
			name:       "no command",
			customName: "prometheus_exec",
			config: &Config{
				ScrapeInterval: 60 * time.Second,
				SubprocessConfig: subconfig.SubprocessConfig{
					Command:    "",
					Port:       9104,
					CustomName: "",
					Env:        []subconfig.EnvConfig{},
				},
			},
			wantReceiverConfig: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						&config.ScrapeConfig{
							ScrapeInterval:  model.Duration(60 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							Scheme:          "http",
							MetricsPath:     "/metrics",
							JobName:         "prometheus_exec",
							HonorLabels:     false,
							HonorTimestamps: true,
							ServiceDiscoveryConfig: sdconfig.ServiceDiscoveryConfig{
								StaticConfigs: []*targetgroup.Group{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:9104")},
										},
									},
								},
							},
						},
					},
				},
			},
			wantSubprocessConfig: nil,
			wantErr:              true,
		},
		{
			name:       "normal config",
			customName: "mysqld",
			config: &Config{
				ScrapeInterval: 90 * time.Second,
				SubprocessConfig: subconfig.SubprocessConfig{
					Command:    "mysqld_exporter",
					Port:       9104,
					CustomName: "mysqld",
					Env: []subconfig.EnvConfig{
						subconfig.EnvConfig{
							Name:  "DATA_SOURCE_NAME",
							Value: "password:username@(url:port)/dbname",
						},
					},
				},
			},
			wantReceiverConfig: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						&config.ScrapeConfig{
							ScrapeInterval:  model.Duration(90 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							Scheme:          "http",
							MetricsPath:     "/metrics",
							JobName:         "mysqld",
							HonorLabels:     false,
							HonorTimestamps: true,
							ServiceDiscoveryConfig: sdconfig.ServiceDiscoveryConfig{
								StaticConfigs: []*targetgroup.Group{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:9104")},
										},
									},
								},
							},
						},
					},
				},
			},
			wantSubprocessConfig: &subprocessmanager.Process{
				Command: "mysqld_exporter",
				Port:    9104,
				Env: []subconfig.EnvConfig{
					subconfig.EnvConfig{
						Name:  "DATA_SOURCE_NAME",
						Value: "password:username@(url:port)/dbname",
					},
				},
				CustomName: "mysqld",
			},
			wantErr: false,
		},
		{
			name:       "lots of defaults",
			customName: "postgres",
			config: &Config{
				ScrapeInterval: 60 * time.Second,
				SubprocessConfig: subconfig.SubprocessConfig{
					Command:    "postgres_exporter",
					CustomName: "postgres",
					Env: []subconfig.EnvConfig{
						subconfig.EnvConfig{
							Name:  "DATA_SOURCE_NAME",
							Value: "password:username@(url:port)/dbname",
						},
					},
				},
			},
			wantReceiverConfig: &prometheusreceiver.Config{
				PrometheusConfig: &config.Config{
					ScrapeConfigs: []*config.ScrapeConfig{
						&config.ScrapeConfig{
							ScrapeInterval:  model.Duration(60 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							Scheme:          "http",
							MetricsPath:     "/metrics",
							JobName:         "postgres",
							HonorLabels:     false,
							HonorTimestamps: true,
							ServiceDiscoveryConfig: sdconfig.ServiceDiscoveryConfig{
								StaticConfigs: []*targetgroup.Group{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:0")},
										},
									},
								},
							},
						},
					},
				},
			},
			wantSubprocessConfig: &subprocessmanager.Process{
				Command: "postgres_exporter",
				Port:    0,
				Env: []subconfig.EnvConfig{
					subconfig.EnvConfig{
						Name:  "DATA_SOURCE_NAME",
						Value: "password:username@(url:port)/dbname",
					},
				},
				CustomName: "postgres",
			},
			wantErr: false,
		},
	}

	customNameTests = []struct {
		name   string
		config *Config
		want   string
	}{
		{
			name: "no custom name",
			config: &Config{
				ReceiverSettings: configmodels.ReceiverSettings{
					TypeVal: "prometheus_exec",
					NameVal: "prometheus_exec",
				},
				ScrapeInterval: 60 * time.Second,
				SubprocessConfig: subconfig.SubprocessConfig{
					Command:    "mysqld_exporter",
					Port:       9104,
					CustomName: "",
					Env:        []subconfig.EnvConfig{},
				},
			},
			want: "prometheus_exec",
		},
		{
			name: "no custom name, only trailing slash",
			config: &Config{
				ReceiverSettings: configmodels.ReceiverSettings{
					TypeVal: "prometheus_exec",
					NameVal: "prometheus_exec/",
				},
				ScrapeInterval: 60 * time.Second,
				SubprocessConfig: subconfig.SubprocessConfig{
					Command:    "mysqld_exporter",
					Port:       9104,
					CustomName: "",
					Env:        []subconfig.EnvConfig{},
				},
			},
			want: "prometheus_exec",
		},
		{
			name: "custom name",
			config: &Config{
				ReceiverSettings: configmodels.ReceiverSettings{
					TypeVal: "prometheus_exec",
					NameVal: "prometheus_exec/custom",
				},
				ScrapeInterval: 60 * time.Second,
				SubprocessConfig: subconfig.SubprocessConfig{
					Command:    "mysqld_exporter",
					Port:       9104,
					CustomName: "",
					Env:        []subconfig.EnvConfig{},
				},
			},
			want: "custom",
		},
		{
			name: "custom name with slashes inside",
			config: &Config{
				ReceiverSettings: configmodels.ReceiverSettings{
					TypeVal: "prometheus_exec",
					NameVal: "prometheus_exec/custom/name",
				},
				ScrapeInterval: 60 * time.Second,
				SubprocessConfig: subconfig.SubprocessConfig{
					Command:    "mysqld_exporter",
					Port:       9104,
					CustomName: "",
					Env:        []subconfig.EnvConfig{},
				},
			},
			want: "custom/name",
		},
	}

	generateRandomPortTests = []struct {
		name     string
		lastPort int
		wantMin  int
		wantMax  int
	}{
		{
			name:     "normal test 1",
			lastPort: 10001,
			wantMin:  10000,
			wantMax:  11000,
		},
		{
			name:     "normal test 2",
			lastPort: 0,
			wantMin:  10000,
			wantMax:  11000,
		},
		{
			name:     "normal test 3",
			lastPort: 10500,
			wantMin:  10000,
			wantMax:  11000,
		},
	}

	fillPortPlaceholdersTests = []struct {
		name    string
		wrapper *prometheusReceiverWrapper
		newPort int
		want    *subprocessmanager.Process
	}{
		{
			name: "port is defined by user",
			wrapper: &prometheusReceiverWrapper{
				config: &Config{
					SubprocessConfig: subconfig.SubprocessConfig{
						Command:    "apache_exporter --port:{{port}}",
						CustomName: "apache exporter with port {{port}}",
						Port:       10500,
						Env: []subconfig.EnvConfig{
							{
								Name:  "DATA_SOURCE_NAME",
								Value: "user:password@(hostname:{{port}})/dbname",
							},
							{
								Name:  "SECONDARY_PORT",
								Value: "{{port}}",
							},
						},
					},
				},
				subprocessConfig: &subprocessmanager.Process{
					Command:    "apache_exporter --port:{{port}}",
					CustomName: "apache exporter with port {{port}}",
					Port:       10500,
					Env: []subconfig.EnvConfig{
						{
							Name:  "DATA_SOURCE_NAME",
							Value: "user:password@(hostname:{{port}})/dbname",
						},
						{
							Name:  "SECONDARY_PORT",
							Value: "{{port}}",
						},
					},
				},
			},
			newPort: 0,
			want: &subprocessmanager.Process{
				Command:    "apache_exporter --port:10500",
				CustomName: "apache exporter with port 10500",
				Port:       10500,
				Env: []subconfig.EnvConfig{
					{
						Name:  "DATA_SOURCE_NAME",
						Value: "user:password@(hostname:10500)/dbname",
					},
					{
						Name:  "SECONDARY_PORT",
						Value: "10500",
					},
				},
			},
		},
		{
			name: "no string templating",
			wrapper: &prometheusReceiverWrapper{
				config: &Config{
					SubprocessConfig: subconfig.SubprocessConfig{
						Command:    "apache_exporter",
						CustomName: "apache exporter",
						Port:       10500,
						Env: []subconfig.EnvConfig{
							{
								Name:  "DATA_SOURCE_NAME",
								Value: "user:password@(hostname:port)/dbname",
							},
							{
								Name:  "SECONDARY_PORT",
								Value: "1234",
							},
						},
					},
				},
				subprocessConfig: &subprocessmanager.Process{
					Command:    "apache_exporter",
					CustomName: "apache exporter",
					Port:       10500,
					Env: []subconfig.EnvConfig{
						{
							Name:  "DATA_SOURCE_NAME",
							Value: "user:password@(hostname:port)/dbname",
						},
						{
							Name:  "SECONDARY_PORT",
							Value: "1234",
						},
					},
				},
			},
			newPort: 0,
			want: &subprocessmanager.Process{
				Command:    "apache_exporter",
				CustomName: "apache exporter",
				Port:       10500,
				Env: []subconfig.EnvConfig{
					{
						Name:  "DATA_SOURCE_NAME",
						Value: "user:password@(hostname:port)/dbname",
					},
					{
						Name:  "SECONDARY_PORT",
						Value: "1234",
					},
				},
			},
		},
		{
			name: "no port defined",
			wrapper: &prometheusReceiverWrapper{
				config: &Config{
					SubprocessConfig: subconfig.SubprocessConfig{
						Command:    "apache_exporter --port={{port}}",
						CustomName: "apache exporter with port:{{port}}",
						Port:       0,
						Env: []subconfig.EnvConfig{
							{
								Name:  "DATA_SOURCE_NAME",
								Value: "user:password@(hostname:{{port}})/dbname",
							},
							{
								Name:  "SECONDARY_PORT",
								Value: "{{port}}",
							},
						},
					},
				},
				subprocessConfig: &subprocessmanager.Process{
					Command:    "apache_exporter --port={{port}}",
					CustomName: "apache exporter with port:{{port}}",
					Port:       0,
					Env: []subconfig.EnvConfig{
						{
							Name:  "DATA_SOURCE_NAME",
							Value: "user:password@(hostname:{{port}})/dbname",
						},
						{
							Name:  "SECONDARY_PORT",
							Value: "{{port}}",
						},
					},
				},
			},
			newPort: 10111,
			want: &subprocessmanager.Process{
				Command:    "apache_exporter --port=10111",
				CustomName: "apache exporter with port:10111",
				Port:       10111,
				Env: []subconfig.EnvConfig{
					{
						Name:  "DATA_SOURCE_NAME",
						Value: "user:password@(hostname:10111)/dbname",
					},
					{
						Name:  "SECONDARY_PORT",
						Value: "10111",
					},
				},
			},
		},
	}

	handleNewPortTests = []struct {
		name    string
		wrapper *prometheusReceiverWrapper
		oldPort int
		wantErr bool
	}{
		{
			name: "port defined by user",
			wrapper: &prometheusReceiverWrapper{
				receiverConfig: &prometheusreceiver.Config{
					PrometheusConfig: &config.Config{
						ScrapeConfigs: []*config.ScrapeConfig{
							{
								MetricsPath:     "/metrics",
								Scheme:          "http",
								HonorLabels:     false,
								HonorTimestamps: true,
								ServiceDiscoveryConfig: sdconfig.ServiceDiscoveryConfig{
									StaticConfigs: []*targetgroup.Group{
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
			oldPort: 0,
		},
	}
)

func TestGetReceiverConfig(t *testing.T) {
	for _, test := range configTests {
		t.Run(test.name, func(t *testing.T) {
			got := getReceiverConfig(test.config, test.customName)
			if !reflect.DeepEqual(got, test.wantReceiverConfig) {
				t.Errorf("getReceiverConfig() got = %v, want %v", got, test.wantReceiverConfig)
			}
		})
	}
}

func TestGetSubprocessConfig(t *testing.T) {
	for _, test := range configTests {
		t.Run(test.name, func(t *testing.T) {
			got, err := getSubprocessConfig(test.config, test.customName)
			if (err != nil) != test.wantErr {
				t.Errorf("getSubprocessConfig() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !reflect.DeepEqual(got, test.wantSubprocessConfig) {
				t.Errorf("getSubprocessConfig() got = %v, want %v", got, test.wantSubprocessConfig)
			}
		})
	}
}

func TestGetCustomName(t *testing.T) {
	for _, test := range customNameTests {
		t.Run(test.name, func(t *testing.T) {
			got := getCustomName(test.config)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("getCustomName() got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGenerateRandomPort(t *testing.T) {
	for _, test := range generateRandomPortTests {
		t.Run(test.name, func(t *testing.T) {
			got := generateRandomPort(test.lastPort)
			if got < test.wantMin || got > test.wantMax {
				t.Errorf("generateRandomPort() got = %v, want smaller than %v and larger than %v", got, test.wantMax, test.wantMin)
			}
		})
	}
}

func TestFillPortPlaceholders(t *testing.T) {
	for _, test := range fillPortPlaceholdersTests {
		t.Run(test.name, func(t *testing.T) {
			got := test.wrapper.fillPortPlaceholders(test.newPort)
			if got.Command != test.want.Command || got.CustomName != test.want.CustomName || !reflect.DeepEqual(got.Env, test.want.Env) {
				t.Errorf("fillPortPlaceholders() got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHandleNewPort(t *testing.T) {
	for _, test := range handleNewPortTests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.wrapper.handleNewPort(test.oldPort)
			if err != nil {
				t.Errorf("handleNewPort() threw an error: %v", err)
			}
			if got == test.oldPort {
				t.Errorf("handleNewPort() got = %v, want different than %v", got, test.oldPort)
			}
		})
	}
}
