// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	promconfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func setFeatureGate(t *testing.T, gate *featuregate.Gate, enabled bool) {
	t.Helper()
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), prev))
	})
}

func TestInitScrapeOptionsEnableOpenMetrics2FeatureGate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rcvr, err := newPrometheusReceiver(receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	t.Run("disabled by default", func(t *testing.T) {
		assert.False(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate.IsEnabled())
		opts := rcvr.initScrapeOptions(prometheusScrapeTestOptions{})
		assert.False(t, opts.EnableOpenMetrics2)
	})

	t.Run("enabled via feature gate", func(t *testing.T) {
		setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, true)
		opts := rcvr.initScrapeOptions(prometheusScrapeTestOptions{})
		assert.True(t, opts.EnableOpenMetrics2)
	})
}

func TestConfigOpenMetrics2ScrapeProtocol(t *testing.T) {
	cm := confmap.NewFromStringMap(map[string]any{
		"config": map[string]any{
			"global": map[string]any{
				"scrape_protocols": []any{"OpenMetricsText2.0.0"},
			},
			"scrape_configs": []any{
				map[string]any{
					"job_name":         "openmetrics2",
					"scrape_interval":  "5s",
					"scrape_protocols": []any{"OpenMetricsText2.0.0", "OpenMetricsText1.0.0"},
					"static_configs": []any{
						map[string]any{
							"targets": []any{"localhost:8080"},
						},
					},
				},
			},
		},
	})

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	require.NoError(t, cm.Unmarshal(cfg))
	require.NoError(t, confmap.Validate(cfg))

	require.Equal(t, []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}, cfg.PrometheusConfig.GlobalConfig.ScrapeProtocols)
	require.Len(t, cfg.PrometheusConfig.ScrapeConfigs, 1)
	assert.Equal(t, []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0, promconfig.OpenMetricsText1_0_0}, cfg.PrometheusConfig.ScrapeConfigs[0].ScrapeProtocols)
}

func TestOpenMetrics2ConfigWarnings(t *testing.T) {
	newObservedSettings := func() (receiver.Settings, *observer.ObservedLogs) {
		core, obs := observer.New(zap.WarnLevel)
		set := receivertest.NewNopSettings(metadata.Type)
		set.Logger = zap.New(core)
		return set, obs
	}

	t.Run("deduplicates global and inherited per-job warnings while warning on explicit job and fallback", func(t *testing.T) {
		setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, false)

		cfg := createDefaultConfig().(*Config)
		cfg.PrometheusConfig.GlobalConfig.ScrapeProtocols = []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}
		cfg.PrometheusConfig.ScrapeConfigs = []*promconfig.ScrapeConfig{
			{
				// Inherits global.scrape_protocols: should not emit a duplicate per-job warning.
				JobName:                "inherited-om2-job",
				ScrapeProtocols:        []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0},
				ScrapeFallbackProtocol: promconfig.PrometheusText0_0_4,
			},
			{
				// Differs from global.scrape_protocols and sets fallback_scrape_protocol to OM2.
				JobName:                "explicit-om2-job",
				ScrapeProtocols:        []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0, promconfig.OpenMetricsText1_0_0},
				ScrapeFallbackProtocol: promconfig.OpenMetricsText2_0_0,
			},
		}

		set, obs := newObservedSettings()
		rcvr, err := NewFactory().CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rcvr.Shutdown(t.Context()))
		}()

		msgs := make([]string, 0, obs.Len())
		for _, entry := range obs.All() {
			msgs = append(msgs, entry.Message)
		}
		assert.Equal(t, []string{
			"OpenMetricsText2.0.0 is configured in global.scrape_protocols, but the feature gate is disabled; OpenMetrics 2.0 responses will fall back to fallback_scrape_protocol (or fail if none is configured)",
			"OpenMetricsText2.0.0 is configured in scrape_protocols, but the feature gate is disabled; OpenMetrics 2.0 responses will fall back to fallback_scrape_protocol (or fail if none is configured)",
			"fallback_scrape_protocol OpenMetricsText2.0.0 is not supported by the pinned Prometheus scrape library and currently falls back to OpenMetrics 1.0",
		}, msgs)
	})

	t.Run("warns on fallback_scrape_protocol OpenMetricsText2.0.0 even when gate is enabled", func(t *testing.T) {
		setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, true)

		cfg := createDefaultConfig().(*Config)
		cfg.PrometheusConfig.GlobalConfig.ScrapeProtocols = []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}
		cfg.PrometheusConfig.ScrapeConfigs = []*promconfig.ScrapeConfig{
			{
				JobName:                "om2-fallback-job",
				ScrapeProtocols:        []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0},
				ScrapeFallbackProtocol: promconfig.OpenMetricsText2_0_0,
			},
		}

		set, obs := newObservedSettings()
		rcvr, err := NewFactory().CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rcvr.Shutdown(t.Context()))
		}()

		require.Equal(t, 1, obs.Len())
		assert.Equal(t, "fallback_scrape_protocol OpenMetricsText2.0.0 is not supported by the pinned Prometheus scrape library and currently falls back to OpenMetrics 1.0", obs.All()[0].Message)
	})

	t.Run("warns for scrape_config_files via programmatic config and confmap", func(t *testing.T) {
		setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, false)

		scrapeFile := filepath.Join(t.TempDir(), "scrape.yaml")
		require.NoError(t, os.WriteFile(scrapeFile, []byte(`scrape_configs:
  - job_name: file-om2-job
    scrape_protocols: [OpenMetricsText2.0.0]
    fallback_scrape_protocol: OpenMetricsText2.0.0
    static_configs:
      - targets: ["localhost:8080"]
`), 0o600))

		// Programmatic config via createDefaultConfig() where PrometheusConfig starts unloaded.
		progCfg := createDefaultConfig().(*Config)
		progCfg.PrometheusConfig.ScrapeConfigFiles = []string{scrapeFile}

		set, obs := newObservedSettings()
		rcvr, err := NewFactory().CreateMetrics(t.Context(), set, progCfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NoError(t, rcvr.Shutdown(t.Context()))

		assert.Equal(t, 2, obs.Len())

		// YAML/confmap-loaded config.
		cm := confmap.NewFromStringMap(map[string]any{
			"config": map[string]any{
				"scrape_config_files": []any{scrapeFile},
			},
		})
		cmCfg := NewFactory().CreateDefaultConfig().(*Config)
		require.NoError(t, cm.Unmarshal(cmCfg))
		require.NoError(t, confmap.Validate(cmCfg))

		set2, obs2 := newObservedSettings()
		rcvr2, err := NewFactory().CreateMetrics(t.Context(), set2, cmCfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NoError(t, rcvr2.Shutdown(t.Context()))

		assert.Equal(t, 2, obs2.Len())
	})

	t.Run("nil PrometheusConfig does not panic in configWarnings and returns error from factory", func(t *testing.T) {
		set, obs := newObservedSettings()
		assert.NotPanics(t, func() {
			configWarnings(set.Logger, &Config{})
		})
		assert.Zero(t, obs.Len())

		_, err := NewFactory().CreateMetrics(t.Context(), set, &Config{}, consumertest.NewNop())
		require.ErrorContains(t, err, "prometheus config cannot be nil")
	})

	t.Run("no warning when gate enabled and fallback is PrometheusText0.0.4", func(t *testing.T) {
		setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, true)

		cfg := createDefaultConfig().(*Config)
		cfg.PrometheusConfig.GlobalConfig.ScrapeProtocols = []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}
		cfg.PrometheusConfig.ScrapeConfigs = []*promconfig.ScrapeConfig{
			{
				JobName:                "om2-job",
				ScrapeProtocols:        []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0},
				ScrapeFallbackProtocol: promconfig.PrometheusText0_0_4,
			},
		}

		set, obs := newObservedSettings()
		rcvr, err := NewFactory().CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
		require.NoError(t, err)
		defer func() {
			require.NoError(t, rcvr.Shutdown(t.Context()))
		}()

		assert.Zero(t, obs.Len())
	})
}

func TestScrapeViaOpenMetrics2(t *testing.T) {
	setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, true)

	om2Payload := `# TYPE http_requests counter
# HELP http_requests Total HTTP requests.
# UNIT http_requests seconds
http_requests{foo="bar"} 17 # {id="e1"} 1.0 1700000001.0 # {id="e2"} 2.0 1700000002.0
# TYPE test_gauge gauge
# HELP test_gauge A test gauge.
test_gauge 33.5
# TYPE test_histogram histogram
# HELP test_histogram A test classic histogram.
test_histogram {count:3,sum:6.0,bucket:[0.1:1,1.0:2,+Inf:3]}
# TYPE "http.request.duration" histogram
# HELP "http.request.duration" UTF-8 composite histogram.
# UNIT "http.request.duration" seconds
{"http.request.duration"} {count:2,sum:1.5,bucket:[0.5:1,+Inf:2]}
# TYPE test_native_histogram histogram
# HELP test_native_histogram A test native histogram.
test_native_histogram {count:6,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:3],positive_buckets:[1,2,1]}
# TYPE test_summary summary
# HELP test_summary A test summary.
test_summary {count:100,sum:300.0,quantile:[0.5:10.0,0.9:20.0,0.99:30.0]}
# EOF
`

	expectations := []metricExpectation{
		{
			"http_requests",
			pmetric.MetricTypeSum,
			"s",
			[]dataPointExpectation{{
				numberPointComparator: []numberPointComparator{
					compareDoubleValue(17),
					compareAttributes(map[string]string{"foo": "bar"}),
					func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
						require.Equal(t, 2, numberDataPoint.Exemplars().Len())
						assert.Equal(t, 1.0, numberDataPoint.Exemplars().At(0).DoubleValue())
						assert.Equal(t, 2.0, numberDataPoint.Exemplars().At(1).DoubleValue())
					},
				},
			}},
			func(t *testing.T, metric pmetric.Metric) {
				compareMetricIsMonotonic(true)(t, metric)
				assert.Equal(t, "Total HTTP requests.", metric.Description())
			},
		},
		{
			"test_gauge",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{{
				numberPointComparator: []numberPointComparator{
					compareDoubleValue(33.5),
				},
			}},
			func(t *testing.T, metric pmetric.Metric) {
				assert.Equal(t, "A test gauge.", metric.Description())
			},
		},
		{
			"test_histogram",
			pmetric.MetricTypeHistogram,
			"",
			[]dataPointExpectation{{
				histogramPointComparator: []histogramPointComparator{
					compareHistogram(3, 6.0, []float64{0.1, 1.0}, []uint64{1, 1, 1}),
				},
			}},
			nil,
		},
		{
			"http.request.duration",
			pmetric.MetricTypeHistogram,
			"s",
			[]dataPointExpectation{{
				histogramPointComparator: []histogramPointComparator{
					compareHistogram(2, 1.5, []float64{0.5}, []uint64{1, 1}),
				},
			}},
			func(t *testing.T, metric pmetric.Metric) {
				assert.Equal(t, "UTF-8 composite histogram.", metric.Description())
			},
		},
		{
			"test_native_histogram",
			pmetric.MetricTypeExponentialHistogram,
			"",
			[]dataPointExpectation{{
				exponentialHistogramComparator: []exponentialHistogramComparator{
					compareExponentialHistogram(0, 6, 12.1, 2, 0, nil, -1, []uint64{1, 2, 1}),
				},
			}},
			nil,
		},
		{
			"test_summary",
			pmetric.MetricTypeSummary,
			"",
			[]dataPointExpectation{{
				summaryPointComparator: []summaryPointComparator{
					compareSummary(100, 300.0, [][]float64{{0.5, 10.0}, {0.9, 20.0}, {0.99, 30.0}}),
				},
			}},
			nil,
		},
	}

	var acceptHeader atomic.Value
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{
					code:        200,
					data:        om2Payload,
					contentType: "application/openmetrics-text; version=2.0.0",
					onRequest: func(req *http.Request) {
						acceptHeader.Store(req.Header.Get("Accept"))
					},
				},
			},
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyNumValidScrapeResults(t, td, result)
				doCompare(t, "target1", td.attributes, result[0], expectations)
			},
		},
	}

	testComponent(t, targets, func(c *Config) {
		truePtr := true
		c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}
		for _, sc := range c.PrometheusConfig.ScrapeConfigs {
			sc.ScrapeProtocols = nil
			sc.ScrapeNativeHistograms = &truePtr
		}
	})

	gotAccept, _ := acceptHeader.Load().(string)
	assert.Contains(t, gotAccept, "application/openmetrics-text;version=2.0.0")
}

func TestScrapeViaOpenMetrics2StartTimestamp(t *testing.T) {
	setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, true)

	om2Payload := `# TYPE st_counter counter
st_counter 42.0 st@1700000000.5
# EOF
`
	for _, tc := range []struct {
		name          string
		enableParseST bool
		wantStartTS   pcommon.Timestamp
	}{
		{
			name:          "start timestamp dropped when created timestamp gate disabled",
			enableParseST: false,
			wantStartTS:   0,
		},
		{
			name:          "start timestamp populated when created timestamp gate enabled",
			enableParseST: true,
			wantStartTS:   pcommon.Timestamp(1700000000500000000),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFeatureGate(t, metadata.ReceiverPrometheusreceiverEnableCreatedTimestampZeroIngestionFeatureGate, tc.enableParseST)

			expectations := []metricExpectation{
				{
					"st_counter",
					pmetric.MetricTypeSum,
					"",
					[]dataPointExpectation{{
						numberPointComparator: []numberPointComparator{
							compareDoubleValue(42.0),
							compareStartTimestamp(tc.wantStartTS),
						},
					}},
					nil,
				},
			}

			targets := []*testData{
				{
					name: "target1",
					pages: []mockPrometheusResponse{
						{
							code:        200,
							data:        om2Payload,
							contentType: "application/openmetrics-text; version=2.0.0",
						},
					},
					validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
						verifyNumValidScrapeResults(t, td, result)
						doCompare(t, "target1", td.attributes, result[0], expectations)
					},
				},
			}

			testComponent(t, targets, func(c *Config) {
				c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}
				for _, sc := range c.PrometheusConfig.ScrapeConfigs {
					sc.ScrapeProtocols = nil
				}
			})
		})
	}
}

func TestScrapeViaOpenMetrics2Disabled(t *testing.T) {
	setFeatureGate(t, metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate, false)

	t.Run("default fallback rejects composite histogram and drops scalar unit", func(t *testing.T) {
		var acceptHeader atomic.Value
		compositePayload := `# TYPE test_histogram histogram
test_histogram {count:3,sum:6.0,bucket:[0.1:1,1.0:2,+Inf:3]}
# EOF
`
		scalarPayload := `# TYPE http_requests counter
# UNIT http_requests seconds
http_requests 17
# EOF
`

		targets := []*testData{
			{
				name: "composite",
				pages: []mockPrometheusResponse{
					{
						code:        200,
						data:        compositePayload,
						contentType: "application/openmetrics-text; version=2.0.0",
						onRequest: func(req *http.Request) {
							acceptHeader.Store(req.Header.Get("Accept"))
						},
					},
				},
				validateScrapes: true,
				validateFunc: func(t *testing.T, _ *testData, result []pmetric.ResourceMetrics) {
					require.NotEmpty(t, result)
					metrics := getMetrics(result[0])
					assertUp(t, 0, metrics)
					for _, m := range metrics {
						assert.NotEqual(t, "test_histogram", m.Name())
					}
				},
			},
			{
				name: "scalar_unit",
				pages: []mockPrometheusResponse{
					{
						code:        200,
						data:        scalarPayload,
						contentType: "application/openmetrics-text; version=2.0.0",
					},
				},
				validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
					verifyNumValidScrapeResults(t, td, result)
					doCompare(t, "scalar_unit", td.attributes, result[0], []metricExpectation{
						{
							"http_requests",
							pmetric.MetricTypeSum,
							"",
							[]dataPointExpectation{{
								numberPointComparator: []numberPointComparator{
									compareDoubleValue(17),
								},
							}},
							nil,
						},
					})
				},
			},
		}

		testComponent(t, targets, func(c *Config) {
			c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}
			for _, sc := range c.PrometheusConfig.ScrapeConfigs {
				sc.ScrapeProtocols = nil
				sc.ScrapeFallbackProtocol = promconfig.PrometheusText0_0_4
			}
		})

		gotAccept, _ := acceptHeader.Load().(string)
		assert.Contains(t, gotAccept, "application/openmetrics-text;version=2.0.0")
	})

	t.Run("empty fallback rejects scalar payload", func(t *testing.T) {
		om2Payload := `# TYPE test_gauge gauge
test_gauge 33.5
# EOF
`
		targets := []*testData{
			{
				name: "target1",
				pages: []mockPrometheusResponse{
					{
						code:        200,
						data:        om2Payload,
						contentType: "application/openmetrics-text; version=2.0.0",
					},
				},
				validateScrapes: true,
				validateFunc: func(t *testing.T, _ *testData, result []pmetric.ResourceMetrics) {
					require.NotEmpty(t, result)
					metrics := getMetrics(result[0])
					assertUp(t, 0, metrics)
					for _, m := range metrics {
						assert.NotEqual(t, "test_gauge", m.Name())
					}
				},
			},
		}

		testComponent(t, targets, func(c *Config) {
			c.PrometheusConfig.GlobalConfig.ScrapeProtocols = []promconfig.ScrapeProtocol{promconfig.OpenMetricsText2_0_0}
			for _, sc := range c.PrometheusConfig.ScrapeConfigs {
				sc.ScrapeProtocols = nil
			}
		})
	})
}
