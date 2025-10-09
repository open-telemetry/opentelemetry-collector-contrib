// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestPrometheusExporter(t *testing.T) {
	tests := []struct {
		config       func() *Config
		wantErr      string
		wantStartErr string
	}{
		{
			config: func() *Config {
				return &Config{
					Namespace: "test",
					ConstLabels: map[string]string{
						"foo0":  "bar0",
						"code0": "one0",
					},
					ServerConfig: confighttp.ServerConfig{
						Endpoint: testutil.GetAvailableLocalAddress(t),
					},
					SendTimestamps:   false,
					MetricExpiration: 60 * time.Second,
				}
			},
		},
		{
			config: func() *Config {
				return &Config{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:88999",
					},
				}
			},
			wantStartErr: "listen tcp: address 88999: invalid port",
		},
		{
			config:  func() *Config { return &Config{} },
			wantErr: "expecting a non-blank address to run the Prometheus metrics handler",
		},
	}

	factory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	for _, tt := range tests {
		// Run it a few times to ensure that shutdowns exit cleanly.
		for range 3 {
			cfg := tt.config()
			exp, err := factory.CreateMetrics(t.Context(), set, cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				continue
			}
			require.NoError(t, err)

			assert.NotNil(t, exp)
			err = exp.Start(t.Context(), componenttest.NewNopHost())

			if tt.wantStartErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantStartErr, err.Error())
			} else {
				require.NoError(t, err)
			}

			require.NoError(t, exp.Shutdown(t.Context()))
		}
	}
}

func TestPrometheusExporter_WithTLS(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := &Config{
		Namespace: "test",
		ConstLabels: map[string]string{
			"foo2":  "bar2",
			"code2": "one2",
		},
		ServerConfig: confighttp.ServerConfig{
			Endpoint: addr,
			TLS: configoptional.Some(configtls.ServerConfig{
				Config: configtls.Config{
					CertFile: "./testdata/certs/server.crt",
					KeyFile:  "./testdata/certs/server.key",
					CAFile:   "./testdata/certs/ca.crt",
				},
			}),
		},
		SendTimestamps:   true,
		MetricExpiration: 120 * time.Minute,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{
			Enabled: true,
		},
	}
	factory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(t.Context(), set, cfg)
	require.NoError(t, err)

	tlscs := configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "./testdata/certs/ca.crt",
			CertFile: "./testdata/certs/client.crt",
			KeyFile:  "./testdata/certs/client.key",
		},
		ServerName: "localhost",
	}
	tls, err := tlscs.LoadTLSConfig(t.Context())
	assert.NoError(t, err)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tls,
		},
	}

	defer func() {
		require.NoError(t, exp.Shutdown(t.Context()))
	}()

	assert.NotNil(t, exp)

	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	assert.NoError(t, exp.ConsumeMetrics(t.Context(), md))

	rsp, err := httpClient.Get("https://" + addr + "/metrics")
	require.NoError(t, err, "Failed to perform a scrape")

	assert.Equal(t, http.StatusOK, rsp.StatusCode, "Mismatched HTTP response status code")

	blob, _ := io.ReadAll(rsp.Body)
	_ = rsp.Body.Close()

	want := []string{
		`# HELP test_counter_int`,
		`# TYPE test_counter_int counter`,
		`test_counter_int{code2="one2",foo2="bar2",label_1="label-value-1",otel_scope_name="",otel_scope_schema_url="",otel_scope_version="",resource_attr="resource-attr-val-1"} 123 1581452773000`,
		`test_counter_int{code2="one2",foo2="bar2",label_2="label-value-2",otel_scope_name="",otel_scope_schema_url="",otel_scope_version="",resource_attr="resource-attr-val-1"} 456 1581452773000`,
	}

	for _, w := range want {
		assert.Contains(t, string(blob), w, "Missing %v from response:\n%v", w, string(blob))
	}
}

// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4986
func TestPrometheusExporter_endToEndMultipleTargets(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := &Config{
		Namespace: "test",
		ConstLabels: map[string]string{
			"foo1":  "bar1",
			"code1": "one1",
		},
		ServerConfig: confighttp.ServerConfig{
			Endpoint: addr,
		},
		MetricExpiration: 120 * time.Minute,
	}

	factory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(t.Context(), set, cfg)
	assert.NoError(t, err)

	defer func() {
		require.NoError(t, exp.Shutdown(t.Context()))
	}()

	assert.NotNil(t, exp)

	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

	// Should accumulate multiple metrics from different targets
	assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(128, "metric_1_", "cpu-exporter", "localhost:8080")))
	assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(128, "metric_1_", "cpu-exporter", "localhost:8081")))

	for delta := 0; delta <= 20; delta += 10 {
		assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(int64(delta), "metric_2_", "cpu-exporter", "localhost:8080")))
		assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(int64(delta), "metric_2_", "cpu-exporter", "localhost:8081")))

		res, err1 := http.Get("http://" + addr + "/metrics")
		require.NoError(t, err1, "Failed to perform a scrape")

		assert.Equal(t, http.StatusOK, res.StatusCode, "Mismatched HTTP response status code")
		blob, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		want := []string{
			`# HELP test_metric_1_this_one_there_where Extra ones`,
			`# TYPE test_metric_1_this_one_there_where counter`,
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 99+128),
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 100+128),
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8081",job="cpu-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 99+128),
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8081",job="cpu-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 100+128),
			`# HELP test_metric_2_this_one_there_where Extra ones`,
			`# TYPE test_metric_2_this_one_there_where counter`,
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 99+delta),
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 100+delta),
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8081",job="cpu-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 99+delta),
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8081",job="cpu-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 100+delta),
		}

		for _, w := range want {
			assert.Contains(t, string(blob), w, "Missing %v from response:\n%v", w, string(blob))
		}
	}

	// Expired metrics should be removed during first scrape
	exp.(*wrapMetricsExporter).exporter.collector.accumulator.(*lastValueAccumulator).metricExpiration = 1 * time.Millisecond
	time.Sleep(10 * time.Millisecond)

	res, err := http.Get("http://" + addr + "/metrics")
	require.NoError(t, err, "Failed to perform a scrape")

	assert.Equal(t, http.StatusOK, res.StatusCode, "Mismatched HTTP response status code")
	blob, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	require.Emptyf(t, string(blob), "Metrics did not expire")
}

func TestPrometheusExporter_endToEnd(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := &Config{
		Namespace: "test",
		ConstLabels: map[string]string{
			"foo1":  "bar1",
			"code1": "one1",
		},
		ServerConfig: confighttp.ServerConfig{
			Endpoint: addr,
		},
		MetricExpiration: 120 * time.Minute,
	}

	factory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(t.Context(), set, cfg)
	assert.NoError(t, err)

	defer func() {
		require.NoError(t, exp.Shutdown(t.Context()))
	}()

	assert.NotNil(t, exp)

	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

	// Should accumulate multiple metrics
	assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(128, "metric_1_", "cpu-exporter", "localhost:8080")))

	for delta := 0; delta <= 20; delta += 10 {
		assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(int64(delta), "metric_2_", "cpu-exporter", "localhost:8080")))

		res, err1 := http.Get("http://" + addr + "/metrics")
		require.NoError(t, err1, "Failed to perform a scrape")

		assert.Equal(t, http.StatusOK, res.StatusCode, "Mismatched HTTP response status code")
		blob, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		want := []string{
			`# HELP test_metric_1_this_one_there_where Extra ones`,
			`# TYPE test_metric_1_this_one_there_where counter`,
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 99+128),
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 100+128),
			`# HELP test_metric_2_this_one_there_where Extra ones`,
			`# TYPE test_metric_2_this_one_there_where counter`,
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 99+delta),
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code1="one1",foo1="bar1",instance="localhost:8080",job="cpu-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v`, 100+delta),
		}

		for _, w := range want {
			assert.Contains(t, string(blob), w, "Missing %v from response:\n%v", w, string(blob))
		}
	}

	// Expired metrics should be removed during first scrape
	exp.(*wrapMetricsExporter).exporter.collector.accumulator.(*lastValueAccumulator).metricExpiration = 1 * time.Millisecond
	time.Sleep(10 * time.Millisecond)

	res, err := http.Get("http://" + addr + "/metrics")
	require.NoError(t, err, "Failed to perform a scrape")

	assert.Equal(t, http.StatusOK, res.StatusCode, "Mismatched HTTP response status code")
	blob, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	require.Emptyf(t, string(blob), "Metrics did not expire")
}

func TestPrometheusExporter_endToEndWithTimestamps(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := &Config{
		Namespace: "test",
		ConstLabels: map[string]string{
			"foo2":  "bar2",
			"code2": "one2",
		},
		ServerConfig: confighttp.ServerConfig{
			Endpoint: addr,
		},
		SendTimestamps:   true,
		MetricExpiration: 120 * time.Minute,
	}

	factory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(t.Context(), set, cfg)
	assert.NoError(t, err)

	defer func() {
		require.NoError(t, exp.Shutdown(t.Context()))
	}()

	assert.NotNil(t, exp)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

	// Should accumulate multiple metrics

	assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(128, "metric_1_", "node-exporter", "localhost:8080")))

	for delta := 0; delta <= 20; delta += 10 {
		assert.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(int64(delta), "metric_2_", "node-exporter", "localhost:8080")))

		res, err1 := http.Get("http://" + addr + "/metrics")
		require.NoError(t, err1, "Failed to perform a scrape")

		assert.Equal(t, http.StatusOK, res.StatusCode, "Mismatched HTTP response status code")
		blob, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		want := []string{
			`# HELP test_metric_1_this_one_there_where Extra ones`,
			`# TYPE test_metric_1_this_one_there_where counter`,
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code2="one2",foo2="bar2",instance="localhost:8080",job="node-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v %v`, 99+128, 1543160298100+128000),
			fmt.Sprintf(`test_metric_1_this_one_there_where{arch="x86",code2="one2",foo2="bar2",instance="localhost:8080",job="node-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v %v`, 100+128, 1543160298100),
			`# HELP test_metric_2_this_one_there_where Extra ones`,
			`# TYPE test_metric_2_this_one_there_where counter`,
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code2="one2",foo2="bar2",instance="localhost:8080",job="node-exporter",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v %v`, 99+delta, 1543160298100+delta*1000),
			fmt.Sprintf(`test_metric_2_this_one_there_where{arch="x86",code2="one2",foo2="bar2",instance="localhost:8080",job="node-exporter",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} %v %v`, 100+delta, 1543160298100),
		}

		for _, w := range want {
			assert.Contains(t, string(blob), w, "Missing %v from response:\n%v", w, string(blob))
		}
	}

	// Expired metrics should be removed during first scrape
	exp.(*wrapMetricsExporter).exporter.collector.accumulator.(*lastValueAccumulator).metricExpiration = 1 * time.Millisecond
	time.Sleep(10 * time.Millisecond)

	res, err := http.Get("http://" + addr + "/metrics")
	require.NoError(t, err, "Failed to perform a scrape")

	assert.Equal(t, http.StatusOK, res.StatusCode, "Mismatched HTTP response status code")
	blob, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	require.Emptyf(t, string(blob), "Metrics did not expire")
}

func TestPrometheusExporter_endToEndWithResource(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	cfg := &Config{
		Namespace: "test",
		ConstLabels: map[string]string{
			"foo2":  "bar2",
			"code2": "one2",
		},
		ServerConfig: confighttp.ServerConfig{
			Endpoint: addr,
		},
		SendTimestamps:   true,
		MetricExpiration: 120 * time.Minute,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{
			Enabled: true,
		},
	}

	factory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(t.Context(), set, cfg)
	assert.NoError(t, err)

	defer func() {
		require.NoError(t, exp.Shutdown(t.Context()))
	}()

	assert.NotNil(t, exp)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	assert.NoError(t, exp.ConsumeMetrics(t.Context(), md))

	rsp, err := http.Get("http://" + addr + "/metrics")
	require.NoError(t, err, "Failed to perform a scrape")

	assert.Equal(t, http.StatusOK, rsp.StatusCode, "Mismatched HTTP response status code")

	blob, _ := io.ReadAll(rsp.Body)
	_ = rsp.Body.Close()

	want := []string{
		`# HELP test_counter_int`,
		`# TYPE test_counter_int counter`,
		`test_counter_int{code2="one2",foo2="bar2",label_1="label-value-1",otel_scope_name="",otel_scope_schema_url="",otel_scope_version="",resource_attr="resource-attr-val-1"} 123 1581452773000`,
		`test_counter_int{code2="one2",foo2="bar2",label_2="label-value-2",otel_scope_name="",otel_scope_schema_url="",otel_scope_version="",resource_attr="resource-attr-val-1"} 456 1581452773000`,
	}

	for _, w := range want {
		assert.Contains(t, string(blob), w, "Missing %v from response:\n%v", w, string(blob))
	}
}

func metricBuilder(delta int64, prefix, job, instance string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics().AppendEmpty()
	rms0 := md.ResourceMetrics().At(0)
	rms0.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), job)
	rms0.Resource().Attributes().PutStr(string(conventions.ServiceInstanceIDKey), instance)

	ms := rms.ScopeMetrics().AppendEmpty().Metrics()

	m1 := ms.AppendEmpty()
	m1.SetName(prefix + "this/one/there(where)")
	m1.SetDescription("Extra ones")
	m1.SetUnit("By")
	d1 := m1.SetEmptySum()
	d1.SetIsMonotonic(true)
	d1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp1 := d1.DataPoints().AppendEmpty()
	dp1.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1543160298+delta, 100000090)))
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1543160298+delta, 100000997)))
	dp1.Attributes().PutStr("os", "windows")
	dp1.Attributes().PutStr("arch", "x86")
	dp1.SetIntValue(99 + delta)

	m2 := ms.AppendEmpty()
	m2.SetName(prefix + "this/one/there(where)")
	m2.SetDescription("Extra ones")
	m2.SetUnit("By")
	d2 := m2.SetEmptySum()
	d2.SetIsMonotonic(true)
	d2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp2 := d2.DataPoints().AppendEmpty()
	dp2.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1543160298, 100000090)))
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1543160298, 100000997)))
	dp2.Attributes().PutStr("os", "linux")
	dp2.Attributes().PutStr("arch", "x86")
	dp2.SetIntValue(100 + delta)

	return md
}

func TestPrometheusExporter_TranslationStrategies(t *testing.T) {
	tests := []struct {
		name               string
		featureGateEnabled bool
		config             *Config
		extraHeaders       map[string]string
		want               string
	}{
		{
			name:               "Legacy AddMetricSuffixes=true (no translation_strategy set)",
			featureGateEnabled: false,
			config: &Config{
				AddMetricSuffixes: true,
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where_bytes_total Extra ones
# TYPE this_one_there_where_bytes_total counter
this_one_there_where_bytes_total{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where_bytes_total{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name:               "Legacy AddMetricSuffixes=false (no translation_strategy set)",
			featureGateEnabled: false,
			config: &Config{
				AddMetricSuffixes: false,
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where Extra ones
# TYPE this_one_there_where counter
this_one_there_where{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name:               "Legacy AddMetricSuffixes=true with feature gate enabled (no translation_strategy set)",
			featureGateEnabled: true,
			config: &Config{
				AddMetricSuffixes: true, // Should be ignored and default 'translation_strategy' is used (UnderscoreEscapingWithSuffixes).
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where_bytes_total Extra ones
# TYPE this_one_there_where_bytes_total counter
this_one_there_where_bytes_total{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where_bytes_total{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name:               "TranslationStrategy takes precedence over AddMetricSuffixes (feature gate disabled)",
			featureGateEnabled: false,
			config: &Config{
				AddMetricSuffixes:   true, // This should be ignored
				TranslationStrategy: underscoreEscapingWithoutSuffixes,
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where Extra ones
# TYPE this_one_there_where counter
this_one_there_where{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name: "UnderscoreEscapingWithSuffixes",
			config: &Config{
				TranslationStrategy: underscoreEscapingWithSuffixes,
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where_bytes_total Extra ones
# TYPE this_one_there_where_bytes_total counter
this_one_there_where_bytes_total{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where_bytes_total{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name: "UnderscoreEscapingWithoutSuffixes",
			config: &Config{
				TranslationStrategy: underscoreEscapingWithoutSuffixes,
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where Extra ones
# TYPE this_one_there_where counter
this_one_there_where{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name: "NoUTF8EscapingWithSuffixes/escaping=allow-utf-8",
			config: &Config{
				TranslationStrategy: noUTF8EscapingWithSuffixes,
			},
			extraHeaders: map[string]string{
				"Accept": "application/openmetrics-text;version=1.0.0;escaping=allow-utf-8;q=0.6,application/openmetrics-text;version=0.0.1;q=0.5,text/plain;version=1.0.0;escaping=allow-utf-8;q=0.4,text/plain;version=0.0.4;q=0.3,*/*;q=0.2",
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP "this/one/there(where)_bytes_total" Extra ones
# TYPE "this/one/there(where)_bytes_total" counter
{"this/one/there(where)_bytes_total",arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
{"this/one/there(where)_bytes_total",arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name: "NoUTF8EscapingWithSuffixes/escaping=underscores",
			config: &Config{
				TranslationStrategy: noUTF8EscapingWithSuffixes,
			},
			extraHeaders: map[string]string{
				"Accept": "application/openmetrics-text;version=1.0.0;escaping=underscores;q=0.5,application/openmetrics-text;version=0.0.1;q=0.4,text/plain;version=1.0.0;escaping=underscores;q=0.3,text/plain;version=0.0.4;q=0.2,/;q=0.1",
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where__bytes_total Extra ones
# TYPE this_one_there_where__bytes_total counter
this_one_there_where__bytes_total{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where__bytes_total{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name:               "NoTranslation/escaping=allow-utf-8",
			featureGateEnabled: true,
			config: &Config{
				TranslationStrategy: noTranslation,
			},
			extraHeaders: map[string]string{
				"Accept": "application/openmetrics-text;version=1.0.0;escaping=allow-utf-8;q=0.6,application/openmetrics-text;version=0.0.1;q=0.5,text/plain;version=1.0.0;escaping=allow-utf-8;q=0.4,text/plain;version=0.0.4;q=0.3,*/*;q=0.2",
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP "this/one/there(where)" Extra ones
# TYPE "this/one/there(where)" counter
{"this/one/there(where)",arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
{"this/one/there(where)",arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
		{
			name: "NoTranslation/escaping=underscores",
			config: &Config{
				TranslationStrategy: noTranslation,
			},
			extraHeaders: map[string]string{
				"Accept": "application/openmetrics-text;version=1.0.0;escaping=underscores;q=0.5,application/openmetrics-text;version=0.0.1;q=0.4,text/plain;version=1.0.0;escaping=underscores;q=0.3,text/plain;version=0.0.4;q=0.2,/;q=0.1",
			},
			want: `# HELP target_info Target metadata
# TYPE target_info gauge
target_info{instance="test-instance",job="test-service"} 1
# HELP this_one_there_where_ Extra ones
# TYPE this_one_there_where_ counter
this_one_there_where_{arch="x86",instance="test-instance",job="test-service",os="linux",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 100
this_one_there_where_{arch="x86",instance="test-instance",job="test-service",os="windows",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 99
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set feature gate state for this test
			originalState := disableAddMetricSuffixesFeatureGate.IsEnabled()
			testutil.SetFeatureGateForTest(t, disableAddMetricSuffixesFeatureGate, tt.featureGateEnabled)
			defer testutil.SetFeatureGateForTest(t, disableAddMetricSuffixesFeatureGate, originalState)

			// Configure the exporter
			addr := testutil.GetAvailableLocalAddress(t)
			cfg := tt.config
			cfg.ServerConfig = confighttp.ServerConfig{
				Endpoint: addr,
			}
			cfg.MetricExpiration = 120 * time.Minute

			factory := NewFactory()
			set := exportertest.NewNopSettings(metadata.Type)
			exp, err := factory.CreateMetrics(t.Context(), set, cfg)
			require.NoError(t, err)

			defer func() {
				require.NoError(t, exp.Shutdown(t.Context()))
			}()

			assert.NotNil(t, exp)
			require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

			md := metricBuilder(0, "", "test-service", "test-instance")
			assert.NoError(t, exp.ConsumeMetrics(t.Context(), md))

			// Scrape metrics, with the Accept header set to the value specified in the test case
			req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/metrics", http.NoBody)
			require.NoError(t, err)
			for k, v := range tt.extraHeaders {
				req.Header.Set(k, v)
			}
			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "Failed to perform a scrape")
			assert.Equal(t, http.StatusOK, res.StatusCode, "Mismatched HTTP response status code")

			blob, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			output := string(blob)

			assert.Equal(t, tt.want, output)
		})
	}
}
