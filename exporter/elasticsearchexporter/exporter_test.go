// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/extension/auth/authtest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestExporterLogs(t *testing.T) {
	t.Run("publish with success", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL)
		mustSendLogRecords(t, exporter, plog.NewLogRecord())
		mustSendLogRecords(t, exporter, plog.NewLogRecord())

		rec.WaitItems(2)
	})

	t.Run("publish with ecs encoding", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			expected := `{"@timestamp":"1970-01-01T00:00:00.000000000Z","agent":{"name":"otlp"},"application":"myapp","attrKey1":"abc","attrKey2":"def","error":{"stacktrace":"no no no no"},"message":"hello world","service":{"name":"myservice"}}`
			actual := string(docs[0].Document)
			assert.Equal(t, expected, actual)

			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "ecs"
		})
		logs := newLogsWithAttributeAndResourceMap(
			// record attrs
			map[string]string{
				"application":          "myapp",
				"service.name":         "myservice",
				"exception.stacktrace": "no no no no",
			},
			// resource attrs
			map[string]string{
				"attrKey1": "abc",
				"attrKey2": "def",
			},
		)
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("hello world")
		mustSendLogs(t, exporter, logs)
		rec.WaitItems(1)
	})

	t.Run("publish with dedot", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			assert.JSONEq(t,
				`{"attr":{"key":"value"},"agent":{"name":"otlp"},"@timestamp":"1970-01-01T00:00:00.000000000Z"}`,
				string(docs[0].Document),
			)
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "ecs"
			cfg.Mapping.Dedot = true
		})
		logs := newLogsWithAttributeAndResourceMap(
			map[string]string{"attr.key": "value"},
			nil,
		)
		mustSendLogs(t, exporter, logs)
		rec.WaitItems(1)
	})

	t.Run("publish with dedup", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			assert.Equal(t, `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Scope":{"name":"","value":"value","version":""},"SeverityNumber":0,"TraceFlags":0}`, string(docs[0].Document))
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "raw"
			// dedup is the default
		})
		logs := newLogsWithAttributeAndResourceMap(
			// Scope collides with the top-level "Scope" field,
			// so will be removed during deduplication.
			map[string]string{"Scope": "value"},
			nil,
		)
		mustSendLogs(t, exporter, logs)
		rec.WaitItems(1)
	})

	t.Run("publish with headers", func(t *testing.T) {
		done := make(chan struct{}, 1)
		server := newESTestServerBulkHandlerFunc(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t,
				fmt.Sprintf("OpenTelemetry Collector/latest (%s/%s)", runtime.GOOS, runtime.GOARCH),
				r.UserAgent(),
			)
			assert.Equal(t, "bah", r.Header.Get("Foo"))

			w.WriteHeader(http.StatusTeapot)
			select {
			case done <- struct{}{}:
			default:
			}
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Headers = map[string]configopaque.String{"foo": "bah"}
		})
		mustSendLogRecords(t, exporter, plog.NewLogRecord())
		<-done
	})

	t.Run("publish with configured user-agent header", func(t *testing.T) {
		done := make(chan struct{}, 1)
		server := newESTestServerBulkHandlerFunc(t, func(w http.ResponseWriter, r *http.Request) {
			// User the configured User-Agent header, rather than
			// the default one derived from BuildInfo.
			assert.Equal(t, "overridden", r.UserAgent())

			w.WriteHeader(http.StatusTeapot)
			select {
			case done <- struct{}{}:
			default:
			}
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Headers = map[string]configopaque.String{"User-Agent": "overridden"}
		})
		mustSendLogRecords(t, exporter, plog.NewLogRecord())
		<-done
	})

	t.Run("publish with dynamic index", func(t *testing.T) {

		rec := newBulkRecorder()
		var (
			prefix = "resprefix-"
			suffix = "-attrsuffix"
			index  = "someindex"
		)

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.NoError(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.NoError(t, err)

			create := jsonVal["create"].(map[string]any)
			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)
			assert.Equal(t, expected, create["_index"].(string))

			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.LogsIndex = index
			cfg.LogsDynamicIndex.Enabled = true
		})
		logs := newLogsWithAttributeAndResourceMap(
			map[string]string{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			map[string]string{
				indexPrefix: prefix,
			},
		)
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("hello world")
		mustSendLogs(t, exporter, logs)

		rec.WaitItems(1)
	})

	t.Run("publish with logstash index format enabled and dynamic index disabled", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.NoError(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.NoError(t, err)

			create := jsonVal["create"].(map[string]any)
			assert.Contains(t, create["_index"], "not-used-index")

			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.LogstashFormat.Enabled = true
			cfg.LogsIndex = "not-used-index"
		})
		mustSendLogs(t, exporter, newLogsWithAttributeAndResourceMap(nil, nil))

		rec.WaitItems(1)
	})

	t.Run("publish with logstash index format enabled and dynamic index enabled", func(t *testing.T) {
		var (
			prefix = "resprefix-"
			suffix = "-attrsuffix"
			index  = "someindex"
		)
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.NoError(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.NoError(t, err)

			create := jsonVal["create"].(map[string]any)
			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)

			assert.Equal(t, strings.Contains(create["_index"].(string), expected), true)

			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.LogsIndex = index
			cfg.LogsDynamicIndex.Enabled = true
			cfg.LogstashFormat.Enabled = true
		})
		mustSendLogs(t, exporter, newLogsWithAttributeAndResourceMap(
			map[string]string{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			map[string]string{
				indexPrefix: prefix,
			},
		))
		rec.WaitItems(1)
	})

	t.Run("retry http request", func(t *testing.T) {
		failures := 0
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			if failures == 0 {
				failures++
				return nil, &httpTestError{status: http.StatusTooManyRequests, message: "oops"}
			}

			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL)
		mustSendLogRecords(t, exporter, plog.NewLogRecord())

		rec.WaitItems(1)
	})

	t.Run("no retry", func(t *testing.T) {
		configurations := map[string]func(*Config){
			"max_requests limited": func(cfg *Config) {
				cfg.Retry.MaxRequests = 1
				cfg.Retry.InitialInterval = 1 * time.Millisecond
				cfg.Retry.MaxInterval = 10 * time.Millisecond
			},
			"retry.enabled is false": func(cfg *Config) {
				cfg.Retry.Enabled = false
				cfg.Retry.MaxRequests = 10
				cfg.Retry.InitialInterval = 1 * time.Millisecond
				cfg.Retry.MaxInterval = 10 * time.Millisecond
			},
		}

		handlers := map[string]func(attempts *atomic.Int64) bulkHandler{
			"fail http request": func(attempts *atomic.Int64) bulkHandler {
				return func([]itemRequest) ([]itemResponse, error) {
					attempts.Add(1)
					return nil, &httpTestError{message: "oops"}
				}
			},
			"fail item": func(attempts *atomic.Int64) bulkHandler {
				return func(docs []itemRequest) ([]itemResponse, error) {
					attempts.Add(1)
					return itemsReportStatus(docs, http.StatusTooManyRequests)
				}
			},
		}

		for name, handler := range handlers {
			handler := handler
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				for name, configurer := range configurations {
					configurer := configurer
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						attempts := &atomic.Int64{}
						server := newESTestServer(t, handler(attempts))

						exporter := newTestLogsExporter(t, server.URL, configurer)
						mustSendLogRecords(t, exporter, plog.NewLogRecord())

						time.Sleep(200 * time.Millisecond)
						assert.Equal(t, int64(1), attempts.Load())
					})
				}
			})
		}
	})

	t.Run("do not retry invalid request", func(t *testing.T) {
		attempts := &atomic.Int64{}
		server := newESTestServer(t, func(_ []itemRequest) ([]itemResponse, error) {
			attempts.Add(1)
			return nil, &httpTestError{message: "oops", status: http.StatusBadRequest}
		})

		exporter := newTestLogsExporter(t, server.URL)
		mustSendLogRecords(t, exporter, plog.NewLogRecord())

		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, int64(1), attempts.Load())
	})

	t.Run("retry single item", func(t *testing.T) {
		var attempts int
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			attempts++

			if attempts == 1 {
				return itemsReportStatus(docs, http.StatusTooManyRequests)
			}

			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL)
		mustSendLogRecords(t, exporter, plog.NewLogRecord())

		rec.WaitItems(1)
	})

	t.Run("do not retry bad item", func(t *testing.T) {
		attempts := &atomic.Int64{}
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			attempts.Add(1)
			return itemsReportStatus(docs, http.StatusBadRequest)
		})

		exporter := newTestLogsExporter(t, server.URL)
		mustSendLogRecords(t, exporter, plog.NewLogRecord())

		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, int64(1), attempts.Load())
	})

	t.Run("only retry failed items", func(t *testing.T) {
		var attempts [3]int
		var wg sync.WaitGroup
		wg.Add(1)

		const retryIdx = 1

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			resp := make([]itemResponse, len(docs))
			for i, doc := range docs {
				resp[i].Status = http.StatusOK

				var idxInfo struct {
					Attributes struct {
						Idx int
					}
				}
				if err := json.Unmarshal(doc.Document, &idxInfo); err != nil {
					panic(err)
				}

				if idxInfo.Attributes.Idx == retryIdx {
					if attempts[retryIdx] == 0 {
						resp[i].Status = http.StatusTooManyRequests
					} else {
						defer wg.Done()
					}
				}
				attempts[idxInfo.Attributes.Idx]++
			}
			return resp, nil
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Flush.Interval = 50 * time.Millisecond
			cfg.Retry.InitialInterval = 1 * time.Millisecond
			cfg.Retry.MaxInterval = 10 * time.Millisecond
		})
		for i := 0; i < 3; i++ {
			logRecord := plog.NewLogRecord()
			logRecord.Attributes().PutInt("idx", int64(i))
			mustSendLogRecords(t, exporter, logRecord)
		}

		wg.Wait() // <- this blocks forever if the event is not retried

		assert.Equal(t, [3]int{1, 2, 1}, attempts)
	})
}

func TestExporterMetrics(t *testing.T) {
	t.Run("publish with success", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL)
		dp := pmetric.NewNumberDataPoint()
		dp.SetDoubleValue(123.456)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		mustSendMetricSumDataPoints(t, exporter, dp)
		mustSendMetricGaugeDataPoints(t, exporter, dp)

		rec.WaitItems(2)
	})

}

func TestExporterTraces(t *testing.T) {
	t.Run("publish with success", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL)
		mustSendSpans(t, exporter, ptrace.NewSpan())
		mustSendSpans(t, exporter, ptrace.NewSpan())

		rec.WaitItems(2)
	})

	t.Run("publish with dynamic index", func(t *testing.T) {

		rec := newBulkRecorder()
		var (
			prefix = "resprefix-"
			suffix = "-attrsuffix"
			index  = "someindex"
		)

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.NoError(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.NoError(t, err)

			create := jsonVal["create"].(map[string]any)

			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)
			assert.Equal(t, expected, create["_index"].(string))

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.TracesIndex = index
			cfg.TracesDynamicIndex.Enabled = true
		})

		mustSendTraces(t, exporter, newTracesWithAttributeAndResourceMap(
			map[string]string{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			map[string]string{
				indexPrefix: prefix,
			},
		))

		rec.WaitItems(1)
	})

	t.Run("publish with logstash format index", func(t *testing.T) {
		var defaultCfg Config

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.NoError(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.NoError(t, err)

			create := jsonVal["create"].(map[string]any)

			assert.Equal(t, strings.Contains(create["_index"].(string), defaultCfg.TracesIndex), true)

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.LogstashFormat.Enabled = true
			cfg.TracesIndex = "not-used-index"
			defaultCfg = *cfg
		})

		mustSendTraces(t, exporter, newTracesWithAttributeAndResourceMap(nil, nil))

		rec.WaitItems(1)
	})

	t.Run("publish with logstash format index and dynamic index enabled", func(t *testing.T) {
		var (
			prefix = "resprefix-"
			suffix = "-attrsuffix"
			index  = "someindex"
		)

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.NoError(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.NoError(t, err)

			create := jsonVal["create"].(map[string]any)
			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)

			assert.Equal(t, strings.Contains(create["_index"].(string), expected), true)

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.TracesIndex = index
			cfg.TracesDynamicIndex.Enabled = true
			cfg.LogstashFormat.Enabled = true
		})

		mustSendTraces(t, exporter, newTracesWithAttributeAndResourceMap(
			map[string]string{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			map[string]string{
				indexPrefix: prefix,
			},
		))
		rec.WaitItems(1)
	})
}

// TestExporterAuth verifies that the Elasticsearch exporter supports
// confighttp.ClientConfig.Auth.
func TestExporterAuth(t *testing.T) {
	done := make(chan struct{}, 1)
	testauthID := component.NewID(component.MustNewType("authtest"))
	exporter := newUnstartedTestLogsExporter(t, "http://testing.invalid", func(cfg *Config) {
		cfg.Auth = &configauth.Authentication{AuthenticatorID: testauthID}
	})
	err := exporter.Start(context.Background(), &mockHost{
		extensions: map[component.ID]component.Component{
			testauthID: &authtest.MockClient{
				ResultRoundTripper: roundTripperFunc(func(*http.Request) (*http.Response, error) {
					select {
					case done <- struct{}{}:
					default:
					}
					return nil, errors.New("nope")
				}),
			},
		},
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exporter.Shutdown(context.Background()))
	}()

	mustSendLogRecords(t, exporter, plog.NewLogRecord())
	<-done
}

func newTestTracesExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Traces {
	f := NewFactory()
	cfg := withDefaultConfig(append([]func(*Config){func(cfg *Config) {
		cfg.Endpoints = []string{url}
		cfg.NumWorkers = 1
		cfg.Flush.Interval = 10 * time.Millisecond
	}}, fns...)...)
	exp, err := f.CreateTracesExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	require.NoError(t, err)

	err = exp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})
	return exp
}

func newTestMetricsExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Metrics {
	f := NewFactory()
	cfg := withDefaultConfig(append([]func(*Config){func(cfg *Config) {
		cfg.Endpoints = []string{url}
		cfg.NumWorkers = 1
		cfg.Flush.Interval = 10 * time.Millisecond
	}}, fns...)...)
	exp, err := f.CreateMetricsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	require.NoError(t, err)

	err = exp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})
	return exp
}

func newTestLogsExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Logs {
	exp := newUnstartedTestLogsExporter(t, url, fns...)
	err := exp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background()))
	})
	return exp
}

func newUnstartedTestLogsExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Logs {
	f := NewFactory()
	cfg := withDefaultConfig(append([]func(*Config){func(cfg *Config) {
		cfg.Endpoints = []string{url}
		cfg.NumWorkers = 1
		cfg.Flush.Interval = 10 * time.Millisecond
	}}, fns...)...)
	exp, err := f.CreateLogsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	require.NoError(t, err)
	return exp
}

func mustSendLogRecords(t *testing.T, exporter exporter.Logs, records ...plog.LogRecord) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	for _, record := range records {
		record.CopyTo(scopeLogs.LogRecords().AppendEmpty())
	}
	mustSendLogs(t, exporter, logs)
}

func mustSendLogs(t *testing.T, exporter exporter.Logs, logs plog.Logs) {
	err := exporter.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)
}

func mustSendMetricSumDataPoints(t *testing.T, exporter exporter.Metrics, dataPoints ...pmetric.NumberDataPoint) {
	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	for _, dataPoint := range dataPoints {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetEmptySum()
		metric.SetName("sum")
		dataPoint.CopyTo(metric.Sum().DataPoints().AppendEmpty())
	}
	mustSendMetrics(t, exporter, metrics)
}

func mustSendMetricGaugeDataPoints(t *testing.T, exporter exporter.Metrics, dataPoints ...pmetric.NumberDataPoint) {
	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	for _, dataPoint := range dataPoints {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetEmptyGauge()
		metric.SetName("gauge")
		dataPoint.CopyTo(metric.Gauge().DataPoints().AppendEmpty())
	}
	mustSendMetrics(t, exporter, metrics)
}

func mustSendMetrics(t *testing.T, exporter exporter.Metrics, metrics pmetric.Metrics) {
	err := exporter.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
}

func mustSendSpans(t *testing.T, exporter exporter.Traces, spans ...ptrace.Span) {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	for _, span := range spans {
		span.CopyTo(scopeSpans.Spans().AppendEmpty())
	}
	mustSendTraces(t, exporter, traces)
}

func mustSendTraces(t *testing.T, exporter exporter.Traces, traces ptrace.Traces) {
	err := exporter.ConsumeTraces(context.Background(), traces)
	require.NoError(t, err)
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (h *mockHost) GetFactory(kind component.Kind, typ component.Type) component.Factory {
	panic(fmt.Errorf("expected call to GetFactory(%v, %v)", kind, typ))
}

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func (h *mockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	panic(fmt.Errorf("expected call to GetExporters"))
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
