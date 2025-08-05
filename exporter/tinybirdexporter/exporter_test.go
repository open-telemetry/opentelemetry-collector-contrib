// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal/metadata"
)

func TestNewExporter(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "build exporter",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://localhost:8080",
				},
				Token: "test-token",
				Metrics: metricSignalConfigs{
					MetricsGauge:                SignalConfig{Datasource: "metrics_gauge"},
					MetricsSum:                  SignalConfig{Datasource: "metrics_sum"},
					MetricsHistogram:            SignalConfig{Datasource: "metrics_histogram"},
					MetricsExponentialHistogram: SignalConfig{Datasource: "metrics_exponential_histogram"},
				},
				Traces: SignalConfig{Datasource: "traces_test"},
				Logs:   SignalConfig{Datasource: "logs_test"},
				Wait:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := newExporter(tt.config, exportertest.NewNopSettings(metadata.Type))
			assert.NotNil(t, exp)
		})
	}
}

func TestExportTraces(t *testing.T) {
	type args struct {
		traces ptrace.Traces
		config Config
	}
	type want struct {
		requestQuery   string
		requestBody    string
		responseStatus int
		err            error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "export without traces",
			args: args{
				traces: func() ptrace.Traces {
					traces := ptrace.NewTraces()
					rs := traces.ResourceSpans().AppendEmpty()
					rs.ScopeSpans().AppendEmpty()
					return traces
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Traces:       SignalConfig{Datasource: "traces_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=traces_test",
				requestBody:    "",
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with full trace",
			args: args{
				traces: func() ptrace.Traces {
					traces := ptrace.NewTraces()
					rs := traces.ResourceSpans().AppendEmpty()
					rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rs.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					ss := rs.ScopeSpans().AppendEmpty()
					ss.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := ss.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					span := ss.Spans().AppendEmpty()
					span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					span.SetParentSpanID(pcommon.SpanID([8]byte{9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetName("test-span")
					span.SetKind(ptrace.SpanKindServer)
					span.SetStartTimestamp(pcommon.Timestamp(1719158400000000000)) // 2024-06-23T16:00:00Z
					span.SetEndTimestamp(pcommon.Timestamp(1719158401000000000))   // 2024-06-23T16:00:01Z
					span.Status().SetCode(ptrace.StatusCodeOk)
					span.Status().SetMessage("success")
					span.Attributes().PutStr("http.method", "GET")
					span.Attributes().PutStr("http.url", "/api/users")
					span.Attributes().PutStr("user.id", "12345")

					// Add span event
					event := span.Events().AppendEmpty()
					event.SetName("exception")
					event.SetTimestamp(pcommon.Timestamp(1719158400500000000)) // 2024-06-23T16:00:00.5Z
					event.Attributes().PutStr("exception.type", "RuntimeException")
					event.Attributes().PutStr("exception.message", "Something went wrong")

					// Add span link
					link := span.Links().AppendEmpty()
					link.SetTraceID(pcommon.TraceID([16]byte{17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}))
					link.SetSpanID(pcommon.SpanID([8]byte{17, 18, 19, 20, 21, 22, 23, 24}))
					link.TraceState().FromRaw("sampled=true")
					link.Attributes().PutStr("link.type", "child")

					return traces
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Traces:       SignalConfig{Datasource: "traces_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=traces_test",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_name":"test-scope","scope_version":"1.0.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","parent_span_id":"090a0b0c0d0e0f10","trace_state":"","trace_flags":0,"span_name":"test-span","span_kind":"Server","span_attributes":{"http.method":"GET","http.url":"/api/users","user.id":"12345"},"start_time":"2024-06-23T16:00:00Z","end_time":"2024-06-23T16:00:01Z","duration":1000000000,"status_code":"Ok","status_message":"success","events_timestamp":["2024-06-23T16:00:00.5Z"],"events_name":["exception"],"events_attributes":[{"exception.type":"RuntimeException","exception.message":"Something went wrong"}],"links_trace_id":["1112131415161718191a1b1c1d1e1f20"],"links_span_id":["1112131415161718"],"links_trace_state":["sampled=true"],"links_attributes":[{"link.type":"child"}]}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v0/events", r.URL.Path)
				assert.Equal(t, tt.want.requestQuery, r.URL.RawQuery)
				assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer "+string(tt.args.config.Token), r.Header.Get("Authorization"))
				gotBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.JSONEq(t, tt.want.requestBody, string(gotBody))

				w.WriteHeader(tt.want.responseStatus)
			}))
			defer server.Close()

			tt.args.config.ClientConfig.Endpoint = server.URL

			exp := newExporter(&tt.args.config, exportertest.NewNopSettings(metadata.Type))
			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

			err := exp.pushTraces(context.Background(), tt.args.traces)
			if tt.want.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExportMetrics(t *testing.T) {
	type args struct {
		metrics pmetric.Metrics
		config  Config
	}
	type want struct {
		requestQuery   string
		requestBody    string
		responseStatus int
		err            error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "export without metrics",
			args: args{
				metrics: func() pmetric.Metrics {
					metrics := pmetric.NewMetrics()
					rm := metrics.ResourceMetrics().AppendEmpty()
					rm.ScopeMetrics().AppendEmpty()
					return metrics
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Metrics: metricSignalConfigs{
						MetricsGauge: SignalConfig{Datasource: "metrics_gauge"},
						MetricsSum:   SignalConfig{Datasource: "metrics_sum"},
					},
					Wait: false,
				},
			},
			want: want{
				requestQuery:   "name=metrics_gauge",
				requestBody:    "",
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with gauge metric",
			args: args{
				metrics: func() pmetric.Metrics {
					metrics := pmetric.NewMetrics()
					rm := metrics.ResourceMetrics().AppendEmpty()
					rm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rm.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					sm := rm.ScopeMetrics().AppendEmpty()
					sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := sm.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					metric := sm.Metrics().AppendEmpty()
					metric.SetName("test.gauge")
					metric.SetDescription("Test gauge metric")
					metric.SetUnit("bytes")
					gauge := metric.SetEmptyGauge()
					dp := gauge.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000)) // 2024-06-23T16:00:00Z
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))      // 2024-06-23T16:00:01Z
					dp.SetDoubleValue(1024.5)
					dp.Attributes().PutStr("host", "server-1")
					dp.Attributes().PutStr("region", "us-west")

					// Add exemplar
					exemplar := dp.Exemplars().AppendEmpty()
					exemplar.SetTimestamp(pcommon.Timestamp(1719158400500000000)) // 2024-06-23T16:00:00.5Z
					exemplar.SetDoubleValue(1500.0)
					exemplar.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					exemplar.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					exemplar.FilteredAttributes().PutStr("exemplar.type", "outlier")

					return metrics
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Metrics: metricSignalConfigs{
						MetricsGauge: SignalConfig{Datasource: "metrics_gauge"},
						MetricsSum:   SignalConfig{Datasource: "metrics_sum"},
					},
					Wait: false,
				},
			},
			want: want{
				requestQuery:   "name=metrics_gauge",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_name":"test-scope","scope_version":"1.0.0","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"metric_name":"test.gauge","metric_description":"Test gauge metric","metric_unit":"bytes","metric_attributes":{"host":"server-1","region":"us-west"},"start_timestamp":"2024-06-23T16:00:00Z","timestamp":"2024-06-23T16:00:01Z","flags":0,"exemplars_filtered_attributes":[{"exemplar.type":"outlier"}],"exemplars_timestamp":["2024-06-23T16:00:00.5Z"],"exemplars_value":[1500],"exemplars_span_id":["0102030405060708"],"exemplars_trace_id":["0102030405060708090a0b0c0d0e0f10"],"value":1024.5}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with sum metric",
			args: args{
				metrics: func() pmetric.Metrics {
					metrics := pmetric.NewMetrics()
					rm := metrics.ResourceMetrics().AppendEmpty()
					rm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rm.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					sm := rm.ScopeMetrics().AppendEmpty()
					sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := sm.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					metric := sm.Metrics().AppendEmpty()
					metric.SetName("test.sum")
					metric.SetDescription("Test sum metric")
					metric.SetUnit("requests")
					sum := metric.SetEmptySum()
					sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					sum.SetIsMonotonic(true)
					dp := sum.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000)) // 2024-06-23T16:00:00Z
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))      // 2024-06-23T16:00:01Z
					dp.SetIntValue(150)
					dp.Attributes().PutStr("endpoint", "/api/users")
					dp.Attributes().PutStr("method", "GET")

					return metrics
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Metrics: metricSignalConfigs{
						MetricsGauge: SignalConfig{Datasource: "metrics_gauge"},
						MetricsSum:   SignalConfig{Datasource: "metrics_sum"},
					},
					Wait: false,
				},
			},
			want: want{
				requestQuery:   "name=metrics_sum",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_name":"test-scope","scope_version":"1.0.0","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"metric_name":"test.sum","metric_description":"Test sum metric","metric_unit":"requests","metric_attributes":{"endpoint":"/api/users","method":"GET"},"start_timestamp":"2024-06-23T16:00:00Z","timestamp":"2024-06-23T16:00:01Z","flags":0,"exemplars_filtered_attributes":[],"exemplars_timestamp":[],"exemplars_value":[],"exemplars_span_id":[],"exemplars_trace_id":[],"value":150,"aggregation_temporality":1,"is_monotonic":true}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with histogram metric",
			args: args{
				metrics: func() pmetric.Metrics {
					metrics := pmetric.NewMetrics()
					rm := metrics.ResourceMetrics().AppendEmpty()
					rm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rm.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					sm := rm.ScopeMetrics().AppendEmpty()
					sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := sm.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					metric := sm.Metrics().AppendEmpty()
					metric.SetName("test.histogram")
					metric.SetDescription("Test histogram metric")
					metric.SetUnit("seconds")
					histogram := metric.SetEmptyHistogram()
					histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					dp := histogram.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000)) // 2024-06-23T16:00:00Z
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))      // 2024-06-23T16:00:01Z
					dp.SetCount(100)
					dp.SetSum(50.5)
					dp.SetMin(0.1)
					dp.SetMax(2.0)
					dp.BucketCounts().FromRaw([]uint64{10, 20, 30, 40})
					dp.ExplicitBounds().FromRaw([]float64{0.5, 1.0, 1.5})
					dp.Attributes().PutStr("operation", "database_query")
					dp.Attributes().PutStr("table", "users")

					return metrics
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Metrics: metricSignalConfigs{
						MetricsHistogram: SignalConfig{Datasource: "metrics_histogram"},
					},
					Wait: false,
				},
			},
			want: want{
				requestQuery:   "name=metrics_histogram",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_name":"test-scope","scope_version":"1.0.0","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"metric_name":"test.histogram","metric_description":"Test histogram metric","metric_unit":"seconds","metric_attributes":{"operation":"database_query","table":"users"},"start_timestamp":"2024-06-23T16:00:00Z","timestamp":"2024-06-23T16:00:01Z","flags":0,"exemplars_filtered_attributes":[],"exemplars_timestamp":[],"exemplars_value":[],"exemplars_span_id":[],"exemplars_trace_id":[],"count":100,"sum":50.5,"bucket_counts":[10,20,30,40],"explicit_bounds":[0.5,1,1.5],"min":0.1,"max":2,"aggregation_temporality":1}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with exponential histogram metric",
			args: args{
				metrics: func() pmetric.Metrics {
					metrics := pmetric.NewMetrics()
					rm := metrics.ResourceMetrics().AppendEmpty()
					rm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rm.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					sm := rm.ScopeMetrics().AppendEmpty()
					sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := sm.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					metric := sm.Metrics().AppendEmpty()
					metric.SetName("test.exponential_histogram")
					metric.SetDescription("Test exponential histogram metric")
					metric.SetUnit("seconds")
					expHistogram := metric.SetEmptyExponentialHistogram()
					expHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					dp := expHistogram.DataPoints().AppendEmpty()
					dp.SetStartTimestamp(pcommon.Timestamp(1719158400000000000)) // 2024-06-23T16:00:00Z
					dp.SetTimestamp(pcommon.Timestamp(1719158401000000000))      // 2024-06-23T16:00:01Z
					dp.SetCount(200)
					dp.SetSum(75.25)
					dp.SetMin(0.05)
					dp.SetMax(5.0)
					dp.SetScale(2)
					dp.SetZeroCount(15)
					dp.Positive().SetOffset(1)
					dp.Positive().BucketCounts().FromRaw([]uint64{5, 10, 15, 20, 25})
					dp.Negative().SetOffset(-2)
					dp.Negative().BucketCounts().FromRaw([]uint64{3, 7, 12, 18})
					dp.Attributes().PutStr("operation", "api_request")
					dp.Attributes().PutStr("endpoint", "/api/data")

					return metrics
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Metrics: metricSignalConfigs{
						MetricsExponentialHistogram: SignalConfig{Datasource: "metrics_exponential_histogram"},
					},
					Wait: false,
				},
			},
			want: want{
				requestQuery:   "name=metrics_exponential_histogram",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_name":"test-scope","scope_version":"1.0.0","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"metric_name":"test.exponential_histogram","metric_description":"Test exponential histogram metric","metric_unit":"seconds","metric_attributes":{"operation":"api_request","endpoint":"/api/data"},"start_timestamp":"2024-06-23T16:00:00Z","timestamp":"2024-06-23T16:00:01Z","flags":0,"exemplars_filtered_attributes":[],"exemplars_timestamp":[],"exemplars_value":[],"exemplars_span_id":[],"exemplars_trace_id":[],"count":200,"sum":75.25,"scale":2,"zero_count":15,"positive_offset":1,"positive_bucket_counts":[5,10,15,20,25],"negative_offset":-2,"negative_bucket_counts":[3,7,12,18],"min":0.05,"max":5,"aggregation_temporality":1}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v0/events", r.URL.Path)
				assert.Equal(t, tt.want.requestQuery, r.URL.RawQuery)
				assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer "+string(tt.args.config.Token), r.Header.Get("Authorization"))
				gotBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.JSONEq(t, tt.want.requestBody, string(gotBody))

				w.WriteHeader(tt.want.responseStatus)
			}))
			defer server.Close()

			tt.args.config.ClientConfig.Endpoint = server.URL

			exp := newExporter(&tt.args.config, exportertest.NewNopSettings(metadata.Type))
			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

			err := exp.pushMetrics(context.Background(), tt.args.metrics)
			if tt.want.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExportLogs(t *testing.T) {
	type args struct {
		logs   plog.Logs
		config Config
	}
	type want struct {
		requestQuery   string
		requestBody    string
		responseStatus int
		err            error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "export without logs",
			args: args{
				logs: func() plog.Logs {
					logs := plog.NewLogs()
					rl := logs.ResourceLogs().AppendEmpty()
					rl.ScopeLogs().AppendEmpty()
					return logs
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Logs:         SignalConfig{Datasource: "logs_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=logs_test",
				requestBody:    "",
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with full log",
			args: args{
				logs: func() plog.Logs {
					logs := plog.NewLogs()
					rl := logs.ResourceLogs().AppendEmpty()
					rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rl.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					sl := rl.ScopeLogs().AppendEmpty()
					sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := sl.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					log := sl.LogRecords().AppendEmpty()
					log.Body().SetStr("User login attempt")
					log.Attributes().PutStr("http.method", "POST")
					log.Attributes().PutStr("http.url", "/api/login")
					log.Attributes().PutStr("user.id", "12345")
					log.SetTimestamp(pcommon.Timestamp(1719158401000000000)) // 2024-06-23T16:00:01Z
					log.SetSeverityText("INFO")
					log.SetSeverityNumber(plog.SeverityNumberInfo)
					log.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					log.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					log.SetFlags(plog.LogRecordFlags(1))
					return logs
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Logs:         SignalConfig{Datasource: "logs_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=logs_test",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_name":"test-scope","scope_version":"1.0.0","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"body":"User login attempt","log_attributes":{"http.method":"POST","http.url":"/api/login","user.id":"12345"},"timestamp":"2024-06-23T16:00:01Z","severity_text":"INFO","severity_number":9,"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","flags":1}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v0/events", r.URL.Path)
				assert.Equal(t, tt.want.requestQuery, r.URL.RawQuery)
				assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer "+string(tt.args.config.Token), r.Header.Get("Authorization"))
				gotBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.JSONEq(t, tt.want.requestBody, string(gotBody))

				w.WriteHeader(tt.want.responseStatus)
			}))
			defer server.Close()

			tt.args.config.ClientConfig.Endpoint = server.URL

			exp := newExporter(&tt.args.config, exportertest.NewNopSettings(metadata.Type))
			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

			err := exp.pushLogs(context.Background(), tt.args.logs)
			if tt.want.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExportErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		headers        map[string]string
		wantErr        bool
	}{
		{
			name:           "success",
			responseStatus: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "throttled",
			responseStatus: http.StatusTooManyRequests,
			headers:        map[string]string{"Retry-After": "30"},
			wantErr:        true,
		},
		{
			name:           "service unavailable",
			responseStatus: http.StatusServiceUnavailable,
			wantErr:        true,
		},
		{
			name:           "permanent error",
			responseStatus: http.StatusBadRequest,
			responseBody:   "invalid request",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					_, err := w.Write([]byte(tt.responseBody))
					assert.NoError(t, err)
				}
			}))
			defer server.Close()

			config := &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: server.URL,
				},
				Token: "test-token",
				Metrics: metricSignalConfigs{
					MetricsGauge:                SignalConfig{Datasource: "metrics_gauge"},
					MetricsSum:                  SignalConfig{Datasource: "metrics_sum"},
					MetricsHistogram:            SignalConfig{Datasource: "metrics_histogram"},
					MetricsExponentialHistogram: SignalConfig{Datasource: "metrics_exponential_histogram"},
				},
				Traces: SignalConfig{Datasource: "traces_test"},
				Logs:   SignalConfig{Datasource: "logs_test"},
			}

			exp := newExporter(config, exportertest.NewNopSettings(metadata.Type))
			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			lr := sl.LogRecords().AppendEmpty()
			lr.Body().SetStr("test-log")
			err := exp.pushLogs(context.Background(), logs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
