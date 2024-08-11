// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"
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
		getConfigFn      getExporterConfigFn
		exporter         component.Type
		skipLifecycle    bool
		expectConsumeErr bool
	}{
		{
			exporter: "awscloudwatchlogs",
			getConfigFn: func() component.Config {
				cfg := expFactories["awscloudwatchlogs"].CreateDefaultConfig().(*awscloudwatchlogsexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"

				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter:         "awss3",
			expectConsumeErr: true,
		},
		{
			exporter: "file",
			getConfigFn: func() component.Config {
				cfg := expFactories["file"].CreateDefaultConfig().(*fileexporter.Config)
				cfg.Path = filepath.Join(t.TempDir(), "file.exporter.random.file")
				return cfg
			},
			skipLifecycle: runtime.GOOS == "windows", // On Windows not all handles are closed when the exporter is shutdown.
		},
		{
			exporter: "kafka",
			getConfigFn: func() component.Config {
				cfg := expFactories["kafka"].CreateDefaultConfig().(*kafkaexporter.Config)
				cfg.Brokers = []string{"invalid:9092"}
				// this disables contacting the broker so we can successfully create the exporter
				cfg.Metadata.Full = false
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "debug",
		},
		{
			exporter: "opencensus",
			getConfigFn: func() component.Config {
				cfg := expFactories["opencensus"].CreateDefaultConfig().(*opencensusexporter.Config)
				cfg.ClientConfig = configgrpc.ClientConfig{
					Endpoint: endpoint,
				}
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "opensearch",
			getConfigFn: func() component.Config {
				cfg := expFactories["opensearch"].CreateDefaultConfig().(*opensearchexporter.Config)
				cfg.ClientConfig = confighttp.ClientConfig{
					Endpoint: "http://" + endpoint,
				}
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "otlp",
			getConfigFn: func() component.Config {
				cfg := expFactories["otlp"].CreateDefaultConfig().(*otlpexporter.Config)
				cfg.ClientConfig = configgrpc.ClientConfig{
					Endpoint: endpoint,
				}
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueConfig.Enabled = false
				cfg.RetryConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "otlphttp",
			getConfigFn: func() component.Config {
				cfg := expFactories["otlphttp"].CreateDefaultConfig().(*otlphttpexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueConfig.Enabled = false
				cfg.RetryConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
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
			getConfigFn: func() component.Config {
				cfg := expFactories["prometheusremotewrite"].CreateDefaultConfig().(*prometheusremotewriteexporter.Config)
				// disable queue/retry to validate passing the test data synchronously
				cfg.RemoteWriteQueue.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
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
			exporter: "rabbitmq",
			getConfigFn: func() component.Config {
				cfg := expFactories["rabbitmq"].CreateDefaultConfig().(*rabbitmqexporter.Config)
				cfg.Connection.Endpoint = "amqp://localhost:5672"
				cfg.Connection.Auth.Plain.Username = "user"
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "sapm",
			getConfigFn: func() component.Config {
				cfg := expFactories["sapm"].CreateDefaultConfig().(*sapmexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "signalfx",
			getConfigFn: func() component.Config {
				cfg := expFactories["signalfx"].CreateDefaultConfig().(*signalfxexporter.Config)
				cfg.AccessToken = "my_fake_token"
				cfg.IngestURL = "http://" + endpoint
				cfg.APIURL = "http://" + endpoint
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "splunk_hec",
			getConfigFn: func() component.Config {
				cfg := expFactories["splunk_hec"].CreateDefaultConfig().(*splunkhecexporter.Config)
				cfg.Token = "my_fake_token"
				cfg.Endpoint = "http://" + endpoint
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "zipkin",
			getConfigFn: func() component.Config {
				cfg := expFactories["zipkin"].CreateDefaultConfig().(*zipkinexporter.Config)
				cfg.Endpoint = endpoint
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
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
			exporter: "alertmanager",
			getConfigFn: func() component.Config {
				cfg := expFactories["alertmanager"].CreateDefaultConfig().(*alertmanagerexporter.Config)
				cfg.ClientConfig = confighttp.ClientConfig{
					Endpoint: "http://" + endpoint,
				}
				cfg.GeneratorURL = "opentelemetry-collector"
				cfg.DefaultSeverity = "info"
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackoffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
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
			exporter: "awsemf",
			getConfigFn: func() component.Config {
				cfg := expFactories["awsemf"].CreateDefaultConfig().(*awsemfexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "awsxray",
			getConfigFn: func() component.Config {
				cfg := expFactories["awsxray"].CreateDefaultConfig().(*awsxrayexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				cfg.Region = "local"
				return cfg
			},
			expectConsumeErr: true,
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
				cfg.ConnectionString = configopaque.String("InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=" + cfg.Endpoint)

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
			expectConsumeErr: true,
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
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "datadog",
			getConfigFn: func() component.Config {
				cfg := expFactories["datadog"].CreateDefaultConfig().(*datadogexporter.Config)
				cfg.API.Key = "cutedogsgotoheaven"
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "dataset",
			getConfigFn: func() component.Config {
				cfg := expFactories["dataset"].CreateDefaultConfig().(*datasetexporter.Config)
				cfg.DatasetURL = "https://" + endpoint
				cfg.APIKey = "secret-key"
				// disable queue/retry to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
			skipLifecycle:    true, // shutdown fails if there is buffered data
		},
		{
			exporter: "elasticsearch",
			getConfigFn: func() component.Config {
				cfg := expFactories["elasticsearch"].CreateDefaultConfig().(*elasticsearchexporter.Config)
				cfg.Endpoints = []string{"http://" + endpoint}
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
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
			exporter:      "googlecloudpubsub",
			skipLifecycle: true,
		},
		{
			exporter: "honeycombmarker",
			getConfigFn: func() component.Config {
				cfg := expFactories["honeycombmarker"].CreateDefaultConfig().(*honeycombmarkerexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "influxdb",
			getConfigFn: func() component.Config {
				cfg := expFactories["influxdb"].CreateDefaultConfig().(*influxdbexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
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
			expectConsumeErr: true,
		},
		{
			exporter: "loadbalancing",
			getConfigFn: func() component.Config {
				cfg := expFactories["loadbalancing"].CreateDefaultConfig().(*loadbalancingexporter.Config)
				cfg.Resolver = loadbalancingexporter.ResolverSettings{Static: &loadbalancingexporter.StaticResolver{Hostnames: []string{"127.0.0.1"}}}
				return cfg
			},
			expectConsumeErr: true, // the exporter requires traces with service.name resource attribute
		},
		{
			exporter: "logicmonitor",
			getConfigFn: func() component.Config {
				cfg := expFactories["logicmonitor"].CreateDefaultConfig().(*logicmonitorexporter.Config)
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			skipLifecycle: true,
		},
		{
			exporter: "logzio",
			getConfigFn: func() component.Config {
				cfg := expFactories["logzio"].CreateDefaultConfig().(*logzioexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "loki",
			getConfigFn: func() component.Config {
				cfg := expFactories["loki"].CreateDefaultConfig().(*lokiexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "mezmo",
			getConfigFn: func() component.Config {
				cfg := expFactories["mezmo"].CreateDefaultConfig().(*mezmoexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
		},
		{
			exporter: "sentry",
			getConfigFn: func() component.Config {
				cfg := expFactories["sentry"].CreateDefaultConfig().(*sentryexporter.Config)
				return cfg
			},
			skipLifecycle: true, // causes race detector to fail
		},
		{
			exporter: "sumologic",
			getConfigFn: func() component.Config {
				cfg := expFactories["sumologic"].CreateDefaultConfig().(*sumologicexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "syslog",
			getConfigFn: func() component.Config {
				cfg := expFactories["syslog"].CreateDefaultConfig().(*syslogexporter.Config)
				cfg.Endpoint = "http://" + endpoint
				// disable queue to validate passing the test data synchronously
				cfg.QueueSettings.Enabled = false
				cfg.BackOffConfig.Enabled = false
				return cfg
			},
			expectConsumeErr: true,
		},
		{
			exporter: "tencentcloud_logservice",
			getConfigFn: func() component.Config {
				cfg := expFactories["tencentcloud_logservice"].CreateDefaultConfig().(*tencentcloudlogserviceexporter.Config)
				return cfg
			},
			expectConsumeErr: true,
		},
	}

	assert.Equal(t, len(expFactories), len(tests), "All user configurable components must be added to the lifecycle test")
	for _, tt := range tests {
		t.Run(string(tt.exporter), func(t *testing.T) {
			factory := expFactories[tt.exporter]
			assert.Equal(t, tt.exporter, factory.Type())
			t.Run("shutdown", func(t *testing.T) {
				verifyExporterShutdown(t, factory, tt.getConfigFn)
			})
			t.Run("lifecycle", func(t *testing.T) {
				if tt.skipLifecycle {
					t.SkipNow()
				}
				verifyExporterLifecycle(t, factory, tt.getConfigFn, tt.expectConsumeErr)
			})
		})
	}
}

// GetExporterConfigFn is used customize the configuration passed to the verification.
// This is used to change ports or provide values required but not provided by the
// default configuration.
type getExporterConfigFn func() component.Config

// verifyExporterLifecycle is used to test if an exporter type can handle the typical
// lifecycle of a component. The getConfigFn parameter only need to be specified if
// the test can't be done with the default configuration for the component.
func verifyExporterLifecycle(t *testing.T, factory exporter.Factory, getConfigFn getExporterConfigFn, expectErr bool) {
	ctx := context.Background()
	host := newAssertNoErrorHost(t)
	expCreateSettings := exportertest.NewNopSettings()

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
			var err error
			assert.NotPanics(t, func() {
				switch e := exp.(type) {
				case exporter.Logs:
					logs := generateTestLogs()
					if !e.Capabilities().MutatesData {
						logs.MarkReadOnly()
					}
					err = e.ConsumeLogs(ctx, logs)
				case exporter.Metrics:
					metrics := generateTestMetrics()
					if !e.Capabilities().MutatesData {
						metrics.MarkReadOnly()
					}
					err = e.ConsumeMetrics(ctx, metrics)
				case exporter.Traces:
					traces := generateTestTraces()
					if !e.Capabilities().MutatesData {
						traces.MarkReadOnly()
					}
					err = e.ConsumeTraces(ctx, traces)
				}
			})
			if !expectErr {
				assert.NoError(t, err)
			}
			assert.NoError(t, exp.Shutdown(ctx))
		}
	}
}

func generateTestLogs() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("resource", "R1")
	l := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	l.Body().SetStr("test log message")
	l.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return logs
}

func generateTestMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("resource", "R1")
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test_metric")
	dp := m.Gauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("test_attr", "value_1")
	dp.SetIntValue(123)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return metrics
}

func generateTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("resource", "R1")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("test_attr", "value_1")
	span.SetName("test_span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return traces
}

// verifyExporterShutdown is used to test if an exporter type can be shutdown without being started first.
func verifyExporterShutdown(tb testing.TB, factory exporter.Factory, getConfigFn getExporterConfigFn) {
	ctx := context.Background()
	expCreateSettings := exportertest.NewNopSettings()

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
	set exporter.Settings,
	cfg component.Config,
) (component.Component, error)

func wrapCreateLogsExp(factory exporter.Factory) createExporterFn {
	return func(ctx context.Context, set exporter.Settings, cfg component.Config) (component.Component, error) {
		return factory.CreateLogsExporter(ctx, set, cfg)
	}
}

func wrapCreateTracesExp(factory exporter.Factory) createExporterFn {
	return func(ctx context.Context, set exporter.Settings, cfg component.Config) (component.Component, error) {
		return factory.CreateTracesExporter(ctx, set, cfg)
	}
}

func wrapCreateMetricsExp(factory exporter.Factory) createExporterFn {
	return func(ctx context.Context, set exporter.Settings, cfg component.Config) (component.Component, error) {
		return factory.CreateMetricsExporter(ctx, set, cfg)
	}
}
