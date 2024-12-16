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
	"github.com/tidwall/gjson"
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
		logs := newLogsWithAttributes(
			// record attrs
			map[string]any{
				"application":          "myapp",
				"service.name":         "myservice",
				"exception.stacktrace": "no no no no",
			},
			nil,
			// resource attrs
			map[string]any{
				"attrKey1": "abc",
				"attrKey2": "def",
			},
		)
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("hello world")
		mustSendLogs(t, exporter, logs)
		rec.WaitItems(1)
	})

	t.Run("publish with bodymap encoding", func(t *testing.T) {
		tableTests := []struct {
			name     string
			body     func() pcommon.Value
			expected string
		}{
			{
				name: "flat",
				body: func() pcommon.Value {
					body := pcommon.NewValueMap()
					m := body.Map()
					m.PutStr("@timestamp", "2024-03-12T20:00:41.123456789Z")
					m.PutInt("id", 1)
					m.PutStr("key", "value")
					return body
				},
				expected: `{"@timestamp":"2024-03-12T20:00:41.123456789Z","id":1,"key":"value"}`,
			},
			{
				name: "dotted key",
				body: func() pcommon.Value {
					body := pcommon.NewValueMap()
					m := body.Map()
					m.PutInt("a", 1)
					m.PutInt("a.b", 2)
					m.PutInt("a.b.c", 3)
					return body
				},
				expected: `{"a":1,"a.b":2,"a.b.c":3}`,
			},
			{
				name: "slice",
				body: func() pcommon.Value {
					body := pcommon.NewValueMap()
					m := body.Map()
					s := m.PutEmptySlice("a")
					for i := 0; i < 2; i++ {
						s.AppendEmpty().SetInt(int64(i))
					}
					return body
				},
				expected: `{"a":[0,1]}`,
			},
			{
				name: "inner map",
				body: func() pcommon.Value {
					body := pcommon.NewValueMap()
					m := body.Map()
					m1 := m.PutEmptyMap("a")
					m1.PutInt("b", 1)
					m1.PutInt("c", 2)
					return body
				},
				expected: `{"a":{"b":1,"c":2}}`,
			},
			{
				name: "nested map",
				body: func() pcommon.Value {
					body := pcommon.NewValueMap()
					m := body.Map()
					m1 := m.PutEmptyMap("a")
					m2 := m1.PutEmptyMap("b")
					m2.PutInt("c", 1)
					m2.PutInt("d", 2)
					return body
				},
				expected: `{"a":{"b":{"c":1,"d":2}}}`,
			},
		}

		for _, tt := range tableTests {
			t.Run(tt.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					assert.JSONEq(t, tt.expected, string(docs[0].Document))
					return itemsAllOK(docs)
				})

				exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
					cfg.Mapping.Mode = "bodymap"
				})
				logs := plog.NewLogs()
				resourceLogs := logs.ResourceLogs().AppendEmpty()
				scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
				logRecords := scopeLogs.LogRecords()
				logRecord := logRecords.AppendEmpty()
				tt.body().CopyTo(logRecord.Body())

				mustSendLogs(t, exporter, logs)
				rec.WaitItems(1)
			})
		}
	})

	t.Run("drops log records for bodymap mode if body is not a map", func(t *testing.T) {
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		logRecords := scopeLogs.LogRecords()

		// Invalid body type should be dropped.
		logRecords.AppendEmpty().Body().SetEmptySlice()

		// We should still process the valid records in the batch.
		bodyMap := logRecords.AppendEmpty().Body().SetEmptyMap()
		bodyMap.PutInt("a", 42)

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			defer rec.Record(docs)
			assert.Len(t, docs, 1)
			assert.JSONEq(t, `{"a":42}`, string(docs[0].Document))
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "bodymap"
		})

		err := exporter.ConsumeLogs(context.Background(), logs)
		assert.NoError(t, err)
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
		logs := newLogsWithAttributes(
			map[string]any{"attr.key": "value"},
			nil,
			nil,
		)
		mustSendLogs(t, exporter, logs)
		rec.WaitItems(1)
	})

	t.Run("publish with dedup", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			assert.JSONEq(t, `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Scope":{"name":"","value":"value","version":""},"SeverityNumber":0,"TraceFlags":0}`, string(docs[0].Document))
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "raw"
			// dedup is the default
		})
		logs := newLogsWithAttributes(
			// Scope collides with the top-level "Scope" field,
			// so will be removed during deduplication.
			map[string]any{"Scope": "value"},
			nil,
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
		logs := newLogsWithAttributes(
			map[string]any{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			nil,
			map[string]any{
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

			expected := "logs-record.dataset.____________-resource.namespace.-____________"
			assert.Equal(t, expected, actionJSONToIndex(t, docs[0].Action))

			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.LogsDynamicIndex.Enabled = true
		})
		logs := newLogsWithAttributes(
			map[string]any{
				dataStreamDataset: "record.dataset.\\/*?\"<>| ,#:",
			},
			nil,
			map[string]any{
				dataStreamDataset:   "resource.dataset",
				dataStreamNamespace: "resource.namespace.-\\/*?\"<>| ,#:",
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
		mustSendLogs(t, exporter, newLogsWithAttributes(nil, nil, nil))

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
		mustSendLogs(t, exporter, newLogsWithAttributes(
			map[string]any{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			nil,
			map[string]any{
				indexPrefix: prefix,
			},
		))
		rec.WaitItems(1)
	})

	t.Run("publish otel mapping mode", func(t *testing.T) {
		for _, tc := range []struct {
			body         pcommon.Value
			isEvent      bool
			wantDocument []byte
		}{
			{
				body: func() pcommon.Value {
					return pcommon.NewValueStr("foo")
				}(),
				wantDocument: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","attributes":{"attr.foo":"attr.foo.value"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"dropped_attributes_count":0,"observed_timestamp":"1970-01-01T00:00:00.000000000Z","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"},"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"severity_number":0,"body":{"text":"foo"}}`),
			},
			{
				body: func() pcommon.Value {
					vm := pcommon.NewValueMap()
					m := vm.SetEmptyMap()
					m.PutBool("true", true)
					m.PutBool("false", false)
					m.PutEmptyMap("inner").PutStr("foo", "bar")
					return vm
				}(),
				wantDocument: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","attributes":{"attr.foo":"attr.foo.value"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"dropped_attributes_count":0,"observed_timestamp":"1970-01-01T00:00:00.000000000Z","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"},"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"severity_number":0,"body":{"flattened":{"true":true,"false":false,"inner":{"foo":"bar"}}}}`),
			},
			{
				body: func() pcommon.Value {
					vm := pcommon.NewValueMap()
					m := vm.SetEmptyMap()
					m.PutBool("true", true)
					m.PutBool("false", false)
					m.PutEmptyMap("inner").PutStr("foo", "bar")
					return vm
				}(),
				isEvent:      true,
				wantDocument: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","attributes":{"attr.foo":"attr.foo.value","event.name":"foo"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"dropped_attributes_count":0,"observed_timestamp":"1970-01-01T00:00:00.000000000Z","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"},"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"severity_number":0,"body":{"structured":{"true":true,"false":false,"inner":{"foo":"bar"}}}}`),
			},
			{
				body: func() pcommon.Value {
					vs := pcommon.NewValueSlice()
					s := vs.Slice()
					s.AppendEmpty().SetStr("foo")
					s.AppendEmpty().SetBool(false)
					s.AppendEmpty().SetEmptyMap().PutStr("foo", "bar")
					return vs
				}(),
				wantDocument: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","attributes":{"attr.foo":"attr.foo.value"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"dropped_attributes_count":0,"observed_timestamp":"1970-01-01T00:00:00.000000000Z","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"},"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"severity_number":0,"body":{"flattened":{"value":["foo",false,{"foo":"bar"}]}}}`),
			},
			{
				body: func() pcommon.Value {
					vs := pcommon.NewValueSlice()
					s := vs.Slice()
					s.AppendEmpty().SetStr("foo")
					s.AppendEmpty().SetBool(false)
					s.AppendEmpty().SetEmptyMap().PutStr("foo", "bar")
					return vs
				}(),
				isEvent:      true,
				wantDocument: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","attributes":{"attr.foo":"attr.foo.value","event.name":"foo"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"dropped_attributes_count":0,"observed_timestamp":"1970-01-01T00:00:00.000000000Z","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"},"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"severity_number":0,"body":{"structured":{"value":["foo",false,{"foo":"bar"}]}}}`),
			},
		} {
			rec := newBulkRecorder()
			server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
				rec.Record(docs)
				return itemsAllOK(docs)
			})

			exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
				cfg.LogsDynamicIndex.Enabled = true
				cfg.Mapping.Mode = "otel"
			})
			recordAttrs := map[string]any{
				"data_stream.dataset": "attr.dataset",
				"attr.foo":            "attr.foo.value",
			}
			if tc.isEvent {
				recordAttrs["event.name"] = "foo"
			}
			logs := newLogsWithAttributes(
				recordAttrs,
				nil,
				map[string]any{
					"data_stream.dataset":   "resource.attribute.dataset",
					"data_stream.namespace": "resource.attribute.namespace",
					"resource.attr.foo":     "resource.attr.foo.value",
				},
			)
			tc.body.CopyTo(logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body())
			mustSendLogs(t, exporter, logs)

			expected := []itemRequest{
				{
					Action:   []byte(`{"create":{"_index":"logs-attr.dataset.otel-resource.attribute.namespace"}}`),
					Document: tc.wantDocument,
				},
			}

			assertRecordedItems(t, expected, rec, false)
		}
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
			"retry.enabled is false": func(cfg *Config) {
				cfg.Retry.Enabled = false
				cfg.Retry.RetryOnStatus = []int{429}
				cfg.Retry.MaxRetries = 10
				cfg.Retry.InitialInterval = 1 * time.Millisecond
				cfg.Retry.MaxInterval = 10 * time.Millisecond
			},
		}

		handlers := map[string]func(attempts *atomic.Int64) bulkHandler{
			"fail http request": func(attempts *atomic.Int64) bulkHandler {
				return func([]itemRequest) ([]itemResponse, error) {
					attempts.Add(1)
					return nil, &httpTestError{message: "oops", status: 429}
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
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				for name, configurer := range configurations {
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

	t.Run("otel mode attribute array value", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		mustSendLogs(t, exporter, newLogsWithAttributes(map[string]any{
			"some.record.attribute": []string{"foo", "bar"},
		}, map[string]any{
			"some.scope.attribute": []string{"foo", "bar"},
		}, map[string]any{
			"some.resource.attribute": []string{"foo", "bar"},
		}))

		rec.WaitItems(1)

		assert.Len(t, rec.Items(), 1)
		doc := rec.Items()[0].Document
		assert.JSONEq(t, `{"some.record.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `attributes`).Raw)
		assert.JSONEq(t, `{"some.scope.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `scope.attributes`).Raw)
		assert.JSONEq(t, `{"some.resource.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `resource.attributes`).Raw)
	})

	t.Run("otel mode attribute key prefix conflict", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		mustSendLogs(t, exporter, newLogsWithAttributes(map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}, map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}, map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}))

		rec.WaitItems(1)
		doc := rec.Items()[0].Document
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `attributes`).Raw)
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `scope.attributes`).Raw)
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `resource.attributes`).Raw)
	})
}

func TestExporterMetrics(t *testing.T) {
	t.Run("publish with success", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "ecs"
		})
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
			cfg.Mapping.Mode = "ecs"
		})
		metrics := newMetricsWithAttributes(
			map[string]any{
				indexSuffix: "-data.point.suffix",
			},
			nil,
			map[string]any{
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

			expected := "metrics-resource.dataset.____________-data.point.namespace.-____________"
			assert.Equal(t, expected, actionJSONToIndex(t, docs[0].Action))

			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.MetricsIndex = "metrics.index"
			cfg.Mapping.Mode = "ecs"
		})
		metrics := newMetricsWithAttributes(
			map[string]any{
				dataStreamNamespace: "data.point.namespace.-\\/*?\"<>| ,#:",
			},
			nil,
			map[string]any{
				dataStreamDataset:   "resource.dataset.\\/*?\"<>| ,#:",
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
			fillAttributeMap(fooOtherDp.Attributes(), map[string]any{
				"dp.attribute": "dp.attribute.value",
			})
			fooOtherDp.SetDoubleValue(1.0)

			barMetric := metricSlice.AppendEmpty()
			barMetric.SetName("metric.bar")
			barDps := barMetric.SetEmptyGauge().DataPoints()
			barDp := barDps.AppendEmpty()
			barDp.SetDoubleValue(1.0)
			barOtherDp := barDps.AppendEmpty()
			fillAttributeMap(barOtherDp.Attributes(), map[string]any{
				"dp.attribute": "dp.attribute.value",
			})
			barOtherDp.SetDoubleValue(1.0)
			barOtherIndexDp := barDps.AppendEmpty()
			fillAttributeMap(barOtherIndexDp.Attributes(), map[string]any{
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
		fillAttributeMap(resourceMetrics.Resource().Attributes(), map[string]any{
			dataStreamNamespace: "resource.namespace",
		})
		scopeA := resourceMetrics.ScopeMetrics().AppendEmpty()
		addToMetricSlice(scopeA.Metrics())

		scopeB := resourceMetrics.ScopeMetrics().AppendEmpty()
		fillAttributeMap(scopeB.Scope().Attributes(), map[string]any{
			dataStreamDataset: "scope.b",
		})
		addToMetricSlice(scopeB.Metrics())

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-bar"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"bar","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1.0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"resource.namespace","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1.0,"foo":1.0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"resource.namespace","type":"metrics"},"metric":{"bar":1.0,"foo":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"resource.namespace","type":"metrics"},"metric":{"baz":1.0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-bar"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"bar","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1.0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"resource.namespace","type":"metrics"},"dp":{"attribute":"dp.attribute.value"},"metric":{"bar":1.0,"foo":1.0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"resource.namespace","type":"metrics"},"metric":{"bar":1.0,"foo":1}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-scope.b-resource.namespace"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"scope.b","namespace":"resource.namespace","type":"metrics"},"metric":{"baz":1.0}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
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
		fooHistogram := fooMetric.SetEmptyHistogram()
		fooHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		fooDps := fooHistogram.DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 3.0})
		fooDp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
		fooOtherDp := fooDps.AppendEmpty()
		fooOtherDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		fooOtherDp.ExplicitBounds().FromRaw([]float64{4.0, 5.0, 6.0})
		fooOtherDp.BucketCounts().FromRaw([]uint64{4, 5, 6, 7})

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"counts":[1,2,3,4],"values":[0.5,1.5,2.5,3.0]}}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"counts":[4,5,6,7],"values":[2.0,4.5,5.5,6.0]}}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("publish exponential histogram", func(t *testing.T) {
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
		fooHistogram := fooMetric.SetEmptyExponentialHistogram()
		fooHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		fooDps := fooHistogram.DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.SetZeroCount(2)
		fooDp.Positive().SetOffset(1)
		fooDp.Positive().BucketCounts().FromRaw([]uint64{0, 1, 1, 0})

		fooDp.Negative().SetOffset(1)
		fooDp.Negative().BucketCounts().FromRaw([]uint64{1, 0, 0, 1})

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"counts":[1,1,2,1,1],"values":[-24.0,-3.0,0.0,6.0,12.0]}}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("publish histogram cumulative temporality", func(t *testing.T) {
		server := newESTestServer(t, func(_ []itemRequest) ([]itemResponse, error) {
			require.Fail(t, "unexpected request")
			return nil, nil
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
		fooHistogram := fooMetric.SetEmptyHistogram()
		fooHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		fooDps := fooHistogram.DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 3.0})
		fooDp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})

		err := exporter.ConsumeMetrics(context.Background(), metrics)
		assert.NoError(t, err)
	})

	t.Run("publish exponential histogram cumulative temporality", func(t *testing.T) {
		server := newESTestServer(t, func(_ []itemRequest) ([]itemResponse, error) {
			require.Fail(t, "unexpected request")
			return nil, nil
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
		fooHistogram := fooMetric.SetEmptyExponentialHistogram()
		fooHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		fooDps := fooHistogram.DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.SetZeroCount(2)
		fooDp.Positive().SetOffset(1)
		fooDp.Positive().BucketCounts().FromRaw([]uint64{0, 1, 1, 0})

		fooDp.Negative().SetOffset(1)
		fooDp.Negative().BucketCounts().FromRaw([]uint64{1, 0, 0, 1})

		err := exporter.ConsumeMetrics(context.Background(), metrics)
		assert.NoError(t, err)
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
		fooHistogram := fooMetric.SetEmptyHistogram()
		fooHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		fooDps := fooHistogram.DataPoints()
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
		assert.NoError(t, err)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"bar":1.0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"counts":[4,5,6,7],"values":[2.0,4.5,5.5,6.0]}}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeA := resourceMetrics.ScopeMetrics().AppendEmpty()
		metricSlice := scopeA.Metrics()
		fooMetric := metricSlice.AppendEmpty()
		fooMetric.SetName("metric.foo")
		fooHistogram := fooMetric.SetEmptyHistogram()
		fooHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		fooDps := fooHistogram.DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 3.0})
		fooDp.BucketCounts().FromRaw([]uint64{1, 2, 3, 4})
		fooOtherDp := fooDps.AppendEmpty()
		fooOtherDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		fooOtherDp.ExplicitBounds().FromRaw([]float64{4.0, 5.0, 6.0})
		fooOtherDp.BucketCounts().FromRaw([]uint64{4, 5, 6, 7})

		sumMetric := metricSlice.AppendEmpty()
		sumMetric.SetName("metric.sum")
		sumDps := sumMetric.SetEmptySum().DataPoints()
		sumDp := sumDps.AppendEmpty()
		sumDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		sumDp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(7200, 0)))
		sumDp.SetDoubleValue(1.5)

		summaryMetric := metricSlice.AppendEmpty()
		summaryMetric.SetName("metric.summary")
		summaryDps := summaryMetric.SetEmptySummary().DataPoints()
		summaryDp := summaryDps.AppendEmpty()
		summaryDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3*3600, 0)))
		summaryDp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(3*3600, 0)))
		summaryDp.SetCount(1)
		summaryDp.SetSum(1.5)

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.metric.foo":"histogram"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"metric.foo":{"counts":[1,2,3,4],"values":[0.5,1.5,2.5,3.0]}},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.metric.foo":"histogram"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"metric.foo":{"counts":[4,5,6,7],"values":[2.0,4.5,5.5,6.0]}},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.metric.sum":"gauge_double"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"metric.sum":1.5},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"start_timestamp":"1970-01-01T02:00:00.000000000Z"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.metric.summary":"summary"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T03:00:00.000000000Z","data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"metric.summary":{"sum":1.5,"value_count":1}},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"start_timestamp":"1970-01-01T03:00:00.000000000Z"}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode attribute array value", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		mustSendMetrics(t, exporter, newMetricsWithAttributes(map[string]any{
			"some.record.attribute": []string{"foo", "bar"},
		}, map[string]any{
			"some.scope.attribute": []string{"foo", "bar"},
		}, map[string]any{
			"some.resource.attribute": []string{"foo", "bar"},
		}))

		rec.WaitItems(1)

		assert.Len(t, rec.Items(), 1)
		doc := rec.Items()[0].Document
		assert.JSONEq(t, `{"some.record.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `attributes`).Raw)
		assert.JSONEq(t, `{"some.scope.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `scope.attributes`).Raw)
		assert.JSONEq(t, `{"some.resource.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `resource.attributes`).Raw)
	})

	t.Run("otel mode _doc_count hint", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		metrics := pmetric.NewMetrics()
		resourceMetric := metrics.ResourceMetrics().AppendEmpty()
		scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()

		sumMetric := scopeMetric.Metrics().AppendEmpty()
		sumMetric.SetName("sum")
		sumDP := sumMetric.SetEmptySum().DataPoints().AppendEmpty()
		sumDP.SetIntValue(0)

		summaryMetric := scopeMetric.Metrics().AppendEmpty()
		summaryMetric.SetName("summary")
		summaryDP := summaryMetric.SetEmptySummary().DataPoints().AppendEmpty()
		summaryDP.SetSum(1)
		summaryDP.SetCount(10)
		fillAttributeMap(summaryDP.Attributes(), map[string]any{
			"elasticsearch.mapping.hints": []string{"_doc_count"},
		})

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.sum":"gauge_long","metrics.summary":"summary"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","_doc_count":10,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"sum":0,"summary":{"sum":1.0,"value_count":10}},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode aggregate_metric_double hint", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		metrics := pmetric.NewMetrics()
		resourceMetric := metrics.ResourceMetrics().AppendEmpty()
		scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()

		histogramMetric := scopeMetric.Metrics().AppendEmpty()
		histogramMetric.SetName("histogram.summary")
		fooHistogram := histogramMetric.SetEmptyHistogram()
		fooHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		fooDp := fooHistogram.DataPoints().AppendEmpty()
		fooDp.SetSum(1)
		fooDp.SetCount(10)
		fillAttributeMap(fooDp.Attributes(), map[string]any{
			"elasticsearch.mapping.hints": []string{"_doc_count", "aggregate_metric_double"},
		})

		exphistogramMetric := scopeMetric.Metrics().AppendEmpty()
		exphistogramMetric.SetName("exphistogram.summary")
		fooExpHistogram := exphistogramMetric.SetEmptyExponentialHistogram()
		fooExpHistogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		fooExpDp := fooExpHistogram.DataPoints().AppendEmpty()
		fooExpDp.SetTimestamp(pcommon.Timestamp(time.Hour))
		fooExpDp.SetSum(1)
		fooExpDp.SetCount(10)
		fillAttributeMap(fooExpDp.Attributes(), map[string]any{
			"elasticsearch.mapping.hints": []string{"_doc_count", "aggregate_metric_double"},
		})

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.histogram.summary":"summary"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","_doc_count":10,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"histogram.summary":{"sum":1.0,"value_count":10}},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.exphistogram.summary":"summary"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","_doc_count":10,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"exphistogram.summary":{"sum":1.0,"value_count":10}},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode metric name conflict", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		metrics := pmetric.NewMetrics()
		resourceMetric := metrics.ResourceMetrics().AppendEmpty()
		scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()

		fooBarMetric := scopeMetric.Metrics().AppendEmpty()
		fooBarMetric.SetName("foo.bar")
		fooBarMetric.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(0)

		fooMetric := scopeMetric.Metrics().AppendEmpty()
		fooMetric.SetName("foo")
		fooMetric.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(0)

		fooBarBazMetric := scopeMetric.Metrics().AppendEmpty()
		fooBarBazMetric.SetName("foo.bar.baz")
		fooBarBazMetric.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(0)

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.foo.bar":"gauge_long","metrics.foo":"gauge_long","metrics.foo.bar.baz":"gauge_long"}}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"foo":0,"foo.bar":0,"foo.bar.baz":0},"resource":{"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode attribute key prefix conflict", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		mustSendMetrics(t, exporter, newMetricsWithAttributes(map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}, map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}, map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}))

		rec.WaitItems(1)
		doc := rec.Items()[0].Document
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `attributes`).Raw)
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `scope.attributes`).Raw)
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `resource.attributes`).Raw)
	})

	t.Run("publish summary", func(t *testing.T) {
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
		fooDps := fooMetric.SetEmptySummary().DataPoints()
		fooDp := fooDps.AppendEmpty()
		fooDp.SetSum(1.5)
		fooDp.SetCount(1)
		fooOtherDp := fooDps.AppendEmpty()
		fooOtherDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		fooOtherDp.SetSum(2)
		fooOtherDp.SetCount(3)

		mustSendMetrics(t, exporter, metrics)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"sum":1.5,"value_count":1}}}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"metric":{"foo":{"sum":2.0,"value_count":3}}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
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

		mustSendTraces(t, exporter, newTracesWithAttributes(
			map[string]any{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			nil,
			map[string]any{
				indexPrefix: prefix,
			},
		))

		rec.WaitItems(1)
	})

	t.Run("publish with dynamic index, data_stream", func(t *testing.T) {
		rec := newBulkRecorder()

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			expected := "traces-span.dataset.____________-default"
			assert.Equal(t, expected, actionJSONToIndex(t, docs[0].Action))

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.TracesDynamicIndex.Enabled = true
		})

		mustSendTraces(t, exporter, newTracesWithAttributes(
			map[string]any{
				dataStreamDataset: "span.dataset.\\/*?\"<>| ,#:",
			},
			nil,
			map[string]any{
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

		mustSendTraces(t, exporter, newTracesWithAttributes(nil, nil, nil))

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

		mustSendTraces(t, exporter, newTracesWithAttributes(
			map[string]any{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			nil,
			map[string]any{
				indexPrefix: prefix,
			},
		))
		rec.WaitItems(1)
	})

	t.Run("otel mode", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.TracesDynamicIndex.Enabled = true
			cfg.Mapping.Mode = "otel"
		})

		traces := ptrace.NewTraces()
		resourceSpans := traces.ResourceSpans()
		rs := resourceSpans.AppendEmpty()

		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("name")
		span.SetTraceID(pcommon.NewTraceIDEmpty())
		span.SetSpanID(pcommon.NewSpanIDEmpty())
		span.SetFlags(1)
		span.SetDroppedAttributesCount(2)
		span.SetDroppedEventsCount(3)
		span.SetDroppedLinksCount(4)
		span.TraceState().FromRaw("foo")
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(7200, 0)))

		event := span.Events().AppendEmpty()
		event.SetName("exception")
		event.Attributes().PutStr("event.attr.foo", "event.attr.bar")
		event.SetDroppedAttributesCount(1)

		scopeAttr := span.Attributes()
		fillAttributeMap(scopeAttr, map[string]any{
			"attr.foo": "attr.bar",
		})

		resAttr := rs.Resource().Attributes()
		fillAttributeMap(resAttr, map[string]any{
			"resource.foo": "resource.bar",
		})

		spanLink := span.Links().AppendEmpty()
		spanLink.SetTraceID(pcommon.NewTraceIDEmpty())
		spanLink.SetSpanID(pcommon.NewSpanIDEmpty())
		spanLink.SetFlags(10)
		spanLink.SetDroppedAttributesCount(11)
		spanLink.TraceState().FromRaw("bar")
		fillAttributeMap(spanLink.Attributes(), map[string]any{
			"link.attr.foo": "link.attr.bar",
		})

		mustSendTraces(t, exporter, traces)

		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"traces-generic.otel-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T01:00:00.000000000Z","attributes":{"attr.foo":"attr.bar"},"data_stream":{"dataset":"generic.otel","namespace":"default","type":"traces"},"dropped_attributes_count":2,"dropped_events_count":3,"dropped_links_count":4,"duration":3600000000000,"kind":"Unspecified","links":[{"attributes":{"link.attr.foo":"link.attr.bar"},"dropped_attributes_count":11,"span_id":"","trace_id":"","trace_state":"bar"}],"name":"name","resource":{"attributes":{"resource.foo":"resource.bar"},"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0},"status":{"code":"Unset"},"trace_state":"foo"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"logs-generic.otel-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","attributes":{"event.attr.foo":"event.attr.bar","event.name":"exception"},"data_stream":{"dataset":"generic.otel","namespace":"default","type":"logs"},"dropped_attributes_count":1,"resource":{"attributes":{"resource.foo":"resource.bar"},"dropped_attributes_count":0},"scope":{"dropped_attributes_count":0}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode attribute array value", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		traces := newTracesWithAttributes(map[string]any{
			"some.record.attribute": []string{"foo", "bar"},
		}, map[string]any{
			"some.scope.attribute": []string{"foo", "bar"},
		}, map[string]any{
			"some.resource.attribute": []string{"foo", "bar"},
		})
		spanEventAttrs := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().AppendEmpty().Attributes()
		fillAttributeMap(spanEventAttrs, map[string]any{
			"some.record.attribute": []string{"foo", "bar"},
		})
		mustSendTraces(t, exporter, traces)

		rec.WaitItems(2)

		assert.Len(t, rec.Items(), 2)
		for _, item := range rec.Items() {
			doc := item.Document
			assert.JSONEq(t, `{"some.record.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `attributes`).Raw)
			assert.JSONEq(t, `{"some.scope.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `scope.attributes`).Raw)
			assert.JSONEq(t, `{"some.resource.attribute":["foo","bar"]}`, gjson.GetBytes(doc, `resource.attributes`).Raw)
		}
	})

	t.Run("otel mode attribute key prefix conflict", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		mustSendTraces(t, exporter, newTracesWithAttributes(map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}, map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}, map[string]any{
			"a":   "a",
			"a.b": "a.b",
		}))

		rec.WaitItems(1)
		doc := rec.Items()[0].Document
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `attributes`).Raw)
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `scope.attributes`).Raw)
		assert.JSONEq(t, `{"a":"a","a.b":"a.b"}`, gjson.GetBytes(doc, `resource.attributes`).Raw)
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
	exp, err := f.CreateTraces(context.Background(), exportertest.NewNopSettings(), cfg)
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
	exp, err := f.CreateMetrics(context.Background(), exportertest.NewNopSettings(), cfg)
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
	exp, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(), cfg)
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

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
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
