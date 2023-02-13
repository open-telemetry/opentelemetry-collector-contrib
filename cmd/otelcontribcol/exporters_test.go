// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package main

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"
	dtconf "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/parquetexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
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
	factories, err := components()
	assert.NoError(t, err)

	expFactories := factories.Exporters
	endpoint := testutil.GetAvailableLocalAddress(t)

	tests := []struct {
		getConfigFn   getExporterConfigFn
		exporter      component.Type
		skipLifecycle bool
	}{
		{
			exporter: "awscloudwatchlogs",
			getConfigFn: func() component.Config {
				return expFactories["awscloudwatchlogs"].CreateDefaultConfig()
			},
			skipLifecycle: true,
		},
		{
			exporter: "awss3",
		},
		{
			exporter: "file",
			getConfigFn: func() component.Config {
				cfg := expFactories["file"].CreateDefaultConfig().(*fileexporter.Config)
				cfg.Path = filepath.Join(t.TempDir(), "random.file")
				return cfg
			},
		},
		{
			exporter: "jaeger",
			getConfigFn: func() component.Config {
				cfg := expFactories["jaeger"].CreateDefaultConfig().(*jaegerexporter.Config)
				cfg.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "jaeger_thrift",
			getConfigFn: func() component.Config {
				cfg := expFactories["jaeger_thrift"].CreateDefaultConfig().(*jaegerthrifthttpexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "kafka",
			getConfigFn: func() component.Config {
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
			getConfigFn: func() component.Config {
				cfg := expFactories["opencensus"].CreateDefaultConfig().(*opencensusexporter.Config)
				cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
					Endpoint: endpoint,
				}
				return cfg
			},
		},
		{
			exporter: "opensearch",
			getConfigFn: func() component.Config {
				cfg := expFactories["opensearch"].CreateDefaultConfig().(*opensearchexporter.Config)
				cfg.Endpoints = []string{"http://" + endpoint}
				return cfg
			},
		},
		{
			exporter: "otlp",
			getConfigFn: func() component.Config {
				cfg := expFactories["otlp"].CreateDefaultConfig().(*otlpexporter.Config)
				cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
					Endpoint: endpoint,
				}
				return cfg
			},
		},
		{
			exporter: "otlphttp",
			getConfigFn: func() component.Config {
				cfg := expFactories["otlphttp"].CreateDefaultConfig().(*otlphttpexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "parquet",
			getConfigFn: func() component.Config {
				cfg := expFactories["parquet"].CreateDefaultConfig().(*parquetexporter.Config)
				cfg.Path = t.TempDir()
				return cfg
			},
		},
		{
			exporter: "prometheus",
			getConfigFn: func() component.Config {
				cfg := expFactories["prometheus"].CreateDefaultConfig().(*prometheusexporter.Config)
				cfg.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "prometheusremotewrite",
		},
		{
			exporter: "pulsar",
			getConfigFn: func() component.Config {
				cfg := expFactories["pulsar"].CreateDefaultConfig().(*pulsarexporter.Config)
				cfg.Endpoint = "http://localhost:6650"
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "sapm",
			getConfigFn: func() component.Config {
				cfg := expFactories["sapm"].CreateDefaultConfig().(*sapmexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "signalfx",
			getConfigFn: func() component.Config {
				cfg := expFactories["signalfx"].CreateDefaultConfig().(*signalfxexporter.Config)
				cfg.AccessToken = "my_fake_token"
				cfg.IngestURL = "http://" + endpoint
				cfg.APIURL = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "splunk_hec",
			getConfigFn: func() component.Config {
				cfg := expFactories["splunk_hec"].CreateDefaultConfig().(*splunkhecexporter.Config)
				cfg.Token = "my_fake_token"
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "zipkin",
			getConfigFn: func() component.Config {
				cfg := expFactories["zipkin"].CreateDefaultConfig().(*zipkinexporter.Config)
				cfg.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "awskinesis",
			getConfigFn: func() component.Config {
				cfg := expFactories["awskinesis"].CreateDefaultConfig().(*awskinesisexporter.Config)
				cfg.AWS.KinesisEndpoint = "http://" + endpoint
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "alibabacloud_logservice",
			getConfigFn: func() component.Config {
				cfg := expFactories["alibabacloud_logservice"].CreateDefaultConfig().(*alibabacloudlogserviceexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Project = "otel-testing"
				cfg.Logstore = "otel-data"
				return cfg
			},
		},
		{
			exporter: "awscloudwatch",
			getConfigFn: func() component.Config {
				cfg := expFactories["awscloudwatch"].CreateDefaultConfig().(*awscloudwatchlogsexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
		},
		{
			exporter: "awsemf",
			getConfigFn: func() component.Config {
				cfg := expFactories["awsemf"].CreateDefaultConfig().(*awsemfexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
		},
		{
			exporter: "awsxray",
			getConfigFn: func() component.Config {
				cfg := expFactories["awsxray"].CreateDefaultConfig().(*awsxrayexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
		},
		{
			exporter: "azuredataexplorer",
			getConfigFn: func() component.Config {
				cfg := expFactories["azuredataexplorer"].CreateDefaultConfig().(*azuredataexplorerexporter.Config)
				cfg.ClusterURI = "https://" + endpoint
				cfg.ApplicationID = "otel-app-id"
				cfg.ApplicationKey = "otel-app-key"
				cfg.TenantID = "otel-tenant-id"
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "azuremonitor",
			getConfigFn: func() component.Config {
				cfg := expFactories["azuremonitor"].CreateDefaultConfig().(*azuremonitorexporter.Config)
				cfg.Endpoint = "http://" + endpoint

				return cfg
			},
		},
		{
			exporter: "carbon",
			getConfigFn: func() component.Config {
				cfg := expFactories["carbon"].CreateDefaultConfig().(*carbonexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "clickhouse",
			getConfigFn: func() component.Config {
				cfg := expFactories["clickhouse"].CreateDefaultConfig().(*clickhouseexporter.Config)
				cfg.Endpoint = "tcp://" + endpoint
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "cassandra",
			getConfigFn: func() component.Config {
				cfg := expFactories["cassandra"].CreateDefaultConfig().(*cassandraexporter.Config)
				cfg.DSN = endpoint
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "coralogix",
			getConfigFn: func() component.Config {
				cfg := expFactories["coralogix"].CreateDefaultConfig().(*coralogixexporter.Config)
				cfg.Traces.Endpoint = endpoint
				cfg.Logs.Endpoint = endpoint
				cfg.Metrics.Endpoint = endpoint
				return cfg
			},
		},
		{
			exporter: "datadog",
			getConfigFn: func() component.Config {
				cfg := expFactories["datadog"].CreateDefaultConfig().(*datadogexporter.Config)
				cfg.API.Key = "cutedogsgotoheaven"
				return cfg
			},
		},
		{
			exporter: "dataset",
			getConfigFn: func() component.Config {
				cfg := expFactories["dataset"].CreateDefaultConfig().(*datasetexporter.Config)
				cfg.DatasetURL = "https://" + endpoint
				cfg.APIKey = "secret-key"
				return cfg
			},
		},
		{
			exporter: "dynatrace",
			getConfigFn: func() component.Config {
				cfg := expFactories["dynatrace"].CreateDefaultConfig().(*dtconf.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.APIToken = "dynamictracing"
				return cfg
			},
		},
		{
			exporter: "elasticsearch",
			getConfigFn: func() component.Config {
				cfg := expFactories["elasticsearch"].CreateDefaultConfig().(*elasticsearchexporter.Config)
				cfg.Endpoints = []string{"http://" + endpoint}
				return cfg
			},
		},
		{
			exporter: "f5cloud",
			getConfigFn: func() component.Config {
				cfg := expFactories["f5cloud"].CreateDefaultConfig().(*f5cloudexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Source = "magic-source"
				cfg.AuthConfig.CredentialFile = filepath.Join(t.TempDir(), "random.file")

				return cfg
			},
			skipLifecycle: true,
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
			exporter:      "googlecloudpubsub",
			skipLifecycle: true,
		},
		{
			exporter: "influxdb",
			getConfigFn: func() component.Config {
				cfg := expFactories["influxdb"].CreateDefaultConfig().(*influxdbexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "instana",
			getConfigFn: func() component.Config {
				cfg := expFactories["instana"].CreateDefaultConfig().(*instanaexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.AgentKey = "Key1"
				return cfg
			},
		},
		{
			exporter: "loadbalancing",
			getConfigFn: func() component.Config {
				cfg := expFactories["loadbalancing"].CreateDefaultConfig().(*loadbalancingexporter.Config)
				cfg.Resolver = loadbalancingexporter.ResolverSettings{Static: &loadbalancingexporter.StaticResolver{Hostnames: []string{"127.0.0.1"}}}
				return cfg
			},
		},
		{
			exporter: "logicmonitor",
			getConfigFn: func() component.Config {
				cfg := expFactories["logicmonitor"].CreateDefaultConfig().(*logicmonitorexporter.Config)
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "logzio",
			getConfigFn: func() component.Config {
				cfg := expFactories["logzio"].CreateDefaultConfig().(*logzioexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "loki",
			getConfigFn: func() component.Config {
				cfg := expFactories["loki"].CreateDefaultConfig().(*lokiexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "mezmo",
			getConfigFn: func() component.Config {
				cfg := expFactories["mezmo"].CreateDefaultConfig().(*mezmoexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "sentry",
			getConfigFn: func() component.Config {
				cfg := expFactories["sentry"].CreateDefaultConfig().(*sentryexporter.Config)
				return cfg
			},
		},
		{
			exporter: "skywalking",
			getConfigFn: func() component.Config {
				cfg := expFactories["skywalking"].CreateDefaultConfig().(*skywalkingexporter.Config)
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "sumologic",
			getConfigFn: func() component.Config {
				cfg := expFactories["sumologic"].CreateDefaultConfig().(*sumologicexporter.Config)
				cfg.Endpoint = "http://" + endpoint

				return cfg
			},
		},
		{
			exporter: "tanzuobservability",
			getConfigFn: func() component.Config {
				cfg := expFactories["tanzuobservability"].CreateDefaultConfig().(*tanzuobservabilityexporter.Config)
				cfg.Traces.Endpoint = "http://" + endpoint
				cfg.Metrics.Endpoint = "http://" + endpoint
				return cfg
			},
		},
		{
			exporter: "tencentcloud_logservice",
			getConfigFn: func() component.Config {
				cfg := expFactories["tencentcloud_logservice"].CreateDefaultConfig().(*tencentcloudlogserviceexporter.Config)

				return cfg
			},
		},
	}

	exporterCount := 0
	expectedExporters := map[component.Type]struct{}{}
	for k := range expFactories {
		expectedExporters[k] = struct{}{}
	}
	for _, tt := range tests {
		_, ok := expFactories[tt.exporter]
		if !ok {
			// not part of the distro, skipping.
			continue
		}
		exporterCount++
		delete(expectedExporters, tt.exporter)
		t.Run(string(tt.exporter), func(t *testing.T) {
			factory := expFactories[tt.exporter]
			assert.Equal(t, tt.exporter, factory.Type())
			verifyExporterShutdown(t, factory, tt.getConfigFn)
			if !tt.skipLifecycle {
				verifyExporterLifecycle(t, factory, tt.getConfigFn)
			}
		})
	}
	assert.Len(t, expFactories, exporterCount, "All user configurable components must be added to the lifecycle test", expectedExporters)
}

// GetExporterConfigFn is used customize the configuration passed to the verification.
// This is used to change ports or provide values required but not provided by the
// default configuration.
type getExporterConfigFn func() component.Config

// verifyExporterLifecycle is used to test if an exporter type can handle the typical
// lifecycle of a component. The getConfigFn parameter only need to be specified if
// the test can't be done with the default configuration for the component.
func verifyExporterLifecycle(t *testing.T, factory exporter.Factory, getConfigFn getExporterConfigFn) {
	ctx := context.Background()
	host := newAssertNoErrorHost(t)
	expCreateSettings := exportertest.NewNopCreateSettings()

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
		var exps []component.Component
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

// verifyExporterShutdown is used to test if an exporter type can be shutdown without being started first.
func verifyExporterShutdown(tb testing.TB, factory exporter.Factory, getConfigFn getExporterConfigFn) {
	ctx := context.Background()
	expCreateSettings := exportertest.NewNopCreateSettings()

	if getConfigFn == nil {
		getConfigFn = factory.CreateDefaultConfig
	}

	createFns := []createExporterFn{
		wrapCreateLogsExp(factory),
		wrapCreateTracesExp(factory),
		wrapCreateMetricsExp(factory),
	}

	for _, createFn := range createFns {
		r, err := createFn(ctx, expCreateSettings, getConfigFn())
		if errors.Is(err, component.ErrDataTypeIsNotSupported) {
			continue
		}
		if r == nil {
			continue
		}
		assert.NotPanics(tb, func() {
			assert.NoError(tb, r.Shutdown(ctx))
		})
	}
}

type createExporterFn func(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (component.Component, error)

func wrapCreateLogsExp(factory exporter.Factory) createExporterFn {
	return func(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (component.Component, error) {
		return factory.CreateLogsExporter(ctx, set, cfg)
	}
}

func wrapCreateTracesExp(factory exporter.Factory) createExporterFn {
	return func(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (component.Component, error) {
		return factory.CreateTracesExporter(ctx, set, cfg)
	}
}

func wrapCreateMetricsExp(factory exporter.Factory) createExporterFn {
	return func(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (component.Component, error) {
		return factory.CreateMetricsExporter(ctx, set, cfg)
	}
}
