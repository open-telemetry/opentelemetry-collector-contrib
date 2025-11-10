// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logstometricsprocessor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const testDataDir = "testdata"

func TestProcessorWithLogs(t *testing.T) {
	testCases := []string{
		"sum",
		"histograms",
		"exponential_histograms",
		"metric_identity",
		"gauge",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			logTestDataDir := filepath.Join(testDataDir, "logs")
			inputLogs, err := golden.ReadLogs(filepath.Join(logTestDataDir, "logs.yaml"))
			require.NoError(t, err)

			metricsSink := &consumertest.MetricsSink{}
			logsSink := &consumertest.LogsSink{}
			tcTestDataDir := filepath.Join(logTestDataDir, tc)
			factory, settings, cfg := setupProcessor(t, tcTestDataDir, metricsSink)
			processor, err := factory.CreateLogs(ctx, settings, cfg, logsSink)
			require.NoError(t, err)

			expectedMetrics, err := golden.ReadMetrics(filepath.Join(tcTestDataDir, "output.yaml"))
			require.NoError(t, err)

			require.NoError(t, processor.Start(ctx, createTestHost(t, cfg.MetricsExporter, metricsSink)))
			defer func() {
				require.NoError(t, processor.Shutdown(ctx))
			}()

			require.NoError(t, processor.ConsumeLogs(ctx, inputLogs))

			// Verify metrics were extracted
			require.Greater(t, len(metricsSink.AllMetrics()), 0, "expected at least one metrics batch")
			assertAggregatedMetrics(t, expectedMetrics, metricsSink.AllMetrics()[0])

			// Verify logs were forwarded (default behavior)
			require.Greater(t, len(logsSink.AllLogs()), 0, "expected logs to be forwarded")
			assertLogsEqual(t, inputLogs, logsSink.AllLogs()[0])
		})
	}
}

func TestProcessorDropLogs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputLogs, err := golden.ReadLogs(filepath.Join(testDataDir, "logs", "logs.yaml"))
	require.NoError(t, err)

	metricsSink := &consumertest.MetricsSink{}
	logsSink := &consumertest.LogsSink{}

	cfg := &config.Config{
		MetricsExporter: component.MustNewID("test_metrics"),
		DropLogs:       true,
		Logs: []config.MetricInfo{
			{
				Name:        "log.count",
				Description: "Count of log records",
				Sum: configoptional.Some(config.Sum{
					Value: "1",
				}),
			},
		},
	}
	require.NoError(t, cfg.Validate())

	factory := NewFactory()
	settings := processortest.NewNopSettings(metadata.Type)
			processor, err := factory.CreateLogs(ctx, settings, cfg, logsSink)
	require.NoError(t, err)

	require.NoError(t, processor.Start(ctx, createTestHost(t, cfg.MetricsExporter, metricsSink)))
	defer func() {
		require.NoError(t, processor.Shutdown(ctx))
	}()

	require.NoError(t, processor.ConsumeLogs(ctx, inputLogs))

	// Verify metrics were extracted
	require.Greater(t, len(metricsSink.AllMetrics()), 0, "expected metrics to be extracted")

	// Verify logs were dropped - check that no log records were forwarded
	require.Equal(t, 0, logsSink.LogRecordCount(), "expected logs to be dropped when drop_logs is true")
}

func TestProcessorForwardLogs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputLogs, err := golden.ReadLogs(filepath.Join(testDataDir, "logs", "logs.yaml"))
	require.NoError(t, err)

	metricsSink := &consumertest.MetricsSink{}
	logsSink := &consumertest.LogsSink{}

	cfg := &config.Config{
		MetricsExporter: component.MustNewID("test_metrics"),
		DropLogs:       false, // Explicitly set to false
		Logs: []config.MetricInfo{
			{
				Name:        "log.count",
				Description: "Count of log records",
				Sum: configoptional.Some(config.Sum{
					Value: "1",
				}),
			},
		},
	}
	require.NoError(t, cfg.Validate())

	factory := NewFactory()
	settings := processortest.NewNopSettings(metadata.Type)
			processor, err := factory.CreateLogs(ctx, settings, cfg, logsSink)
	require.NoError(t, err)

	require.NoError(t, processor.Start(ctx, createTestHost(t, cfg.MetricsExporter, metricsSink)))
	defer func() {
		require.NoError(t, processor.Shutdown(ctx))
	}()

	require.NoError(t, processor.ConsumeLogs(ctx, inputLogs))

	// Verify metrics were extracted
	require.Greater(t, len(metricsSink.AllMetrics()), 0, "expected metrics to be extracted")

	// Verify logs were forwarded
	require.Greater(t, len(logsSink.AllLogs()), 0, "expected logs to be forwarded when drop_logs is false")
	assertLogsEqual(t, inputLogs, logsSink.AllLogs()[0])
}

func TestProcessorMetricsExporterNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &config.Config{
		MetricsExporter: component.MustNewID("nonexistent"),
		DropLogs:       false,
		Logs: []config.MetricInfo{
			{
				Name:        "log.count",
				Description: "Count of log records",
				Sum: configoptional.Some(config.Sum{
					Value: "1",
				}),
			},
		},
	}
	require.NoError(t, cfg.Validate())

	factory := NewFactory()
	settings := processortest.NewNopSettings(metadata.Type)
	logsSink := &consumertest.LogsSink{}
			processor, err := factory.CreateLogs(ctx, settings, cfg, logsSink)
	require.NoError(t, err)

	// Start should fail because exporter is not found
	// Create a test host that implements exporterHost but doesn't have the exporter
	host := &testHost{
		exporters: map[DataType]map[component.ID]component.Component{
			DataTypeMetrics: {
				// Exporter with different ID, so the one we're looking for won't be found
				component.MustNewID("other_exporter"): &wrappedMetricsSink{MetricsSink: &consumertest.MetricsSink{}},
			},
		},
	}
	err = processor.Start(ctx, host)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestProcessorMetricsExporterError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputLogs, err := golden.ReadLogs(filepath.Join(testDataDir, "logs", "logs.yaml"))
	require.NoError(t, err)

	// Create a metrics exporter that returns an error
	errMetricsExporter := &errorMetricsExporter{err: errors.New("export error")}
	logsSink := &consumertest.LogsSink{}

	cfg := &config.Config{
		MetricsExporter: component.MustNewID("test_metrics"),
		DropLogs:       false,
		Logs: []config.MetricInfo{
			{
				Name:        "log.count",
				Description: "Count of log records",
				Sum: configoptional.Some(config.Sum{
					Value: "1",
				}),
			},
		},
	}
	require.NoError(t, cfg.Validate())

	factory := NewFactory()
	settings := processortest.NewNopSettings(metadata.Type)
			processor, err := factory.CreateLogs(ctx, settings, cfg, logsSink)
	require.NoError(t, err)

	host := createTestHostWithExporter(t, cfg.MetricsExporter, errMetricsExporter)
	require.NoError(t, processor.Start(ctx, host))
	defer func() {
		require.NoError(t, processor.Shutdown(ctx))
	}()

	// Processing should succeed even if metrics export fails
	require.NoError(t, processor.ConsumeLogs(ctx, inputLogs))

	// Logs should still be forwarded despite metrics export error
	require.Greater(t, len(logsSink.AllLogs()), 0, "expected logs to be forwarded even if metrics export fails")
}

func TestProcessorNoMetricDefinitions(t *testing.T) {
	cfg := &config.Config{
		MetricsExporter: component.MustNewID("test_metrics"),
		DropLogs:       false,
		Logs:           []config.MetricInfo{}, // No metric definitions
	}

	// Validation should reject empty logs configuration
	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no logs configuration provided")
}

func TestProcessorInvalidConfig(t *testing.T) {
	testCases := []struct {
		name        string
		cfg         *config.Config
		expectedErr string
	}{
		{
			name: "missing metrics_exporter",
			cfg: &config.Config{
				DropLogs: false,
				Logs: []config.MetricInfo{
					{
						Name:        "log.count",
						Description: "Count of log records",
						Sum: configoptional.Some(config.Sum{
							Value: "1",
						}),
					},
				},
			},
			expectedErr: "metrics_exporter must be specified",
		},
		{
			name: "no logs configuration",
			cfg: &config.Config{
				MetricsExporter: component.MustNewID("test_metrics"),
				DropLogs:       false,
				Logs:           []config.MetricInfo{},
			},
			expectedErr: "no logs configuration provided",
		},
		{
			name: "invalid metric name",
			cfg: &config.Config{
				MetricsExporter: component.MustNewID("test_metrics"),
				DropLogs:       false,
				Logs: []config.MetricInfo{
					{
						Name:        "", // Missing name
						Description:  "Count of log records",
						Sum: configoptional.Some(config.Sum{
							Value: "1",
						}),
					},
				},
			},
			expectedErr: "missing required metric name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func BenchmarkProcessorWithLogs(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	metricsSink := &consumertest.MetricsSink{}
	logsSink := &consumertest.LogsSink{}

	cfg := &config.Config{
		MetricsExporter: component.MustNewID("test_metrics"),
		DropLogs:       false,
		Logs: []config.MetricInfo{
			{
				Name:        "log.count",
				Description: "Count of log records",
				Sum: configoptional.Some(config.Sum{
					Value: "1",
				}),
			},
		},
	}
	require.NoError(b, cfg.Validate())

	factory := NewFactory()
	settings := processortest.NewNopSettings(metadata.Type)
			processor, err := factory.CreateLogs(ctx, settings, cfg, logsSink)
	require.NoError(b, err)

	require.NoError(b, processor.Start(ctx, createTestHost(b, cfg.MetricsExporter, metricsSink)))
	defer func() {
		require.NoError(b, processor.Shutdown(ctx))
	}()

	inputLogs, err := golden.ReadLogs("testdata/logs/logs.yaml")
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := processor.ConsumeLogs(ctx, inputLogs); err != nil {
			b.Fatal(err)
		}
	}
}

// Helper functions

func setupProcessor(t testing.TB, testDataDir string, metricsSink *consumertest.MetricsSink) (processor.Factory, processor.Settings, *config.Config) {
	t.Helper()
	factory := NewFactory()
	settings := processortest.NewNopSettings(metadata.Type)

	cfg := factory.CreateDefaultConfig().(*config.Config)
	cfg.MetricsExporter = component.MustNewID("test_metrics")

	conf, err := confmaptest.LoadConf(filepath.Join(testDataDir, "config.yaml"))
	require.NoError(t, err)
	
	// Extract the processor config from under the processor name key
	sub, err := conf.Sub(metadata.Type.String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Override metrics exporter for testing
	cfg.MetricsExporter = component.MustNewID("test_metrics")
	require.NoError(t, cfg.Validate())

	return factory, settings, cfg
}

func createTestHost(t testing.TB, exporterID component.ID, metricsSink *consumertest.MetricsSink) component.Host {
	// Wrap MetricsSink to implement component.Component
	wrappedExporter := &wrappedMetricsSink{MetricsSink: metricsSink}
	return &testHost{
		exporters: map[DataType]map[component.ID]component.Component{
			DataTypeMetrics: {
				exporterID: wrappedExporter,
			},
		},
	}
}

// wrappedMetricsSink wraps consumertest.MetricsSink to implement component.Component
type wrappedMetricsSink struct {
	*consumertest.MetricsSink
}

func (w *wrappedMetricsSink) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (w *wrappedMetricsSink) Shutdown(ctx context.Context) error {
	return nil
}

func createTestHostWithExporter(t testing.TB, exporterID component.ID, metricsExporter exporter.Metrics) component.Host {
	return &testHost{
		exporters: map[DataType]map[component.ID]component.Component{
			DataTypeMetrics: {
				exporterID: metricsExporter,
			},
		},
	}
}

// testHost is a test implementation of component.Host
type testHost struct {
	exporters map[DataType]map[component.ID]component.Component
}

func (h *testHost) GetExporters() map[DataType]map[component.ID]component.Component {
	return h.exporters
}

func (h *testHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (h *testHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	return nil
}

func assertAggregatedMetrics(t *testing.T, expected, actual pmetric.Metrics) {
	opts := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		// Ignore collector instance info attributes that are added automatically
		pmetrictest.IgnoreResourceAttributeValue("logstometricsprocessor.service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("logstometricsprocessor.service.name"),
		pmetrictest.IgnoreResourceAttributeValue("logstometricsprocessor.service.namespace"),
	}
	assert.NoError(t, pmetrictest.CompareMetrics(expected, actual, opts...))
}

func assertLogsEqual(t *testing.T, expected, actual plog.Logs) {
	assert.NoError(t, plogtest.CompareLogs(expected, actual, plogtest.IgnoreObservedTimestamp()))
}

// errorMetricsExporter is a test metrics exporter that always returns an error
type errorMetricsExporter struct {
	err error
}

func (e *errorMetricsExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (e *errorMetricsExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (e *errorMetricsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *errorMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return e.err
}

