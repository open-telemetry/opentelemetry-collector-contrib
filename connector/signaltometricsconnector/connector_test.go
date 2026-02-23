// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signaltometricsconnector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const testDataDir = "testdata"

func TestConnectorWithTraces(t *testing.T) {
	testCases := []string{
		"sum",
		"histograms",
		"exponential_histograms",
		"metric_identity",
		"gauge",
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			traceTestDataDir := filepath.Join(testDataDir, "traces")
			inputTraces, err := golden.ReadTraces(filepath.Join(traceTestDataDir, "traces.yaml"))
			require.NoError(t, err)

			next := &consumertest.MetricsSink{}
			tcTestDataDir := filepath.Join(traceTestDataDir, tc)
			factory, settings, cfg := setupConnector(t, tcTestDataDir)
			connector, err := factory.CreateTracesToMetrics(ctx, settings, cfg, next)
			require.NoError(t, err)
			require.IsType(t, &signalToMetrics{}, connector)
			expectedMetrics, err := golden.ReadMetrics(filepath.Join(tcTestDataDir, "output.yaml"))
			require.NoError(t, err)

			require.NoError(t, connector.ConsumeTraces(ctx, inputTraces))
			require.Len(t, next.AllMetrics(), 1)
			assertAggregatedMetrics(t, expectedMetrics, next.AllMetrics()[0])
		})
	}
}

func TestConnectorWithMetrics(t *testing.T) {
	testCases := []string{
		"sum",
		"histograms",
		"exponential_histograms",
		"gauge",
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			metricTestDataDir := filepath.Join(testDataDir, "metrics")
			inputMetrics, err := golden.ReadMetrics(filepath.Join(metricTestDataDir, "metrics.yaml"))
			require.NoError(t, err)

			next := &consumertest.MetricsSink{}
			tcTestDataDir := filepath.Join(metricTestDataDir, tc)
			factory, settings, cfg := setupConnector(t, tcTestDataDir)
			connector, err := factory.CreateMetricsToMetrics(ctx, settings, cfg, next)
			require.NoError(t, err)
			require.IsType(t, &signalToMetrics{}, connector)
			expectedMetrics, err := golden.ReadMetrics(filepath.Join(tcTestDataDir, "output.yaml"))
			require.NoError(t, err)

			require.NoError(t, connector.ConsumeMetrics(ctx, inputMetrics))
			require.Len(t, next.AllMetrics(), 1)
			assertAggregatedMetrics(t, expectedMetrics, next.AllMetrics()[0])
		})
	}
}

func TestConnectorWithLogs(t *testing.T) {
	testCases := []string{
		"sum",
		"histograms",
		"exponential_histograms",
		"metric_identity",
		"gauge",
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			logTestDataDir := filepath.Join(testDataDir, "logs")
			inputLogs, err := golden.ReadLogs(filepath.Join(logTestDataDir, "logs.yaml"))
			require.NoError(t, err)

			next := &consumertest.MetricsSink{}
			tcTestDataDir := filepath.Join(logTestDataDir, tc)
			factory, settings, cfg := setupConnector(t, tcTestDataDir)
			connector, err := factory.CreateLogsToMetrics(ctx, settings, cfg, next)
			require.NoError(t, err)
			require.IsType(t, &signalToMetrics{}, connector)
			expectedMetrics, err := golden.ReadMetrics(filepath.Join(tcTestDataDir, "output.yaml"))
			require.NoError(t, err)

			require.NoError(t, connector.ConsumeLogs(ctx, inputLogs))
			require.Len(t, next.AllMetrics(), 1)
			assertAggregatedMetrics(t, expectedMetrics, next.AllMetrics()[0])
		})
	}
}

func TestConnectorWithProfiles(t *testing.T) {
	testCases := []string{
		"sum",
		"histograms",
		"exponential_histograms",
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			profileTestDataDir := filepath.Join(testDataDir, "profiles")
			inputProfiles, err := golden.ReadProfiles(filepath.Join(profileTestDataDir, "profiles.yaml"))
			require.NoError(t, err)

			next := &consumertest.MetricsSink{}
			tcTestDataDir := filepath.Join(profileTestDataDir, tc)
			factory, settings, cfg := setupConnector(t, tcTestDataDir)
			connector, err := factory.CreateProfilesToMetrics(ctx, settings, cfg, next)
			require.NoError(t, err)
			require.IsType(t, &signalToMetrics{}, connector)

			require.NoError(t, connector.ConsumeProfiles(ctx, inputProfiles))
			require.Len(t, next.AllMetrics(), 1)

			expectedMetrics, err := golden.ReadMetrics(filepath.Join(tcTestDataDir, "output.yaml"))
			require.NoError(t, err)

			assertAggregatedMetrics(t, expectedMetrics, next.AllMetrics()[0])
		})
	}
}

func BenchmarkConnectorWithTraces(b *testing.B) {
	factory := NewFactory()
	settings := connectortest.NewNopSettings(metadata.Type)
	settings.Logger = zaptest.NewLogger(b, zaptest.Level(zapcore.DebugLevel))
	next, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
		return nil
	})
	require.NoError(b, err)

	cfg := &config.Config{Spans: testMetricInfo(b)}
	require.NoError(b, cfg.Unmarshal(confmap.New())) // set required fields to default
	require.NoError(b, cfg.Validate())
	connector, err := factory.CreateTracesToMetrics(b.Context(), settings, cfg, next)
	require.NoError(b, err)
	inputTraces, err := golden.ReadTraces("testdata/traces/traces.yaml")
	require.NoError(b, err)

	b.ReportAllocs()

	for b.Loop() {
		if err := connector.ConsumeTraces(b.Context(), inputTraces); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConnectorWithMetrics(b *testing.B) {
	factory := NewFactory()
	settings := connectortest.NewNopSettings(metadata.Type)
	settings.Logger = zaptest.NewLogger(b, zaptest.Level(zapcore.DebugLevel))
	next, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
		return nil
	})
	require.NoError(b, err)

	cfg := &config.Config{Datapoints: testMetricInfo(b)}
	require.NoError(b, cfg.Unmarshal(confmap.New())) // set required fields to default
	require.NoError(b, cfg.Validate())
	connector, err := factory.CreateMetricsToMetrics(b.Context(), settings, cfg, next)
	require.NoError(b, err)
	inputMetrics, err := golden.ReadMetrics("testdata/metrics/metrics.yaml")
	require.NoError(b, err)

	b.ReportAllocs()

	for b.Loop() {
		if err := connector.ConsumeMetrics(b.Context(), inputMetrics); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConnectorWithLogs(b *testing.B) {
	factory := NewFactory()
	settings := connectortest.NewNopSettings(metadata.Type)
	settings.Logger = zaptest.NewLogger(b, zaptest.Level(zapcore.DebugLevel))
	next, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error {
		return nil
	})
	require.NoError(b, err)

	cfg := &config.Config{Logs: testMetricInfo(b)}
	require.NoError(b, cfg.Unmarshal(confmap.New())) // set required fields to default
	require.NoError(b, cfg.Validate())
	connector, err := factory.CreateLogsToMetrics(b.Context(), settings, cfg, next)
	require.NoError(b, err)
	inputLogs, err := golden.ReadLogs("testdata/logs/logs.yaml")
	require.NoError(b, err)

	b.ReportAllocs()

	for b.Loop() {
		if err := connector.ConsumeLogs(b.Context(), inputLogs); err != nil {
			b.Fatal(err)
		}
	}
}

// testMetricInfo creates a metric info with all metric types that could be used
// for all the supported signals. To do this, it uses common OTTL funcs and literals.
func testMetricInfo(b *testing.B) []config.MetricInfo {
	b.Helper()

	return []config.MetricInfo{
		{
			Name:        "test.histogram",
			Description: "Test histogram",
			Unit:        "ms",
			IncludeResourceAttributes: []config.Attribute{
				{
					Key: "resource.foo",
				},
				{
					Key:          "404.attribute",
					DefaultValue: "test_404_attribute",
				},
			},
			Attributes: []config.Attribute{
				{
					Key: "http.response.status_code",
				},
			},
			Histogram: configoptional.Some(config.Histogram{
				Buckets: []float64{2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000},
				Value:   "1.4",
			}),
		},
		{
			Name:        "test.exphistogram",
			Description: "Test exponential histogram",
			Unit:        "ms",
			IncludeResourceAttributes: []config.Attribute{
				{
					Key: "resource.foo",
				},
				{
					Key:          "404.attribute",
					DefaultValue: "test_404_attribute",
				},
			},
			Attributes: []config.Attribute{
				{
					Key: "http.response.status_code",
				},
			},
			ExponentialHistogram: configoptional.Some(config.ExponentialHistogram{
				Value:   "2.4",
				MaxSize: 160,
			}),
		},
		{
			Name:        "test.sum",
			Description: "Test sum",
			Unit:        "ms",
			IncludeResourceAttributes: []config.Attribute{
				{
					Key: "resource.foo",
				},
				{
					Key:          "404.attribute",
					DefaultValue: "test_404_attribute",
				},
			},
			Attributes: []config.Attribute{
				{
					Key: "http.response.status_code",
				},
			},
			Sum: configoptional.Some(config.Sum{
				Value: "5.4",
			}),
		},
	}
}

func setupConnector(
	t *testing.T, testFilePath string,
) (xconnector.Factory, connector.Settings, component.Config) {
	t.Helper()
	factory := NewFactory()
	settings := connectortest.NewNopSettings(metadata.Type)
	telemetryResource(t).CopyTo(settings.Resource)
	settings.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))

	cfg := createDefaultConfig()
	cm, err := confmaptest.LoadConf(filepath.Join(testFilePath, "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(&cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	return factory.(xconnector.Factory), settings, cfg
}

func telemetryResource(t *testing.T) pcommon.Resource {
	t.Helper()

	r := pcommon.NewResource()
	r.Attributes().PutStr("service.instance.id", "627cc493-f310-47de-96bd-71410b7dec09")
	r.Attributes().PutStr("service.name", "signal_to_metrics")
	r.Attributes().PutStr("service.namespace", "test")
	return r
}

func assertAggregatedMetrics(t *testing.T, expected, actual pmetric.Metrics) {
	t.Helper()
	assert.NoError(t, pmetrictest.CompareMetrics(
		expected, actual,
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreTimestamp(),
	))
}

// TestErrorMode tests error handling behavior with different error modes
func TestErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		expectError   bool
		expectLogs    bool
		expectMetrics bool
	}{
		{
			name:          "propagate error",
			errorMode:     ottl.PropagateError,
			expectError:   true,
			expectLogs:    false,
			expectMetrics: false,
		},
		{
			name:          "ignore error",
			errorMode:     ottl.IgnoreError,
			expectError:   false,
			expectLogs:    true,
			expectMetrics: true,
		},
		{
			name:          "silent error",
			errorMode:     ottl.SilentError,
			expectError:   false,
			expectLogs:    false,
			expectMetrics: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("traces", func(t *testing.T) {
				ctx := t.Context()
				factory := NewFactory()

				// Setup logger observer to capture logs
				observedZapCore, observedLogs := observer.New(zap.InfoLevel)
				settings := connectortest.NewNopSettings(metadata.Type)
				settings.Logger = zap.New(observedZapCore)

				cfg := &config.Config{
					ErrorMode: tt.errorMode,
					Spans: []config.MetricInfo{
						{
							Name:        "test.sum",
							Description: "Test sum",
							Sum: configoptional.Some(config.Sum{
								// Will fail on invalid attribute
								Value: `Int(attributes["invalid_numeric"])`,
							}),
						},
					},
				}

				next := &consumertest.MetricsSink{}
				conn, err := factory.CreateTracesToMetrics(ctx, settings, cfg, next)
				require.NoError(t, err)

				// Create trace with invalid attribute
				traces := ptrace.NewTraces()
				span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetName("test-span")
				span.Attributes().PutStr("invalid_numeric", "not-a-number")

				err = conn.ConsumeTraces(ctx, traces)

				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if tt.expectLogs {
					assert.Positive(t, observedLogs.Len(), "expected error to be logged")
				} else {
					assert.Empty(t, observedLogs.All(), "expected no logs")
				}

				if tt.expectMetrics {
					assert.Len(t, next.AllMetrics(), 1, "expected metrics to be generated")
				} else {
					assert.Empty(t, next.AllMetrics(), "expected no metrics")
				}
			})

			t.Run("logs", func(t *testing.T) {
				ctx := t.Context()
				factory := NewFactory()

				observedZapCore, observedLogs := observer.New(zap.InfoLevel)
				settings := connectortest.NewNopSettings(metadata.Type)
				settings.Logger = zap.New(observedZapCore)

				cfg := &config.Config{
					ErrorMode: tt.errorMode,
					Logs: []config.MetricInfo{
						{
							Name:        "test.sum",
							Description: "Test sum",
							Sum: configoptional.Some(config.Sum{
								Value: `Int(attributes["invalid_numeric"])`,
							}),
						},
					},
				}

				next := &consumertest.MetricsSink{}
				conn, err := factory.CreateLogsToMetrics(ctx, settings, cfg, next)
				require.NoError(t, err)

				logs := plog.NewLogs()
				logRecord := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				logRecord.Body().SetStr("test log")
				logRecord.Attributes().PutStr("invalid_numeric", "not-a-number")

				err = conn.ConsumeLogs(ctx, logs)

				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if tt.expectLogs {
					assert.Positive(t, observedLogs.Len())
				} else {
					assert.Empty(t, observedLogs.All())
				}

				if tt.expectMetrics {
					assert.Len(t, next.AllMetrics(), 1)
				} else {
					assert.Empty(t, next.AllMetrics())
				}
			})
		})
	}
}
