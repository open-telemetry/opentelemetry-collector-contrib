// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"runtime"
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

	t.Run("publish with dynamic index, prefix_suffix", func(t *testing.T) {

		rec := newBulkRecorder()
		var (
			prefix = "resprefix-"
			suffix = "-attrsuffix"
			index  = "someindex"
		)

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)
			assert.Equal(t, expected, actionJSONToIndex(t, docs[0].Action))

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

	t.Run("publish with dynamic index, data_stream", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			assert.Equal(t, "logs-record.dataset-resource.namespace", actionJSONToIndex(t, docs[0].Action))

			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.LogsDynamicIndex.Enabled = true
		})
		logs := newLogsWithAttributeAndResourceMap(
			map[string]string{
				dataStreamDataset: "record.dataset",
			},
			map[string]string{
				dataStreamDataset:   "resource.dataset",
				dataStreamNamespace: "resource.namespace",
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

			assert.Contains(t, actionJSONToIndex(t, docs[0].Action), "not-used-index")

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

			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)
			assert.Contains(t, actionJSONToIndex(t, docs[0].Action), expected)

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

	t.Run("publish with dynamic index, prefix_suffix", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			expected := "resource.prefix-metrics.index-resource.suffix"
			assert.Equal(t, expected, actionJSONToIndex(t, docs[0].Action))

			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.MetricsIndex = "metrics.index"
		})
		metrics := newMetricsWithAttributeAndResourceMap(
			map[string]string{
				indexSuffix: "-data.point.suffix",
			},
			map[string]string{
				indexPrefix: "resource.prefix-",
				indexSuffix: "-resource.suffix",
			},
		)
		metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("my.metric")
		metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetEmptySum().DataPoints().AppendEmpty().SetIntValue(0)
		mustSendMetrics(t, exporter, metrics)

		rec.WaitItems(1)
	})

	t.Run("publish with dynamic index, data_stream", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			expected := "metrics-resource.dataset-data.point.namespace"
			assert.Equal(t, expected, actionJSONToIndex(t, docs[0].Action))

			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.MetricsIndex = "metrics.index"
		})
		metrics := newMetricsWithAttributeAndResourceMap(
			map[string]string{
				dataStreamNamespace: "data.point.namespace",
			},
			map[string]string{
				dataStreamDataset:   "resource.dataset",
				dataStreamNamespace: "resource.namespace",
			},
		)
		metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("my.metric")
		mustSendMetrics(t, exporter, metrics)

		rec.WaitItems(1)
	})

	t.Run("publish with metrics grouping", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.MetricsIndex = "metrics.index"
			cfg.Mapping.Mode = "ecs"
		})

		addToMetricSlice := func(metricSlice pmetric.MetricSlice) {
			fooMetric := metricSlice.AppendEmpty()
			fooMetric.SetName("metric.foo")
			fooDps := fooMetric.SetEmptyGauge().DataPoints()
			fooDp := fooDps.AppendEmpty()
			fooDp.SetIntValue(1)
			fooOtherDp := fooDps.AppendEmpty()
			fillResourceAttributeMap(fooOtherDp.Attributes(), map[string]string{
				"dp.attribute": "dp.attribute.value",
			})
			fooOtherDp.SetDoubleValue(1.0)

			barMetric := metricSlice.AppendEmpty()
			barMetric.SetName("metric.bar")
			barDps := barMetric.SetEmptyGauge().DataPoints()
			barDp := barDps.AppendEmpty()
			barDp.SetDoubleValue(1.0)
			barOtherDp := barDps.AppendEmpty()
			fillResourceAttributeMap(barOtherDp.Attributes(), map[string]string{
				"dp.attribute": "dp.attribute.value",
			})
			barOtherDp.SetDoubleValue(1.0)
			barOtherIndexDp := barDps.AppendEmpty()
			fillResourceAttributeMap(barOtherIndexDp.Attributes(), map[string]string{
				"dp.attribute":      "dp.attribute.value",
				dataStreamNamespace: "bar",
			})
			barOtherIndexDp.SetDoubleValue(1.0)

			bazMetric := metricSlice.AppendEmpty()
			bazMetric.SetName("metric.baz")
			bazDps := bazMetric.SetEmptyGauge().DataPoints()
			bazDp := bazDps.AppendEmpty()
			bazDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
			bazDp.SetDoubleValue(1.0)
		}

		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		fillResourceAttributeMap(resourceMetrics.Resource().Attributes(), map[string]string{
			dataStreamNamespace: "resource.namespace",
		})
		scopeA := resourceMetrics.ScopeMetrics().AppendEmpty()
		addToMetricSlice(scopeA.Metrics())

		scopeB := resourceMetrics.ScopeMetrics().AppendEmpty()
		fillResourceAttributeMap(scopeB.Scope().Attributes(), map[string]string{
			dataStreamDataset: "scope.b",
		})
		addToMetricSlice(scopeB.Metrics())

		mustSendMetrics(t, exporter, metrics)

		rec.WaitItems(8)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-bar"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"bar","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"resource.namespace","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1,"foo":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"resource.namespace","type":"metrics"},"metric":{"bar":1,"foo":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"resource.namespace","type":"metrics"},"metric":{"baz":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-bar"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"bar","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"resource.namespace","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1,"foo":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"resource.namespace","type":"metrics"},"metric":{"bar":1,"foo":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"resource.namespace","type":"metrics"},"metric":{"baz":1}}`),
			},
		}

		assertItemsEqual(t, expected, rec.Items(), false)
	})

	t.Run("publish histogram", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "ecs"
		})

		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeA := resourceMetrics.ScopeMetrics().AppendEmpty()
		metricSlice := scopeA.Metrics()
		fooMetric := metricSlice.AppendEmpty()
		fooMetric.SetName("metric.foo")
		fooDps := fooMetric.SetEmptyHistogram().DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 3.0})
		fooDp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
		fooOtherDp := fooDps.AppendEmpty()
		fooOtherDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		fooOtherDp.ExplicitBounds().FromRaw([]float64{4.0, 5.0, 6.0})
		fooOtherDp.BucketCounts().FromRaw([]uint64{4, 5, 6, 7})

		mustSendMetrics(t, exporter, metrics)

		rec.WaitItems(2)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"counts":[1,2,3,4],"values":[0.5,1.5,2.5,3]}}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"counts":[4,5,6,7],"values":[2,4.5,5.5,6]}}}`),
			},
		}

		assertItemsEqual(t, expected, rec.Items(), false)
	})

	t.Run("publish only valid data points", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "ecs"
		})

		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeA := resourceMetrics.ScopeMetrics().AppendEmpty()
		metricSlice := scopeA.Metrics()
		fooMetric := metricSlice.AppendEmpty()
		fooMetric.SetName("metric.foo")
		fooDps := fooMetric.SetEmptyHistogram().DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 3.0})
		fooDp.BucketCounts().FromRaw([]uint64{})
		fooOtherDp := fooDps.AppendEmpty()
		fooOtherDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		fooOtherDp.ExplicitBounds().FromRaw([]float64{4.0, 5.0, 6.0})
		fooOtherDp.BucketCounts().FromRaw([]uint64{4, 5, 6, 7})
		barMetric := metricSlice.AppendEmpty()
		barMetric.SetName("metric.bar")
		barDps := barMetric.SetEmptySum().DataPoints()
		barDp := barDps.AppendEmpty()
		barDp.SetDoubleValue(math.Inf(1))
		barOtherDp := barDps.AppendEmpty()
		barOtherDp.SetDoubleValue(1.0)

		err := exporter.ConsumeMetrics(context.Background(), metrics)
		require.ErrorContains(t, err, "invalid histogram data point")
		require.ErrorContains(t, err, "invalid number data point")

		rec.WaitItems(2)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"bar":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"counts":[4,5,6,7],"values":[2,4.5,5.5,6]}}}`),
			},
		}

		assertItemsEqual(t, expected, rec.Items(), false)
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

	t.Run("publish with dynamic index, prefix_suffix", func(t *testing.T) {

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

	t.Run("publish with dynamic index, data_stream", func(t *testing.T) {

		rec := newBulkRecorder()

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			expected := "traces-span.dataset-default"
			assert.Equal(t, expected, actionJSONToIndex(t, docs[0].Action))

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.TracesDynamicIndex.Enabled = true
		})

		mustSendTraces(t, exporter, newTracesWithAttributeAndResourceMap(
			map[string]string{
				dataStreamDataset: "span.dataset",
			},
			map[string]string{
				dataStreamDataset: "resource.dataset",
			},
		))

		rec.WaitItems(1)
	})

	t.Run("publish with logstash format index", func(t *testing.T) {
		var defaultCfg Config

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			assert.Contains(t, actionJSONToIndex(t, docs[0].Action), defaultCfg.TracesIndex)

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

			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)
			assert.Contains(t, actionJSONToIndex(t, docs[0].Action), expected)

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

func TestExporterBatcher(t *testing.T) {
	var requests []*http.Request
	testauthID := component.NewID(component.MustNewType("authtest"))
	batcherEnabled := false // sync bulk indexer is used without batching
	exporter := newUnstartedTestLogsExporter(t, "http://testing.invalid", func(cfg *Config) {
		cfg.Batcher = BatcherConfig{Enabled: &batcherEnabled}
		cfg.Auth = &configauth.Authentication{AuthenticatorID: testauthID}
	})
	err := exporter.Start(context.Background(), &mockHost{
		extensions: map[component.ID]component.Component{
			testauthID: &authtest.MockClient{
				ResultRoundTripper: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					requests = append(requests, req)
					return nil, errors.New("nope")
				}),
			},
		},
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exporter.Shutdown(context.Background()))
	}()

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.LogRecords().AppendEmpty().Body().SetStr("log record body")

	type key struct{}
	_ = exporter.ConsumeLogs(context.WithValue(context.Background(), key{}, "value1"), logs)
	_ = exporter.ConsumeLogs(context.WithValue(context.Background(), key{}, "value2"), logs)
	require.Len(t, requests, 2) // flushed immediately by Consume

	assert.Equal(t, "value1", requests[0].Context().Value(key{}))
	assert.Equal(t, "value2", requests[1].Context().Value(key{}))
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

func actionJSONToIndex(t *testing.T, actionJSON json.RawMessage) string {
	action := struct {
		Create struct {
			Index string `json:"_index"`
		} `json:"create"`
	}{}
	err := json.Unmarshal(actionJSON, &action)
	require.NoError(t, err)
	return action.Create.Index
}
