// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package components

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"
	ddconf "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	dtconf "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/humioexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/parquetexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/skywalkingexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestDefaultExporters(t *testing.T) {
	factories, err := Components()
	assert.NoError(t, err)

	expFactories := factories.Exporters
	endpoint := testutil.GetAvailableLocalAddress(t)

	tests := []struct {
		exporter      config.Type
		getConfigFn   getExporterConfigFn
		skipLifecycle bool
	}{
		{
			exporter: "file",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["file"].CreateDefaultConfig().(*fileexporter.Config)
				cfg.Path = filepath.Join(t.TempDir(), "random.file")
				return cfg
			},
		},
		{
			exporter: "jaeger",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["jaeger"].CreateDefaultConfig().(*jaegerexporter.Config)
				cfg.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "jaeger_thrift",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["jaeger_thrift"].CreateDefaultConfig().(*jaegerthrifthttpexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "kafka",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["kafka"].CreateDefaultConfig().(*kafkaexporter.Config)
				cfg.Brokers = []string{"invalid:9092"}
				// this disables contacting the broker so we can successfully create the exporter
				cfg.Metadata.Full = false
				return cfg
			},
		},
		{
			exporter:      "logging",
			skipLifecycle: runtime.GOOS == "darwin", // TODO: investigate why this fails on darwin.
		},
		{
			exporter: "opencensus",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["opencensus"].CreateDefaultConfig().(*opencensusexporter.Config)
				cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
					Endpoint: endpoint,
				}
				return cfg
			},
		},
		{
			exporter: "otlp",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["otlp"].CreateDefaultConfig().(*otlpexporter.Config)
				cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
					Endpoint: endpoint,
				}
				return cfg
			},
		},
		{
			exporter: "otlphttp",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["otlphttp"].CreateDefaultConfig().(*otlphttpexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "parquet",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["parquet"].CreateDefaultConfig().(*parquetexporter.Config)
				cfg.Path = t.TempDir()
				return cfg
			},
		},
		{
			exporter: "prometheus",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["prometheus"].CreateDefaultConfig().(*prometheusexporter.Config)
				cfg.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "prometheusremotewrite",
		},
		{
			exporter: "sapm",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["sapm"].CreateDefaultConfig().(*sapmexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "signalfx",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["signalfx"].CreateDefaultConfig().(*signalfxexporter.Config)
				cfg.AccessToken = "my_fake_token"
				cfg.IngestURL = "http://" + endpoint
				cfg.APIURL = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "splunk_hec",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["splunk_hec"].CreateDefaultConfig().(*splunkhecexporter.Config)
				cfg.Token = "my_fake_token"
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "zipkin",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["zipkin"].CreateDefaultConfig().(*zipkinexporter.Config)
				cfg.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "awskinesis",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["awskinesis"].CreateDefaultConfig().(*awskinesisexporter.Config)
				cfg.AWS.KinesisEndpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "awsprometheusremotewrite",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["awsprometheusremotewrite"].CreateDefaultConfig().(*awsprometheusremotewriteexporter.Config)
				cfg.HTTPClientSettings.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "alibabacloud_logservice",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["alibabacloud_logservice"].CreateDefaultConfig().(*alibabacloudlogserviceexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Project = "otel-testing"
				cfg.Logstore = "otel-data"
				return cfg
			},
		},
		{
			exporter: "awscloudwatch",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["awscloudwatch"].CreateDefaultConfig().(*awscloudwatchlogsexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
		},
		{
			exporter: "awsemf",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["awsemf"].CreateDefaultConfig().(*awsemfexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
		},
		{
			exporter: "awsxray",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["awsxray"].CreateDefaultConfig().(*awsxrayexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
		},
		{
			exporter: "azuremonitor",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["azuremonitor"].CreateDefaultConfig().(*azuremonitorexporter.Config)
				cfg.Endpoint = "http://" + endpoint

				return cfg
			},
		},
		{
			exporter: "carbon",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["carbon"].CreateDefaultConfig().(*carbonexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "clickhouse",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["clickhouse"].CreateDefaultConfig().(*clickhouseexporter.Config)
				cfg.DSN = "clickhouse://" + endpoint
				return cfg
			},
		},
		{
			exporter: "coralogix",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["coralogix"].CreateDefaultConfig().(*coralogixexporter.Config)
				cfg.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "datadog",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["datadog"].CreateDefaultConfig().(*ddconf.Config)
				cfg.API.Key = "cutedogsgotoheaven"
				return cfg
			},
		},
		{
			exporter: "dynatrace",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["dynatrace"].CreateDefaultConfig().(*dtconf.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.APIToken = "dynamictracing"
				return cfg
			},
		},
		{
			exporter: "elastic",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["elastic"].CreateDefaultConfig().(*elasticexporter.Config)
				cfg.APMServerURL = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "elasticsearch",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["elasticsearch"].CreateDefaultConfig().(*elasticsearchexporter.Config)
				cfg.Endpoints = []string{"http://" + endpoint}
				return cfg
			},
		},
		{
			exporter: "f5cloud",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["f5cloud"].CreateDefaultConfig().(*f5cloudexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Source = "magic-source"
				cfg.AuthConfig.CredentialFile = filepath.Join(t.TempDir(), "random.file")

				return cfg
			},
		},
		{
			exporter:      "googlecloud",
			skipLifecycle: true, // Requires credentials to be able to successfully load the exporter
		},
		{
			exporter:      "googlemanagedprometheus",
			skipLifecycle: true, // Requires credentials to be able to successfully load the exporter
		},
		{
			exporter: "googlecloudpubsub",
		},
		{
			exporter: "honeycomb",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["honeycomb"].CreateDefaultConfig().(*honeycombexporter.Config)
				cfg.APIURL = "http://" + endpoint
				cfg.APIKey = "busybeesworking"
				return cfg
			},
		},
		{
			exporter: "humio",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["humio"].CreateDefaultConfig().(*humioexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "influxdb",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["influxdb"].CreateDefaultConfig().(*influxdbexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "loadbalancing",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["loadbalancing"].CreateDefaultConfig().(*loadbalancingexporter.Config)
				return cfg
			},
		},
		{
			exporter: "logzio",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["logzio"].CreateDefaultConfig().(*logzioexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "loki",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["loki"].CreateDefaultConfig().(*lokiexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "mezmo",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["mezmo"].CreateDefaultConfig().(*mezmoexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "sentry",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["sentry"].CreateDefaultConfig().(*sentryexporter.Config)
				return cfg
			},
		},
		{
			exporter: "skywalking",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["skywalking"].CreateDefaultConfig().(*skywalkingexporter.Config)
				return cfg
			},
		},
		{
			exporter: "sumologic",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["sumologic"].CreateDefaultConfig().(*sumologicexporter.Config)
				cfg.Endpoint = "http://" + endpoint

				return cfg
			},
		},
		{
			exporter: "tanzuobservability",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["tanzuobservability"].CreateDefaultConfig().(*tanzuobservabilityexporter.Config)
				cfg.Traces.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "tencentcloud_logservice",
			getConfigFn: func() config.Exporter {
				cfg := expFactories["tencentcloud_logservice"].CreateDefaultConfig().(*tencentcloudlogserviceexporter.Config)

				return cfg
			},
		},
	}

	assert.Len(t, tests, len(expFactories), "All user configurable components must be added to the lifecycle test")
	for _, tt := range tests {
		t.Run(string(tt.exporter), func(t *testing.T) {
			t.Parallel()

			factory, ok := expFactories[tt.exporter]
			require.True(t, ok)
			assert.Equal(t, tt.exporter, factory.Type())
			assert.Equal(t, config.NewComponentID(tt.exporter), factory.CreateDefaultConfig().ID())

			if tt.skipLifecycle {
				t.Skip("Skipping lifecycle test", tt.exporter)
				return
			}

			verifyExporterLifecycle(t, factory, tt.getConfigFn)
		})
	}
}

// GetExporterConfigFn is used customize the configuration passed to the verification.
// This is used to change ports or provide values required but not provided by the
// default configuration.
type getExporterConfigFn func() config.Exporter

// verifyExporterLifecycle is used to test if an exporter type can handle the typical
// lifecycle of a component. The getConfigFn parameter only need to be specified if
// the test can't be done with the default configuration for the component.
func verifyExporterLifecycle(t *testing.T, factory component.ExporterFactory, getConfigFn getExporterConfigFn) {
	ctx := context.Background()
	host := newAssertNoErrorHost(t)
	expCreateSettings := componenttest.NewNopExporterCreateSettings()

	cfg := factory.CreateDefaultConfig()
	if getConfigFn != nil {
		cfg = getConfigFn()
	}

	createFns := []createExporterFn{
		wrapCreateLogsExp(factory),
		wrapCreateTracesExp(factory),
		wrapCreateMetricsExp(factory),
	}

	for i := 0; i < 2; i++ {
		var exps []component.Exporter
		for _, createFn := range createFns {
			exp, err := createFn(ctx, expCreateSettings, cfg)
			if errors.Is(err, component.ErrDataTypeIsNotSupported) {
				continue
			}
			require.NoError(t, err)
			require.NoError(t, exp.Start(ctx, host))
			exps = append(exps, exp)
		}
		for _, exp := range exps {
			assert.NoError(t, exp.Shutdown(ctx))
		}
	}
}

type createExporterFn func(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.Exporter, error)

func wrapCreateLogsExp(factory component.ExporterFactory) createExporterFn {
	return func(ctx context.Context, set component.ExporterCreateSettings, cfg config.Exporter) (component.Exporter, error) {
		return factory.CreateLogsExporter(ctx, set, cfg)
	}
}

func wrapCreateTracesExp(factory component.ExporterFactory) createExporterFn {
	return func(ctx context.Context, set component.ExporterCreateSettings, cfg config.Exporter) (component.Exporter, error) {
		return factory.CreateTracesExporter(ctx, set, cfg)
	}
}

func wrapCreateMetricsExp(factory component.ExporterFactory) createExporterFn {
	return func(ctx context.Context, set component.ExporterCreateSettings, cfg config.Exporter) (component.Exporter, error) {
		return factory.CreateMetricsExporter(ctx, set, cfg)
	}
}
