// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
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
		for j := 0; j < 3; j++ {
			cfg := tt.config()
			exp, err := factory.CreateMetrics(context.Background(), set, cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				continue
			}
			require.NoError(t, err)

			assert.NotNil(t, exp)
			err = exp.Start(context.Background(), componenttest.NewNopHost())

			if tt.wantStartErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantStartErr, err.Error())
			} else {
				require.NoError(t, err)
			}

			require.NoError(t, exp.Shutdown(context.Background()))
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
			TLSSetting: &configtls.ServerConfig{
				Config: configtls.Config{
					CertFile: "./testdata/certs/server.crt",
					KeyFile:  "./testdata/certs/server.key",
					CAFile:   "./testdata/certs/ca.crt",
				},
			},
		},
		SendTimestamps:   true,
		MetricExpiration: 120 * time.Minute,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{
			Enabled: true,
		},
	}
	factory := NewFactory()
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := factory.CreateMetrics(context.Background(), set, cfg)
	require.NoError(t, err)

	tlscs := configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "./testdata/certs/ca.crt",
			CertFile: "./testdata/certs/client.crt",
			KeyFile:  "./testdata/certs/client.key",
		},
		ServerName: "localhost",
	}
	tls, err := tlscs.LoadTLSConfig(context.Background())
	assert.NoError(t, err)
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tls,
		},
	}

	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})

	assert.NotNil(t, exp)

	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))

	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	assert.NoError(t, exp.ConsumeMetrics(context.Background(), md))

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
	exp, err := factory.CreateMetrics(context.Background(), set, cfg)
	assert.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})

	assert.NotNil(t, exp)

	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))

	// Should accumulate multiple metrics from different targets
	assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(128, "metric_1_", "cpu-exporter", "localhost:8080")))
	assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(128, "metric_1_", "cpu-exporter", "localhost:8081")))

	for delta := 0; delta <= 20; delta += 10 {
		assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(int64(delta), "metric_2_", "cpu-exporter", "localhost:8080")))
		assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(int64(delta), "metric_2_", "cpu-exporter", "localhost:8081")))

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
	exp, err := factory.CreateMetrics(context.Background(), set, cfg)
	assert.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})

	assert.NotNil(t, exp)

	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))

	// Should accumulate multiple metrics
	assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(128, "metric_1_", "cpu-exporter", "localhost:8080")))

	for delta := 0; delta <= 20; delta += 10 {
		assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(int64(delta), "metric_2_", "cpu-exporter", "localhost:8080")))

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
	exp, err := factory.CreateMetrics(context.Background(), set, cfg)
	assert.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})

	assert.NotNil(t, exp)
	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))

	// Should accumulate multiple metrics

	assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(128, "metric_1_", "node-exporter", "localhost:8080")))

	for delta := 0; delta <= 20; delta += 10 {
		assert.NoError(t, exp.ConsumeMetrics(context.Background(), metricBuilder(int64(delta), "metric_2_", "node-exporter", "localhost:8080")))

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
	exp, err := factory.CreateMetrics(context.Background(), set, cfg)
	assert.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})

	assert.NotNil(t, exp)
	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))

	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	assert.NoError(t, exp.ConsumeMetrics(context.Background(), md))

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
	m1.SetUnit("1")
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
	m2.SetUnit("1")
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
