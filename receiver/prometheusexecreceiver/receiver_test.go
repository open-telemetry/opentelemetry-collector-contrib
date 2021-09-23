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

package prometheusexecreceiver

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

// loadConfigAssertNoError loads the test config and asserts there are no errors, and returns the receiver wanted
func loadConfigAssertNoError(t *testing.T, receiverConfigID config.ComponentID) config.Receiver {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[factory.Type()] = factory

	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	return cfg.Receivers[receiverConfigID]
}

// TestExecKeyMissing loads config and asserts there is an error with that config
func TestExecKeyMissing(t *testing.T) {
	receiverConfig := loadConfigAssertNoError(t, config.NewID(typeStr))

	assertErrorWhenExecKeyMissing(t, receiverConfig)
}

// assertErrorWhenExecKeyMissing makes sure the config passed throws an error, since it's missing the exec key
func assertErrorWhenExecKeyMissing(t *testing.T, errorReceiverConfig config.Receiver) {
	_, err := newPromExecReceiver(componenttest.NewNopReceiverCreateSettings(), errorReceiverConfig.(*Config), nil)
	assert.Error(t, err, "newPromExecReceiver() didn't return an error")
}

// TestEndToEnd loads the test config and completes an 2e2 test where Prometheus metrics are scrapped twice from `test_prometheus_exporter.go`
func TestEndToEnd(t *testing.T) {
	receiverConfig := loadConfigAssertNoError(t, config.NewIDWithName(typeStr, "end_to_end_test/2"))

	// e2e test with port undefined by user
	endToEndScrapeTest(t, receiverConfig, "end-to-end port not defined")
}

// endToEndScrapeTest creates a receiver that invokes `go run test_prometheus_exporter.go` and waits until it has scraped the /metrics endpoint twice - the application will crash between each scrape
func endToEndScrapeTest(t *testing.T, receiverConfig config.Receiver, testName string) {
	sink := new(consumertest.MetricsSink)
	wrapper, err := newPromExecReceiver(componenttest.NewNopReceiverCreateSettings(), receiverConfig.(*Config), sink)
	assert.NoError(t, err, "newPromExecReceiver() returned an error")

	ctx := context.Background()
	err = wrapper.Start(ctx, componenttest.NewNopHost())
	assert.NoError(t, err, "Start() returned an error")
	defer func() { assert.NoError(t, wrapper.Shutdown(ctx)) }()

	var metrics []pdata.Metrics

	// Make sure two scrapes have been completed (this implies the process was started, scraped, restarted and finally scraped a second time)
	const waitFor = 20 * time.Second
	const tick = 100 * time.Millisecond
	require.Eventuallyf(t, func() bool {
		got := sink.AllMetrics()
		if len(got) < 2 {
			return false
		}
		metrics = got
		return true
	}, waitFor, tick, "Two scrapes not completed after %v (%v)", waitFor, testName)

	assertTwoUniqueValuesScraped(t, metrics)
}

// assertTwoUniqueValuesScraped iterates over the found metrics and returns true if it finds at least 2 unique metrics, meaning the endpoint
// was successfully scraped twice AND the subprocess being handled was stopped and restarted
func assertTwoUniqueValuesScraped(t *testing.T, metricsSlice []pdata.Metrics) {
	var value float64
	for i := range metricsSlice {
		ms := metricsSlice[i].ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
		var tempM pdata.Metric
		ok := false
		for j := 0; j < ms.Len(); j++ {
			if ms.At(j).Name() == "timestamp_now" {
				tempM = ms.At(j)
				ok = true
				break
			}
		}
		require.True(t, ok, "timestamp_now metric not found")
		assert.Equal(t, pdata.MetricDataTypeGauge, tempM.DataType())
		tempV := tempM.Gauge().DataPoints().At(0).DoubleVal()
		if i != 0 && tempV != value {
			return
		}
		if tempV != value {
			value = tempV
		}
	}

	assert.Fail(t, fmt.Sprintf("All %v scraped values were non-unique", len(metricsSlice)))
}

func TestConfigBuilderFunctions(t *testing.T) {
	configTests := []struct {
		name                 string
		customName           string
		cfg                  *Config
		wantReceiverConfig   *prometheusreceiver.Config
		wantSubprocessConfig *subprocessmanager.SubprocessConfig
		wantErr              bool
	}{
		{
			name: "no command",
			cfg: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
				ScrapeInterval:   60 * time.Second,
				Port:             9104,
				SubprocessConfig: subprocessmanager.SubprocessConfig{
					Command: "",
					Env:     []subprocessmanager.EnvConfig{},
				},
			},
			wantReceiverConfig: &prometheusreceiver.Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
				PrometheusConfig: &promconfig.Config{
					ScrapeConfigs: []*promconfig.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(60 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							Scheme:          "http",
							MetricsPath:     "/metrics",
							JobName:         "prometheus_exec",
							HonorLabels:     false,
							HonorTimestamps: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
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
			wantSubprocessConfig: &subprocessmanager.SubprocessConfig{
				Env: []subprocessmanager.EnvConfig{},
			},
			wantErr: true,
		},
		{
			name: "normal config",
			cfg: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "mysqld")),
				ScrapeInterval:   90 * time.Second,
				Port:             9104,
				SubprocessConfig: subprocessmanager.SubprocessConfig{
					Command: "mysqld_exporter",
					Env: []subprocessmanager.EnvConfig{
						{
							Name:  "DATA_SOURCE_NAME",
							Value: "password:username@(url:port)/dbname",
						},
					},
				},
			},
			wantReceiverConfig: &prometheusreceiver.Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "mysqld")),
				PrometheusConfig: &promconfig.Config{
					ScrapeConfigs: []*promconfig.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(90 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							Scheme:          "http",
							MetricsPath:     "/metrics",
							JobName:         "mysqld",
							HonorLabels:     false,
							HonorTimestamps: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
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
			wantSubprocessConfig: &subprocessmanager.SubprocessConfig{
				Command: "mysqld_exporter",
				Env: []subprocessmanager.EnvConfig{
					{
						Name:  "DATA_SOURCE_NAME",
						Value: "password:username@(url:port)/dbname",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "lots of defaults",
			cfg: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "postgres/test")),
				ScrapeInterval:   60 * time.Second,
				SubprocessConfig: subprocessmanager.SubprocessConfig{
					Command: "postgres_exporter",
					Env: []subprocessmanager.EnvConfig{
						{
							Name:  "DATA_SOURCE_NAME",
							Value: "password:username@(url:port)/dbname",
						},
					},
				},
			},
			wantReceiverConfig: &prometheusreceiver.Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "postgres/test")),
				PrometheusConfig: &promconfig.Config{
					ScrapeConfigs: []*promconfig.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(60 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							Scheme:          "http",
							MetricsPath:     "/metrics",
							JobName:         "postgres/test",
							HonorLabels:     false,
							HonorTimestamps: true,
							ServiceDiscoveryConfigs: discovery.Configs{
								&discovery.StaticConfig{
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
			wantSubprocessConfig: &subprocessmanager.SubprocessConfig{
				Command: "postgres_exporter",
				Env: []subprocessmanager.EnvConfig{
					{
						Name:  "DATA_SOURCE_NAME",
						Value: "password:username@(url:port)/dbname",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, test := range configTests {
		t.Run(test.name, func(t *testing.T) {
			got := getPromReceiverConfig(test.cfg)
			assert.Equal(t, test.wantReceiverConfig, got)
		})
	}

	for _, test := range configTests {
		t.Run(test.name, func(t *testing.T) {
			got := getSubprocessConfig(test.cfg)
			assert.Equal(t, test.wantSubprocessConfig, got)
		})
	}
}

func TestFillPortPlaceholders(t *testing.T) {
	fillPortPlaceholdersTests := []struct {
		name    string
		wrapper *prometheusExecReceiver
		newPort int
		want    *subprocessmanager.SubprocessConfig
	}{
		{
			name: "port is defined by user",
			wrapper: &prometheusExecReceiver{
				port: 10500,
				config: &Config{
					SubprocessConfig: subprocessmanager.SubprocessConfig{
						Command: "apache_exporter --port:{{port}}",
						Env: []subprocessmanager.EnvConfig{
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
				subprocessConfig: &subprocessmanager.SubprocessConfig{
					Env: []subprocessmanager.EnvConfig{
						{
							Name: "DATA_SOURCE_NAME",
						},
						{
							Name: "SECONDARY_PORT",
						},
					},
				},
			},
			newPort: 10500,
			want: &subprocessmanager.SubprocessConfig{
				Command: "apache_exporter --port:10500",
				Env: []subprocessmanager.EnvConfig{
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
			wrapper: &prometheusExecReceiver{
				config: &Config{
					SubprocessConfig: subprocessmanager.SubprocessConfig{
						Command: "apache_exporter",
						Env: []subprocessmanager.EnvConfig{
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
				subprocessConfig: &subprocessmanager.SubprocessConfig{
					Env: []subprocessmanager.EnvConfig{
						{
							Name: "DATA_SOURCE_NAME",
						},
						{
							Name: "SECONDARY_PORT",
						},
					},
				},
			},
			newPort: 0,
			want: &subprocessmanager.SubprocessConfig{
				Command: "apache_exporter",
				Env: []subprocessmanager.EnvConfig{
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
			wrapper: &prometheusExecReceiver{
				config: &Config{
					SubprocessConfig: subprocessmanager.SubprocessConfig{
						Command: "apache_exporter --port={{port}}",
						Env: []subprocessmanager.EnvConfig{
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
				subprocessConfig: &subprocessmanager.SubprocessConfig{
					Env: []subprocessmanager.EnvConfig{
						{
							Name: "DATA_SOURCE_NAME",
						},
						{
							Name: "SECONDARY_PORT",
						},
					},
				},
			},
			newPort: 10111,
			want: &subprocessmanager.SubprocessConfig{
				Command: "apache_exporter --port=10111",
				Env: []subprocessmanager.EnvConfig{
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

	for _, test := range fillPortPlaceholdersTests {
		t.Run(test.name, func(t *testing.T) {
			got := test.wrapper.fillPortPlaceholders(test.newPort)
			assert.Equal(t, test.want.Command, got.Command)
			assert.Equal(t, test.want.Env, got.Env)
		})
	}
}

func TestGetDelayAndComputeCrashCount(t *testing.T) {
	var (
		getDelayAndComputeCrashCountTests = []struct {
			name               string
			elapsed            time.Duration
			healthyProcessTime time.Duration
			crashCount         int
			healthyCrashCount  int
			wantDelay          time.Duration
			wantCrashCount     int
		}{
			{
				name:               "healthy process 1",
				elapsed:            15 * time.Minute,
				healthyProcessTime: 30 * time.Minute,
				crashCount:         2,
				healthyCrashCount:  3,
				wantDelay:          1 * time.Second,
				wantCrashCount:     3,
			},
			{
				name:               "healthy process 2",
				elapsed:            15 * time.Hour,
				healthyProcessTime: 20 * time.Minute,
				crashCount:         6,
				healthyCrashCount:  2,
				wantDelay:          1 * time.Second,
				wantCrashCount:     1,
			},
			{
				name:               "unhealthy process 1",
				elapsed:            15 * time.Second,
				healthyProcessTime: 45 * time.Minute,
				crashCount:         4,
				healthyCrashCount:  3,
				wantCrashCount:     5,
			},
			{
				name:               "unhealthy process 2",
				elapsed:            15 * time.Second,
				healthyProcessTime: 75 * time.Second,
				crashCount:         5,
				healthyCrashCount:  3,
				wantCrashCount:     6,
			},
			{
				name:               "unhealthy process 3",
				elapsed:            15 * time.Second,
				healthyProcessTime: 30 * time.Minute,
				crashCount:         6,
				healthyCrashCount:  3,
				wantCrashCount:     7,
			},
			{
				name:               "unhealthy process 4",
				elapsed:            15 * time.Second,
				healthyProcessTime: 10 * time.Minute,
				crashCount:         7,
				healthyCrashCount:  3,
				wantCrashCount:     8,
			},
		}
		previousResult time.Duration
	)

	// getDelay() test
	t.Run("GetDelay test", func(t *testing.T) {
		for _, test := range getDelayAndComputeCrashCountTests {
			got := getDelay(test.elapsed, test.healthyProcessTime, test.crashCount, test.healthyCrashCount)

			if test.wantDelay > 0 {
				assert.Equalf(t, test.wantDelay, got, "getDelay() '%v', got = %v, want %v", test.name, got, test.wantDelay)
				continue
			}

			if previousResult > got {
				t.Errorf("getDelay() '%v', got = %v, want something larger than the previous result %v", test.name, got, previousResult)
			}
			previousResult = got
		}
	})

	// computeCrashCount() test
	per := &prometheusExecReceiver{}

	for _, test := range getDelayAndComputeCrashCountTests {
		t.Run(test.name, func(t *testing.T) {
			got := per.computeCrashCount(test.elapsed, test.crashCount)
			assert.Equal(t, test.wantCrashCount, got)
		})
	}
}
