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
	"net/url"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-docappender/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
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

			expected := `{"@timestamp":"1970-01-01T00:00:00.000000000Z","application":"myapp","attrKey1":"abc","attrKey2":"def","data_stream":{"dataset":"generic","namespace":"default","type":"logs"},"error":{"stacktrace":"no no no no"},"message":"hello world","service":{"name":"myservice"}}`
			actual := string(docs[0].Document)
			assert.JSONEq(t, expected, actual)

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
					for i := range 2 {
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

		err := exporter.ConsumeLogs(t.Context(), logs)
		assert.NoError(t, err)
		rec.WaitItems(1)
	})

	t.Run("publish with dedot", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			assert.JSONEq(t,
				`{"attr":{"key":"value"},"@timestamp":"1970-01-01T00:00:00.000000000Z"}`,
				string(docs[0].Document),
			)
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "ecs"
			cfg.LogsIndex = "index"
			// deduplication is always performed except in otel mapping mode -
			// there is no other configuration that controls it
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
			assert.JSONEq(t, `{"@timestamp":"1970-01-01T00:00:00.000000000Z","Scope.name":"","Scope.value":"value","Scope.version":"","SeverityNumber":0,"TraceFlags":0}`, string(docs[0].Document))
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "raw"
			cfg.LogsIndex = "index"
			// deduplication is always performed - there is no configuration that controls it
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
			cfg.Headers = configopaque.MapList{
				{Name: "foo", Value: "bah"},
			}
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
			cfg.Headers = configopaque.MapList{
				{Name: "User-Agent", Value: "overridden"},
			}
		})
		mustSendLogRecords(t, exporter, plog.NewLogRecord())
		<-done
	})

	t.Run("publish with elasticsearch.index", func(t *testing.T) {
		rec := newBulkRecorder()
		index := "someindex"

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})
		logs := newLogsWithAttributes(
			map[string]any{
				"elasticsearch.index": index,
			},
			map[string]any{
				"elasticsearch.index": "ignored",
			},
			map[string]any{
				"elasticsearch.index": "ignored",
			},
		)
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("hello world")
		mustSendLogs(t, exporter, logs)

		docs := rec.WaitItems(1)
		doc := docs[0]
		assert.Equal(t, index, actionJSONToIndex(t, doc.Action))
		assert.JSONEq(t, `{}`, gjson.GetBytes(doc.Document, `attributes`).Raw)
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
			cfg.Mapping.Mode = "none"
		})
		logs := newLogsWithAttributes(
			map[string]any{
				elasticsearch.DataStreamDataset: "record.dataset.\\/*?\"<>| ,#:",
			},
			nil,
			map[string]any{
				elasticsearch.DataStreamDataset:   "resource.dataset",
				elasticsearch.DataStreamNamespace: "resource.namespace.-\\/*?\"<>| ,#:",
			},
		)
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("hello world")
		mustSendLogs(t, exporter, logs)

		rec.WaitItems(1)
	})

	t.Run("publish with logstash index format enabled", func(t *testing.T) {
		index := "someindex"
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			assert.Contains(t, actionJSONToIndex(t, docs[0].Action), index)

			return itemsAllOK(docs)
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.LogstashFormat.Enabled = true
		})
		mustSendLogs(t, exporter, newLogsWithAttributes(
			map[string]any{
				"elasticsearch.index": index,
			},
			nil,
			nil,
		))
		rec.WaitItems(1)
	})

	t.Run("publish otel mapping mode", func(t *testing.T) {
		for _, tc := range []struct {
			name         string
			body         pcommon.Value
			isEvent      bool
			wantDocument []byte
		}{
			{
				name: "basic",
				body: func() pcommon.Value {
					return pcommon.NewValueStr("foo")
				}(),
				wantDocument: []byte(`{"@timestamp":"0.0","attributes":{"attr.foo":"attr.foo.value"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"observed_timestamp":"0.0","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"}},"scope":{},"body":{"text":"foo"}}`),
			},
			{
				name: "map body",
				body: func() pcommon.Value {
					vm := pcommon.NewValueMap()
					m := vm.SetEmptyMap()
					m.PutBool("true", true)
					m.PutBool("false", false)
					m.PutEmptyMap("inner").PutStr("foo", "bar")
					return vm
				}(),
				wantDocument: []byte(`{"@timestamp":"0.0","attributes":{"attr.foo":"attr.foo.value"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"observed_timestamp":"0.0","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"}},"scope":{},"body":{"structured":{"true":true,"false":false,"inner":{"foo":"bar"}}}}`),
			},
			{
				name: "map body event",
				body: func() pcommon.Value {
					vm := pcommon.NewValueMap()
					m := vm.SetEmptyMap()
					m.PutBool("true", true)
					m.PutBool("false", false)
					m.PutEmptyMap("inner").PutStr("foo", "bar")
					return vm
				}(),
				isEvent:      true,
				wantDocument: []byte(`{"@timestamp":"0.0","attributes":{"attr.foo":"attr.foo.value","event.name":"foo"},"event_name":"foo","data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"observed_timestamp":"0.0","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"}},"scope":{},"body":{"structured":{"true":true,"false":false,"inner":{"foo":"bar"}}}}`),
			},
			{
				name: "heterogeneous slice body",
				body: func() pcommon.Value {
					vs := pcommon.NewValueSlice()
					s := vs.Slice()
					s.AppendEmpty().SetStr("foo")
					s.AppendEmpty().SetBool(false)
					s.AppendEmpty().SetEmptyMap().PutStr("foo", "bar")
					return vs
				}(),
				wantDocument: []byte(`{"@timestamp":"0.0","attributes":{"attr.foo":"attr.foo.value"},"data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"observed_timestamp":"0.0","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"}},"scope":{},"body":{"structured":{"value":["foo",false,{"foo":"bar"}]}}}`),
			},
			{
				name: "heterogeneous slice body event",
				body: func() pcommon.Value {
					vs := pcommon.NewValueSlice()
					s := vs.Slice()
					s.AppendEmpty().SetStr("foo")
					s.AppendEmpty().SetBool(false)
					s.AppendEmpty().SetEmptyMap().PutStr("foo", "bar")
					return vs
				}(),
				isEvent:      true,
				wantDocument: []byte(`{"@timestamp":"0.0","attributes":{"attr.foo":"attr.foo.value","event.name":"foo"},"event_name":"foo","data_stream":{"dataset":"attr.dataset.otel","namespace":"resource.attribute.namespace","type":"logs"},"observed_timestamp":"0.0","resource":{"attributes":{"resource.attr.foo":"resource.attr.foo.value"}},"scope":{},"body":{"structured":{"value":["foo",false,{"foo":"bar"}]}}}`),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
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
			})
		}
	})

	t.Run("otel mode attribute", func(t *testing.T) {
		for _, tc := range []struct {
			name string

			recordAttrs   map[string]any
			scopeAttrs    map[string]any
			resourceAttrs map[string]any

			wantRecordAttrs   string
			wantScopeAttrs    string
			wantResourceAttrs string
		}{
			{
				name: "slice value",
				recordAttrs: map[string]any{
					"some.record.attribute": []string{"foo", "bar"},
				},
				scopeAttrs: map[string]any{
					"some.scope.attribute": []string{"foo", "bar"},
				},
				resourceAttrs: map[string]any{
					"some.resource.attribute": []string{"foo", "bar"},
				},
				wantRecordAttrs:   `{"some.record.attribute":["foo","bar"]}`,
				wantScopeAttrs:    `{"some.scope.attribute":["foo","bar"]}`,
				wantResourceAttrs: `{"some.resource.attribute":["foo","bar"]}`,
			},
			{
				// only valid for log attributes https://opentelemetry.io/docs/specs/otel/logs/data-model/#field-attributes
				// but testing for all attributes anyway
				name: "map value",
				recordAttrs: map[string]any{
					"a.b": map[string]any{
						"c": "a.b.c",
						"c.d": map[string]any{
							"e": "a.b.c.d.e",
						},
					},
				},
				scopeAttrs: map[string]any{
					"a.b": map[string]any{
						"c": "a.b.c",
						"c.d": map[string]any{
							"e": "a.b.c.d.e",
						},
					},
				},
				resourceAttrs: map[string]any{
					"a.b": map[string]any{
						"c": "a.b.c",
						"c.d": map[string]any{
							"e": "a.b.c.d.e",
						},
					},
				},
				wantRecordAttrs:   `{"a.b":{"c":"a.b.c","c.d":{"e":"a.b.c.d.e"}}}`,
				wantScopeAttrs:    `{"a.b":{"c":"a.b.c","c.d":{"e":"a.b.c.d.e"}}}`,
				wantResourceAttrs: `{"a.b":{"c":"a.b.c","c.d":{"e":"a.b.c.d.e"}}}`,
			},
			{
				name: "key prefix conflict",
				recordAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				scopeAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				resourceAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				wantRecordAttrs:   `{"a":"a","a.b":"a.b"}`,
				wantScopeAttrs:    `{"a":"a","a.b":"a.b"}`,
				wantResourceAttrs: `{"a":"a","a.b":"a.b"}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
					cfg.Mapping.Mode = "otel"
				})

				mustSendLogs(t, exporter, newLogsWithAttributes(tc.recordAttrs, tc.scopeAttrs, tc.resourceAttrs))

				rec.WaitItems(1)

				assert.Len(t, rec.Items(), 1)
				doc := rec.Items()[0].Document
				assert.JSONEq(t, tc.wantRecordAttrs, gjson.GetBytes(doc, `attributes`).Raw)
				assert.JSONEq(t, tc.wantScopeAttrs, gjson.GetBytes(doc, `scope.attributes`).Raw)
				assert.JSONEq(t, tc.wantResourceAttrs, gjson.GetBytes(doc, `resource.attributes`).Raw)
			})
		}
	})

	t.Run("retry http request batcher", func(t *testing.T) {
		for _, maxRetries := range []int{0, 1, 11} {
			t.Run(fmt.Sprintf("max retries %d", maxRetries), func(t *testing.T) {
				t.Parallel()
				expectedRetries := maxRetries
				if maxRetries == 0 {
					expectedRetries = defaultMaxRetries
				}

				var attempts atomic.Int64
				rec := newBulkRecorder()
				server := newESTestServer(t, func(_ []itemRequest) ([]itemResponse, error) {
					// always return error, and assert that the number of attempts is expected, not more, not less.
					attempts.Add(1)
					return nil, &httpTestError{status: http.StatusServiceUnavailable, message: "oops"}
				})

				exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
					cfg.Retry.Enabled = true
					cfg.Retry.RetryOnStatus = []int{http.StatusServiceUnavailable}
					cfg.Retry.MaxRetries = maxRetries
					cfg.Retry.InitialInterval = 1 * time.Millisecond
					cfg.Retry.MaxInterval = 5 * time.Millisecond

					// use sync flushing
					cfg.QueueBatchConfig.Get().WaitForResult = true
				})

				logs := plog.NewLogs()
				resourceLogs := logs.ResourceLogs().AppendEmpty()
				scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
				scopeLogs.LogRecords().AppendEmpty()
				logs.MarkReadOnly()
				err := exporter.ConsumeLogs(t.Context(), logs) // as sync bulk indexer is used, retries are finished on return
				var errFlushFailed docappender.ErrorFlushFailed
				require.ErrorAs(t, err, &errFlushFailed)

				assert.Equal(t, 0, rec.countItems())
				assert.Equal(t, int64(expectedRetries+1), attempts.Load()) // initial request + retries
			})
		}
	})

	t.Run("retry http request", func(t *testing.T) {
		for _, maxRetries := range []int{0, 1, 11} {
			t.Run(fmt.Sprintf("max retries %d", maxRetries), func(t *testing.T) {
				t.Parallel()
				expectedRetries := maxRetries
				if maxRetries == 0 {
					expectedRetries = defaultMaxRetries
				}

				var attempts atomic.Int64
				rec := newBulkRecorder()
				server := newESTestServer(t, func(_ []itemRequest) ([]itemResponse, error) {
					// always return error, and assert that the number of attempts is expected, not more, not less.
					attempts.Add(1)
					return nil, &httpTestError{status: http.StatusServiceUnavailable, message: "oops"}
				})

				exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
					cfg.Retry.Enabled = true
					cfg.Retry.RetryOnStatus = []int{http.StatusServiceUnavailable}
					cfg.Retry.MaxRetries = maxRetries
					cfg.Retry.InitialInterval = 1 * time.Millisecond
					cfg.Retry.MaxInterval = 5 * time.Millisecond

					// use async indexer
					cfg.QueueBatchConfig.Get().WaitForResult = false
					cfg.QueueBatchConfig.Get().BlockOnOverflow = false
				})
				mustSendLogRecords(t, exporter, plog.NewLogRecord()) // as sync bulk indexer is used, retries are not guaranteed to finish

				assert.Eventually(t, func() bool {
					return int64(expectedRetries+1) == attempts.Load()
				}, time.Second, 5*time.Millisecond)

				// assert that it does not retry in async more than expected
				time.Sleep(20 * time.Millisecond)
				assert.Equal(t, int64(expectedRetries+1), attempts.Load())
				assert.Equal(t, 0, rec.countItems())
			})
		}
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

		handlers := map[string]func(*atomic.Int64) bulkHandler{
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

						assert.Eventually(t, func() bool {
							return attempts.Load() == 1
						}, 5*time.Second, 20*time.Millisecond)
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
		var attempts [3]atomic.Int64
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

				newAttemptCount := attempts[idxInfo.Attributes.Idx].Add(1)
				if idxInfo.Attributes.Idx == retryIdx {
					if newAttemptCount == 1 {
						resp[i].Status = http.StatusTooManyRequests
					} else {
						defer wg.Done()
					}
				}
			}
			return resp, nil
		})

		exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
			cfg.Retry.InitialInterval = 1 * time.Millisecond
			cfg.Retry.MaxInterval = 10 * time.Millisecond
		})
		for i := range 3 {
			logRecord := plog.NewLogRecord()
			logRecord.Attributes().PutInt("idx", int64(i))
			mustSendLogRecords(t, exporter, logRecord)
		}

		wg.Wait() // <- this blocks forever if the event is not retried

		expected := [3]int64{1, 2, 1}
		for i := range attempts {
			assert.Equal(t, expected[i], attempts[i].Load(), "attempt count mismatch for index %d", i)
		}
	})

	t.Run("publish logs with dynamic id", func(t *testing.T) {
		t.Parallel()
		exampleDocID := "example-doc-id-123"
		tableTests := []struct {
			name          string
			expectedDocID string // "" means the document ID will not be set
			recordAttrs   map[string]any
		}{
			{
				name:          "missing document id attribute should not set _id",
				expectedDocID: "",
			},
			{
				name:          "empty document id attribute should not set _id",
				expectedDocID: "",
				recordAttrs: map[string]any{
					elasticsearch.DocumentIDAttributeName: "",
				},
			},
			{
				name:          "valid document id attribute should set _id",
				expectedDocID: exampleDocID,
				recordAttrs: map[string]any{
					elasticsearch.DocumentIDAttributeName: exampleDocID,
				},
			},
		}

		for _, tt := range tableTests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)

					if tt.expectedDocID == "" {
						assert.NotContains(t, string(docs[0].Action), "_id", "expected _id to not be set")
					} else {
						assert.Equal(t, tt.expectedDocID, actionJSONToID(t, docs[0].Action), "expected _id to be set")
					}

					// Ensure the document id attribute is removed from the final document.
					assert.NotContains(t, string(docs[0].Document), elasticsearch.DocumentIDAttributeName, "expected document id attribute to be removed")
					return itemsAllOK(docs)
				})

				exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
					cfg.LogsDynamicID.Enabled = true
				})
				logs := newLogsWithAttributes(
					tt.recordAttrs,
					map[string]any{
						"elastic.mapping.mode": "otel",
					},
					map[string]any{},
				)
				logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("hello world")
				mustSendLogs(t, exporter, logs)

				rec.WaitItems(1)
			})
		}
	})

	t.Run("publish with dynamic pipeline", func(t *testing.T) {
		t.Parallel()
		examplePipeline := "abc123"
		tableTests := []struct {
			name             string
			expectedPipeline string // "" means the pipeline will not be set
			recordAttrs      map[string]any
		}{
			{
				name:             "missing document pipeline attribute should not set pipeline",
				expectedPipeline: "",
			},
			{
				name:             "empty document pipeline attribute should not set pipeline",
				expectedPipeline: "",
				recordAttrs: map[string]any{
					elasticsearch.DocumentPipelineAttributeName: "",
				},
			},
			{
				name:             "record attributes",
				expectedPipeline: examplePipeline,
				recordAttrs: map[string]any{
					elasticsearch.DocumentPipelineAttributeName: examplePipeline,
				},
			},
		}

		cfgs := map[string]func(*Config){
			"async": func(cfg *Config) {
				cfg.QueueBatchConfig.Get().WaitForResult = false
				cfg.QueueBatchConfig.Get().BlockOnOverflow = false
			},
			"sync": func(cfg *Config) {
				cfg.QueueBatchConfig.Get().WaitForResult = true
				cfg.QueueBatchConfig.Get().BlockOnOverflow = false
			},
		}
		for _, tt := range tableTests {
			for cfgName, cfgFn := range cfgs {
				t.Run(tt.name+"/"+cfgName, func(t *testing.T) {
					t.Parallel()
					rec := newBulkRecorder()
					server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
						rec.Record(docs)

						if tt.expectedPipeline == "" {
							assert.NotContainsf(t, string(docs[0].Action), "pipeline", "%s: expected pipeline to not be set", tt.name)
						} else {
							assert.Equalf(t, tt.expectedPipeline, actionJSONToPipeline(t, docs[0].Action), "%s: expected pipeline to be set in action: %s", tt.name, docs[0].Action)
						}

						// Ensure the document id attribute is removed from the final document.
						assert.NotContainsf(t, string(docs[0].Document), elasticsearch.DocumentPipelineAttributeName, "%s: expected document pipeline attribute to be removed", tt.name)
						return itemsAllOK(docs)
					})

					exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
						cfg.Mapping.Mode = "otel"
						cfg.LogsDynamicPipeline.Enabled = true
						cfgFn(cfg)
					})
					logs := newLogsWithAttributes(
						tt.recordAttrs,
						map[string]any{},
						map[string]any{},
					)
					logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("hello world")
					mustSendLogs(t, exporter, logs)

					rec.WaitItems(1)
				})
			}
		}
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
		dpSum := pmetric.NewNumberDataPoint()
		dpSum.SetDoubleValue(123.456)
		dpSum.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-2 * time.Second)))
		mustSendMetricSumDataPoints(t, exporter, dpSum)

		dpGauge := pmetric.NewNumberDataPoint()
		dpGauge.SetDoubleValue(123.456)
		// Keep timestamp different to avoid metric grouping putting them in same doc
		dpGauge.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Second)))
		mustSendMetricGaugeDataPoints(t, exporter, dpGauge)

		rec.WaitItems(2)
	})

	t.Run("publish with elasticsearch.index", func(t *testing.T) {
		index := "someindex"
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})
		metrics := newMetricsWithAttributes(
			map[string]any{
				"elasticsearch.index": index,
			},
			map[string]any{
				"elasticsearch.index": "ignored",
			},
			map[string]any{
				"elasticsearch.index": "ignored",
			},
		)
		mustSendMetrics(t, exporter, metrics)

		docs := rec.WaitItems(1)
		doc := docs[0]
		assert.Equal(t, index, actionJSONToIndex(t, doc.Action))
		assert.JSONEq(t, `{}`, gjson.GetBytes(doc.Document, `attributes`).Raw)
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
			cfg.Mapping.Mode = "ecs"
		})
		metrics := newMetricsWithAttributes(
			map[string]any{
				elasticsearch.DataStreamNamespace: "data.point.namespace.-\\/*?\"<>| ,#:",
			},
			nil,
			map[string]any{
				elasticsearch.DataStreamDataset:   "resource.dataset.\\/*?\"<>| ,#:",
				elasticsearch.DataStreamNamespace: "resource.namespace",
			},
		)
		metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("my.metric")
		mustSendMetrics(t, exporter, metrics)

		rec.WaitItems(1)
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

		err := exporter.ConsumeMetrics(t.Context(), metrics)
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

		err := exporter.ConsumeMetrics(t.Context(), metrics)
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

		err := exporter.ConsumeMetrics(t.Context(), metrics)
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

		sumMonotonicCumulative := metricSlice.AppendEmpty()
		sumMonotonicCumulative.SetName("metric.sum.monotonic.cumulative")
		sumMonotonicCumulative.SetEmptySum()
		sumMonotonicCumulative.Sum().SetIsMonotonic(true)
		sumMonotonicCumulative.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp := sumMonotonicCumulative.Sum().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(7200, 0)))
		dp.SetDoubleValue(1.5)

		sumMonotonicDelta := metricSlice.AppendEmpty()
		sumMonotonicDelta.SetName("metric.sum.monotonic.delta")
		sumMonotonicDelta.SetEmptySum()
		sumMonotonicDelta.Sum().SetIsMonotonic(true)
		sumMonotonicDelta.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp = sumMonotonicDelta.Sum().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(7200, 0)))
		dp.SetDoubleValue(2.5)

		sumCumulative := metricSlice.AppendEmpty()
		sumCumulative.SetName("metric.sum.nonmonotonic.cumulative")
		sumCumulative.SetEmptySum()
		sumCumulative.Sum().SetIsMonotonic(false)
		sumCumulative.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp = sumCumulative.Sum().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(7200, 0)))
		dp.SetDoubleValue(3.5)

		sumDelta := metricSlice.AppendEmpty()
		sumDelta.SetName("metric.sum.nonmonotonic.delta")
		sumDelta.SetEmptySum()
		sumDelta.Sum().SetIsMonotonic(false)
		sumDelta.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp = sumDelta.Sum().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0)))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(7200, 0)))
		dp.SetDoubleValue(4.5)

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
				Document: []byte(`{"@timestamp":0,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"metric.foo":{"counts":[1,2,3,4],"values":[0.5,1.5,2.5,3.0]}},"resource":{},"scope":{},"_metric_names_hash":"b23939f78dc5f649"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.metric.foo":"histogram"}}}`),
				Document: []byte(`{"@timestamp":3600000,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"metric.foo":{"counts":[4,5,6,7],"values":[2.0,4.5,5.5,6.0]}},"resource":{},"scope":{},"_metric_names_hash":"b23939f78dc5f649"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.metric.sum.monotonic.delta":"gauge_double","metrics.metric.sum.nonmonotonic.cumulative":"gauge_double","metrics.metric.sum.nonmonotonic.delta":"gauge_double","metrics.metric.sum.monotonic.cumulative":"counter_double"}}}`),
				Document: []byte(`{"@timestamp":3600000,"start_timestamp":7200000,"data_stream":{"type":"metrics","dataset":"generic.otel","namespace":"default"},"resource":{},"scope":{},"metrics":{"metric.sum.monotonic.cumulative":1.5,"metric.sum.monotonic.delta":2.5,"metric.sum.nonmonotonic.cumulative":3.5,"metric.sum.nonmonotonic.delta":4.5},"_metric_names_hash":"4c23ec9ba381ab20"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.metric.summary":"summary"}}}`),
				Document: []byte(`{"@timestamp":10800000,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"metric.summary":{"sum":1.5,"value_count":1}},"resource":{},"scope":{},"start_timestamp":10800000,"_metric_names_hash":"2f30c89222c9d308"}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode attribute", func(t *testing.T) {
		for _, tc := range []struct {
			name string

			recordAttrs   map[string]any
			scopeAttrs    map[string]any
			resourceAttrs map[string]any

			wantRecordAttrs   string
			wantScopeAttrs    string
			wantResourceAttrs string
		}{
			{
				name: "slice value",
				recordAttrs: map[string]any{
					"some.record.attribute": []string{"foo", "bar"},
				},
				scopeAttrs: map[string]any{
					"some.scope.attribute": []string{"foo", "bar"},
				},
				resourceAttrs: map[string]any{
					"some.resource.attribute": []string{"foo", "bar"},
				},
				wantRecordAttrs:   `{"some.record.attribute":["foo","bar"]}`,
				wantScopeAttrs:    `{"some.scope.attribute":["foo","bar"]}`,
				wantResourceAttrs: `{"some.resource.attribute":["foo","bar"]}`,
			},
			{
				name: "key prefix conflict",
				recordAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				scopeAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				resourceAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				wantRecordAttrs:   `{"a":"a","a.b":"a.b"}`,
				wantScopeAttrs:    `{"a":"a","a.b":"a.b"}`,
				wantResourceAttrs: `{"a":"a","a.b":"a.b"}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
					cfg.Mapping.Mode = "otel"
				})

				mustSendMetrics(t, exporter, newMetricsWithAttributes(tc.recordAttrs, tc.scopeAttrs, tc.resourceAttrs))
				rec.WaitItems(1)

				assert.Len(t, rec.Items(), 1)
				doc := rec.Items()[0].Document
				assert.JSONEq(t, tc.wantRecordAttrs, gjson.GetBytes(doc, `attributes`).Raw)
				assert.JSONEq(t, tc.wantScopeAttrs, gjson.GetBytes(doc, `scope.attributes`).Raw)
				assert.JSONEq(t, tc.wantResourceAttrs, gjson.GetBytes(doc, `resource.attributes`).Raw)
			})
		}
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
				Document: []byte(`{"@timestamp":0,"_doc_count":10,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"sum":0,"summary":{"sum":1.0,"value_count":10}},"resource":{},"scope":{},"_metric_names_hash":"e446964dc8337bbb"}`),
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
				Document: []byte(`{"@timestamp":0,"_doc_count":10,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"attributes":{},"metrics":{"histogram.summary":{"sum":1.0,"value_count":10}},"resource":{},"scope":{},"_metric_names_hash":"fcd1d6737d725996"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic.otel-default","dynamic_templates":{"metrics.exphistogram.summary":"summary"}}}`),
				Document: []byte(`{"@timestamp":3600000,"_doc_count":10,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"attributes":{},"metrics":{"exphistogram.summary":{"sum":1.0,"value_count":10}},"resource":{},"scope":{},"_metric_names_hash":"6a10ca190ae63c5"}`),
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
				Document: []byte(`{"@timestamp":0,"data_stream":{"dataset":"generic.otel","namespace":"default","type":"metrics"},"metrics":{"foo":0,"foo.bar":0,"foo.bar.baz":0},"resource":{},"scope":{},"_metric_names_hash":"9c732a69b35274fe"}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
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

func TestExporterMetrics_Grouping(t *testing.T) {
	{
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
			barDp := barDps.AppendEmpty() // dp without attribute
			barDp.SetDoubleValue(1.0)
			barOtherDp := barDps.AppendEmpty()
			fillAttributeMap(barOtherDp.Attributes(), map[string]any{
				"dp.attribute": "dp.attribute.value",
			})
			barOtherDp.SetDoubleValue(1.0)
			barOtherIndexDp := barDps.AppendEmpty()
			fillAttributeMap(barOtherIndexDp.Attributes(), map[string]any{
				"dp.attribute":                    "dp.attribute.value",
				elasticsearch.DataStreamNamespace: "bar",
			})
			barOtherIndexDp.SetDoubleValue(1.0)

			bazMetric := metricSlice.AppendEmpty()
			bazMetric.SetName("metric.baz")
			bazDps := bazMetric.SetEmptyGauge().DataPoints()
			bazDp := bazDps.AppendEmpty()
			bazDp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(3600, 0))) // dp with different timestamp
			bazDp.SetDoubleValue(1.0)
		}

		metrics := pmetric.NewMetrics()
		resourceA := metrics.ResourceMetrics().AppendEmpty()
		resourceA.SetSchemaUrl("http://example.com/resource_schema")
		fillAttributeMap(resourceA.Resource().Attributes(), map[string]any{
			elasticsearch.DataStreamNamespace: "resource.namespace",
		})
		scopeAA := resourceA.ScopeMetrics().AppendEmpty()
		addToMetricSlice(scopeAA.Metrics())

		scopeAB := resourceA.ScopeMetrics().AppendEmpty()
		fillAttributeMap(scopeAB.Scope().Attributes(), map[string]any{
			elasticsearch.DataStreamDataset: "scope.ab", // routes to a different index and should not be grouped together
		})
		scopeAB.SetSchemaUrl("http://example.com/scope_schema")
		addToMetricSlice(scopeAB.Metrics())

		scopeAC := resourceA.ScopeMetrics().AppendEmpty()
		fillAttributeMap(scopeAC.Scope().Attributes(), map[string]any{
			// ecs: scope attributes are ignored, and duplicates are dropped silently.
			// otel: scope attributes are dimensions and should result in a separate group.
			"some.scope.attribute": "scope.ac",
		})
		addToMetricSlice(scopeAC.Metrics())

		resourceB := metrics.ResourceMetrics().AppendEmpty()
		fillAttributeMap(resourceB.Resource().Attributes(), map[string]any{
			"my.resource": "resource.b",
		})
		scopeBA := resourceB.ScopeMetrics().AppendEmpty()
		addToMetricSlice(scopeBA.Metrics())

		scopeBB := resourceB.ScopeMetrics().AppendEmpty()
		scopeBB.Scope().SetName("scope.bb")
		addToMetricSlice(scopeBB.Metrics())

		// identical resource
		resourceAnotherB := metrics.ResourceMetrics().AppendEmpty()
		fillAttributeMap(resourceAnotherB.Resource().Attributes(), map[string]any{
			"my.resource": "resource.b",
		})
		addToMetricSlice(resourceAnotherB.ScopeMetrics().AppendEmpty().Metrics())

		assertDocsInIndices := func(t *testing.T, wantDocsPerIndex map[string]int, rec *bulkRecorder) {
			var sum int
			for _, v := range wantDocsPerIndex {
				sum += v
			}
			rec.WaitItems(sum)

			actualDocsPerIndex := make(map[string]int)
			for _, item := range rec.Items() {
				idx := gjson.GetBytes(item.Action, "create._index")
				actualDocsPerIndex[idx.String()]++
			}
			assert.Equal(t, wantDocsPerIndex, actualDocsPerIndex)
		}

		t.Run("ecs", func(t *testing.T) {
			rec := newBulkRecorder()
			server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
				rec.Record(docs)
				return itemsAllOK(docs)
			})

			exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
				cfg.Mapping.Mode = "ecs"
			})

			mustSendMetrics(t, exporter, metrics)

			assertDocsInIndices(t, map[string]int{
				"metrics-generic-bar":                 2, // AA, BA
				"metrics-generic-resource.namespace":  3,
				"metrics-scope.ab-bar":                1,
				"metrics-scope.ab-resource.namespace": 3,
				"metrics-generic-default":             3,
			}, rec)
		})

		t.Run("otel", func(t *testing.T) {
			rec := newBulkRecorder()
			server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
				rec.Record(docs)
				return itemsAllOK(docs)
			})

			exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
				cfg.Mapping.Mode = "otel"
			})

			mustSendMetrics(t, exporter, metrics)

			assertDocsInIndices(t, map[string]int{
				"metrics-generic.otel-bar":                 4, // AA->bar, AC->bar, BA->bar, BB->bar
				"metrics-generic.otel-resource.namespace":  6, // AA, AC
				"metrics-scope.ab.otel-bar":                1, // AB->bar
				"metrics-scope.ab.otel-resource.namespace": 3, // AB
				"metrics-generic.otel-default":             6, // BA, BB
			}, rec)

			type resourceScope struct {
				resource attribute.Distinct
				scope    attribute.Distinct
			}

			resourceScopeGroupings := make(map[resourceScope]int)
			for _, item := range rec.Items() {
				resource := gjson.GetBytes(item.Document, "resource")
				require.True(t, resource.Exists())
				scope := gjson.GetBytes(item.Document, "scope")
				require.True(t, scope.Exists())
				rs := resourceScope{
					resource: mapToDistinct(resource.Value().(map[string]any)),
					scope:    mapToDistinct(scope.Value().(map[string]any)),
				}
				resourceScopeGroupings[rs]++
			}

			assert.Equal(t, map[resourceScope]int{
				{
					// AA
					resource: mapToDistinct(map[string]any{
						"schema_url": "http://example.com/resource_schema",
					}),
					scope: mapToDistinct(nil),
				}: 4,
				{
					// AB
					resource: mapToDistinct(map[string]any{
						"schema_url": "http://example.com/resource_schema",
					}),
					scope: mapToDistinct(map[string]any{
						"schema_url": "http://example.com/scope_schema",
					}),
				}: 4,
				{
					// AC
					resource: mapToDistinct(map[string]any{
						"schema_url": "http://example.com/resource_schema",
					}),
					scope: mapToDistinct(map[string]any{
						"attributes": map[string]any{"some.scope.attribute": "scope.ac"},
					}),
				}: 4,
				{
					// BA
					resource: mapToDistinct(map[string]any{
						"attributes": map[string]any{"my.resource": "resource.b"},
					}),
					scope: mapToDistinct(nil),
				}: 4,
				{
					// BB
					resource: mapToDistinct(map[string]any{
						"attributes": map[string]any{"my.resource": "resource.b"},
					}),
					scope: mapToDistinct(map[string]any{
						"name": "scope.bb",
					}),
				}: 4,
			}, resourceScopeGroupings)
		})
	}

	type kv struct {
		k, v string
	}

	// addMetric adds a new metric with name `name` and data point attributes `dpAttrs` to MetricSlice `slice`.
	addMetric := func(slice pmetric.MetricSlice, name string, dpAttrs []kv) {
		metric := slice.AppendEmpty()
		metric.SetName(name)
		dp := metric.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetDoubleValue(1)
		for _, kv := range dpAttrs {
			dp.Attributes().PutStr(kv.k, kv.v)
		}
	}

	t.Run("ecs dp attribute overwrites resource attribute", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "ecs"
		})

		metrics := pmetric.NewMetrics()
		resource := metrics.ResourceMetrics().AppendEmpty()
		resource.Resource().Attributes().PutStr("a", "b")
		scope := resource.ScopeMetrics().AppendEmpty()

		// a=c overwrites a=b in resource attribute
		addMetric(scope.Metrics(), "foo", []kv{{k: "a", v: "c"}})

		// no attribute, should still serialize as a=b
		addMetric(scope.Metrics(), "bar", nil)

		// a=b overwrites a=b in resource attribute
		addMetric(scope.Metrics(), "baz", []kv{{k: "a", v: "b"}})

		mustSendMetrics(t, exporter, metrics)
		expected := []itemRequest{
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"foo":1.0,"a":"c"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"metrics-generic-default"}}`),
				Document: []byte(`{"@timestamp":"1970-01-01T00:00:00.000000000Z","data_stream":{"dataset":"generic","namespace":"default","type":"metrics"},"bar":1.0,"baz":1.0,"a":"b"}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("unordered attributes", func(t *testing.T) {
		// Test equal attributes but in a different order. Metrics should be grouped together.
		for _, mode := range []string{"ecs", "otel"} {
			t.Run(mode, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				exporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
					cfg.Mapping.Mode = mode
				})

				metrics := pmetric.NewMetrics()
				resourceA := metrics.ResourceMetrics().AppendEmpty()
				resourceA.Resource().Attributes().PutStr("a", "a")
				resourceA.Resource().Attributes().PutStr("b", "b")
				scopeA := resourceA.ScopeMetrics().AppendEmpty()
				scopeA.Scope().Attributes().PutStr("c", "c")
				scopeA.Scope().Attributes().PutStr("d", "d")

				addMetric(scopeA.Metrics(), "a_foo", []kv{
					{"e", "e"},
					{"f", "f"},
				})

				addMetric(scopeA.Metrics(), "a_bar", []kv{
					{"f", "f"},
					{"e", "e"},
				})

				resourceB := metrics.ResourceMetrics().AppendEmpty()
				resourceB.Resource().Attributes().PutStr("b", "b")
				resourceB.Resource().Attributes().PutStr("a", "a")
				scopeB := resourceB.ScopeMetrics().AppendEmpty()
				scopeB.Scope().Attributes().PutStr("d", "d")
				scopeB.Scope().Attributes().PutStr("c", "c")

				addMetric(scopeB.Metrics(), "b_foo", []kv{
					{"e", "e"},
					{"f", "f"},
				})

				addMetric(scopeB.Metrics(), "b_bar", []kv{
					{"f", "f"},
					{"e", "e"},
				})

				mustSendMetrics(t, exporter, metrics)

				rec.WaitItems(1)
				assert.Len(t, rec.Items(), 1)
				// Sanity check that all metrics are included
				assert.Contains(t, string(rec.Items()[0].Document), "a_foo")
				assert.Contains(t, string(rec.Items()[0].Document), "a_bar")
				assert.Contains(t, string(rec.Items()[0].Document), "b_foo")
				assert.Contains(t, string(rec.Items()[0].Document), "b_bar")
			})
		}
	})

	t.Run("mapping hints excluded from grouping", func(t *testing.T) {
		// Test that data points with different mapping hints are grouped together
		// since elasticsearch.mapping.hints should be excluded from the hash.
		for _, mode := range []string{"ecs", "otel"} {
			t.Run(mode, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				testMetricsExporter := newTestMetricsExporter(t, server.URL, func(cfg *Config) {
					cfg.Mapping.Mode = mode
				})

				metrics := pmetric.NewMetrics()
				resource := metrics.ResourceMetrics().AppendEmpty()
				scope := resource.ScopeMetrics().AppendEmpty()

				// Metric with mapping hints
				fooMetric := scope.Metrics().AppendEmpty()
				fooMetric.SetName("metric.foo")
				fooDp := fooMetric.SetEmptyGauge().DataPoints().AppendEmpty()
				fooDp.SetDoubleValue(1.0)
				hints := fooDp.Attributes().PutEmptySlice(elasticsearch.MappingHintsAttrKey)
				hints.AppendEmpty().SetStr(string(elasticsearch.HintAggregateMetricDouble))

				// Metric without mapping hints - should be grouped with the above
				barMetric := scope.Metrics().AppendEmpty()
				barMetric.SetName("metric.bar")
				barDp := barMetric.SetEmptyGauge().DataPoints().AppendEmpty()
				barDp.SetDoubleValue(2.0)

				// Metric with different mapping hints - should still be grouped
				bazMetric := scope.Metrics().AppendEmpty()
				bazMetric.SetName("metric.baz")
				bazDp := bazMetric.SetEmptyGauge().DataPoints().AppendEmpty()
				bazDp.SetDoubleValue(3.0)
				bazHints := bazDp.Attributes().PutEmptySlice(elasticsearch.MappingHintsAttrKey)
				bazHints.AppendEmpty().SetStr(string(elasticsearch.HintDocCount))

				mustSendMetrics(t, testMetricsExporter, metrics)

				rec.WaitItems(1)
				assert.Len(t, rec.Items(), 1)
				// Sanity check that all metrics are included in a single document
				// ECS mode uses short names (foo, bar, baz), OTel mode uses full names (metric.foo, etc.)
				doc := string(rec.Items()[0].Document)
				assert.Contains(t, doc, "foo")
				assert.Contains(t, doc, "bar")
				assert.Contains(t, doc, "baz")
			})
		}
	})
}

func mapToDistinct(m map[string]any) attribute.Distinct {
	var kvs []attribute.KeyValue
	for k, v := range m {
		switch v := v.(type) {
		case string:
			kvs = append(kvs, attribute.String(k, v))
		case map[string]any:
			for subk, v := range v {
				kvs = append(kvs, attribute.String(k+"."+subk, v.(string)))
			}
		default:
			panic("aiee!")
		}
	}
	set := attribute.NewSet(kvs...)
	return set.Equivalent()
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

	t.Run("publish with elasticsearch.index", func(t *testing.T) {
		rec := newBulkRecorder()
		index := "someindex"
		eventIndex := "some-event-index"

		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.Mapping.Mode = "otel"
		})

		traces := newTracesWithAttributes(
			map[string]any{
				"elasticsearch.index": index,
			},
			map[string]any{
				"elasticsearch.index": "ignored",
			},
			map[string]any{
				"elasticsearch.index": "ignored",
			},
		)
		event := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().AppendEmpty()
		event.Attributes().PutStr("elasticsearch.index", eventIndex)
		mustSendTraces(t, exporter, traces)

		docs := rec.WaitItems(2)
		doc := docs[0]
		assert.Equal(t, index, actionJSONToIndex(t, doc.Action))
		assert.JSONEq(t, `{}`, gjson.GetBytes(doc.Document, `attributes`).Raw)
		eventDoc := docs[1]
		assert.Equal(t, eventIndex, actionJSONToIndex(t, eventDoc.Action))
		assert.JSONEq(t, `{}`, gjson.GetBytes(eventDoc.Document, `attributes`).Raw)
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
			cfg.Mapping.Mode = "none"
		})

		mustSendTraces(t, exporter, newTracesWithAttributes(
			map[string]any{
				elasticsearch.DataStreamDataset: "span.dataset.\\/*?\"<>| ,#:",
			},
			nil,
			map[string]any{
				elasticsearch.DataStreamDataset: "resource.dataset",
			},
		))

		rec.WaitItems(1)
	})

	t.Run("publish with logstash format index, default traces index", func(t *testing.T) {
		var defaultCfg Config

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			assert.Contains(t, actionJSONToIndex(t, docs[0].Action), defaultCfg.TracesIndex)

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.LogstashFormat.Enabled = true
			defaultCfg = *cfg
		})

		mustSendTraces(t, exporter, newTracesWithAttributes(nil, nil, nil))

		rec.WaitItems(1)
	})

	t.Run("publish with logstash format index", func(t *testing.T) {
		index := "someindex"

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			assert.Contains(t, actionJSONToIndex(t, docs[0].Action), index)

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.LogstashFormat.Enabled = true
		})

		mustSendTraces(t, exporter, newTracesWithAttributes(
			map[string]any{
				"elasticsearch.index": index,
			},
			nil,
			nil,
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
		spanLink.SetTraceID([16]byte{1})
		spanLink.SetSpanID([8]byte{1})
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
				Document: []byte(`{"@timestamp":"3600000.0","attributes":{"attr.foo":"attr.bar"},"data_stream":{"dataset":"generic.otel","namespace":"default","type":"traces"},"dropped_attributes_count":2,"dropped_events_count":3,"dropped_links_count":4,"duration":3600000000000,"kind":"Unspecified","links":[{"attributes":{"link.attr.foo":"link.attr.bar"},"dropped_attributes_count":11,"span_id":"0100000000000000","trace_id":"01000000000000000000000000000000","trace_state":"bar"}],"name":"name","resource":{"attributes":{"resource.foo":"resource.bar"}},"scope":{},"status":{},"trace_state":"foo"}`),
			},
			{
				Action:   []byte(`{"create":{"_index":"logs-generic.otel-default"}}`),
				Document: []byte(`{"@timestamp":"0.0","event_name":"exception","attributes":{"event.attr.foo":"event.attr.bar","event.name":"exception"},"event_name":"exception","data_stream":{"dataset":"generic.otel","namespace":"default","type":"logs"},"dropped_attributes_count":1,"resource":{"attributes":{"resource.foo":"resource.bar"}},"scope":{}}`),
			},
		}

		assertRecordedItems(t, expected, rec, false)
	})

	t.Run("otel mode span event routing", func(t *testing.T) {
		for _, tc := range []struct {
			name           string
			config         func(cfg *Config)
			spanEventAttrs map[string]any
			wantIndex      string
		}{
			{
				name:      "default",
				wantIndex: "logs-generic.otel-default",
			},
			{
				name: "static index config",
				config: func(cfg *Config) {
					cfg.LogsIndex = "someindex"
					cfg.MetricsIndex = "ignored"
					cfg.TracesIndex = "ignored"
				},
				wantIndex: "someindex",
			},
			{
				name: "dynamic elasticsearch.index",
				spanEventAttrs: map[string]any{
					"elasticsearch.index": "someindex",
				},
				wantIndex: "someindex",
			},
			{
				name: "dynamic data_stream.*",
				spanEventAttrs: map[string]any{
					"data_stream.dataset":   "foo",
					"data_stream.namespace": "bar",
				},
				wantIndex: "logs-foo.otel-bar",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				configs := []func(cfg *Config){
					func(cfg *Config) {
						cfg.Mapping.Mode = "otel"
					},
				}
				if tc.config != nil {
					configs = append(configs, tc.config)
				}

				exporter := newTestTracesExporter(t, server.URL, configs...)

				traces := newTracesWithAttributes(nil, nil, nil)
				spanEvent := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().AppendEmpty()
				spanEvent.SetName("some_event_name")
				fillAttributeMap(spanEvent.Attributes(), tc.spanEventAttrs)
				mustSendTraces(t, exporter, traces)

				rec.WaitItems(2)
				var spanEventDocs []itemRequest
				for _, doc := range rec.Items() {
					if result := gjson.GetBytes(doc.Document, "event_name"); result.Raw != "" {
						spanEventDocs = append(spanEventDocs, doc)
					}
				}
				require.Len(t, spanEventDocs, 1)
				assert.Equal(t, tc.wantIndex, gjson.GetBytes(spanEventDocs[0].Action, "create._index").Str)
			})
		}
	})

	t.Run("otel mode attribute", func(t *testing.T) {
		for _, tc := range []struct {
			name string

			recordAttrs   map[string]any
			scopeAttrs    map[string]any
			resourceAttrs map[string]any

			wantRecordAttrs   string
			wantScopeAttrs    string
			wantResourceAttrs string
		}{
			{
				name: "slice value",
				recordAttrs: map[string]any{
					"some.record.attribute": []string{"foo", "bar"},
				},
				scopeAttrs: map[string]any{
					"some.scope.attribute": []string{"foo", "bar"},
				},
				resourceAttrs: map[string]any{
					"some.resource.attribute": []string{"foo", "bar"},
				},
				wantRecordAttrs:   `{"some.record.attribute":["foo","bar"]}`,
				wantScopeAttrs:    `{"some.scope.attribute":["foo","bar"]}`,
				wantResourceAttrs: `{"some.resource.attribute":["foo","bar"]}`,
			},
			{
				name: "key prefix conflict",
				recordAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				scopeAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				resourceAttrs: map[string]any{
					"a":   "a",
					"a.b": "a.b",
				},
				wantRecordAttrs:   `{"a":"a","a.b":"a.b"}`,
				wantScopeAttrs:    `{"a":"a","a.b":"a.b"}`,
				wantResourceAttrs: `{"a":"a","a.b":"a.b"}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
					cfg.Mapping.Mode = "otel"
				})

				traces := newTracesWithAttributes(tc.recordAttrs, tc.scopeAttrs, tc.resourceAttrs)
				spanEventAttrs := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().AppendEmpty().Attributes()
				fillAttributeMap(spanEventAttrs, tc.recordAttrs)
				mustSendTraces(t, exporter, traces)

				rec.WaitItems(2)

				assert.Len(t, rec.Items(), 2)
				for _, item := range rec.Items() {
					doc := item.Document
					assert.JSONEq(t, tc.wantRecordAttrs, gjson.GetBytes(doc, `attributes`).Raw)
					assert.JSONEq(t, tc.wantScopeAttrs, gjson.GetBytes(doc, `scope.attributes`).Raw)
					assert.JSONEq(t, tc.wantResourceAttrs, gjson.GetBytes(doc, `resource.attributes`).Raw)
				}
			})
		}
	})

	t.Run("publish spans with dynamic id", func(t *testing.T) {
		t.Parallel()
		exampleDocID := "example-doc-id-123"
		tableTests := []struct {
			name          string
			expectedDocID string // "" means the document ID will not be set
			spanAttrs     map[string]any
		}{
			{
				name:          "missing document id attribute should not set _id",
				expectedDocID: "",
			},
			{
				name:          "empty document id attribute should not set _id",
				expectedDocID: "",
				spanAttrs: map[string]any{
					elasticsearch.DocumentIDAttributeName: "",
				},
			},
			{
				name:          "valid document id attribute should set _id",
				expectedDocID: exampleDocID,
				spanAttrs: map[string]any{
					elasticsearch.DocumentIDAttributeName: exampleDocID,
				},
			},
		}

		for _, tt := range tableTests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)

					if tt.expectedDocID == "" {
						assert.NotContains(t, string(docs[0].Action), "_id", "expected _id to not be set")
					} else {
						assert.Equal(t, tt.expectedDocID, actionJSONToID(t, docs[0].Action), "expected _id to be set")
					}

					// Ensure the document id attribute is removed from the final document.
					assert.NotContains(t, string(docs[0].Document), elasticsearch.DocumentIDAttributeName, "expected document id attribute to be removed")
					return itemsAllOK(docs)
				})

				exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
					cfg.TracesDynamicID.Enabled = true
				})
				traces := newTracesWithAttributes(
					tt.spanAttrs,
					map[string]any{
						"elastic.mapping.mode": "otel",
					},
					map[string]any{},
				)
				mustSendTraces(t, exporter, traces)

				rec.WaitItems(1)
			})
		}
	})

	t.Run("publish span events with dynamic id", func(t *testing.T) {
		t.Parallel()
		exampleDocID := "span-event-doc-id-456"
		tableTests := []struct {
			name          string
			expectedDocID string
			eventAttrs    map[string]any
		}{
			{
				name:          "missing document id attribute should not set _id",
				expectedDocID: "",
			},
			{
				name:          "empty document id attribute should not set _id",
				expectedDocID: "",
				eventAttrs: map[string]any{
					elasticsearch.DocumentIDAttributeName: "",
				},
			},
			{
				name:          "span event attributes",
				expectedDocID: exampleDocID,
				eventAttrs: map[string]any{
					elasticsearch.DocumentIDAttributeName: exampleDocID,
				},
			},
		}

		for _, tt := range tableTests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
					cfg.TracesDynamicID.Enabled = true
				})
				traces := newTracesWithAttributes(
					map[string]any{},
					map[string]any{
						"elastic.mapping.mode": "otel",
					},
					map[string]any{},
				)
				spanEvent := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().AppendEmpty()
				spanEvent.SetName("test_event")
				fillAttributeMap(spanEvent.Attributes(), tt.eventAttrs)
				mustSendTraces(t, exporter, traces)

				// Wait for both span and span event
				rec.WaitItems(2)

				// Find the span event document
				var spanEventDoc *itemRequest
				for _, doc := range rec.Items() {
					if result := gjson.GetBytes(doc.Document, "event_name"); result.Raw != "" {
						spanEventDoc = &doc
						break
					}
				}

				require.NotNil(t, spanEventDoc, "span event document should be present")

				if tt.expectedDocID == "" {
					assert.NotContains(t, string(spanEventDoc.Action), "_id", "expected _id to not be set")
				} else {
					assert.Equal(t, tt.expectedDocID, actionJSONToID(t, spanEventDoc.Action), "expected _id to be set")
				}

				// Ensure the document id attribute is removed from the final document.
				assert.NotContains(t, string(spanEventDoc.Document), elasticsearch.DocumentIDAttributeName, "expected document id attribute to be removed")
			})
		}
	})

	t.Run("publish with dynamic id in ecs mode", func(t *testing.T) {
		t.Parallel()
		// Test that spans respect dynamic document IDs in ECS mode,
		// while span events are embedded (not separate documents)
		exampleDocID := "ecs-span-doc-id-789"

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.TracesDynamicID.Enabled = true
		})

		traces := newTracesWithAttributes(
			map[string]any{
				elasticsearch.DocumentIDAttributeName: exampleDocID,
			},
			map[string]any{
				"elastic.mapping.mode": "ecs",
			},
			map[string]any{},
		)

		// Add a span event with its own document ID attribute (should be ignored in ECS mode)
		spanEvent := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().AppendEmpty()
		spanEvent.SetName("test_event_ecs")
		spanEvent.Attributes().PutStr(elasticsearch.DocumentIDAttributeName, "should-be-ignored")

		mustSendTraces(t, exporter, traces)

		// In ECS mode, only the span document is created (span events are embedded)
		rec.WaitItems(1)
		docs := rec.Items()
		require.Len(t, docs, 1, "should only have 1 document (span) in ECS mode")

		// Verify the span document has the correct _id
		assert.Equal(t, exampleDocID, actionJSONToID(t, docs[0].Action), "span should have dynamic _id")

		// Verify the document ID attribute is removed from the span
		assert.NotContains(t, string(docs[0].Document), elasticsearch.DocumentIDAttributeName, "document id attribute should be removed")
	})
}

func TestExporter_DynamicMappingMode(t *testing.T) {
	otelContext := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{"X-Elastic-Mapping-Mode": {"otel"}}),
	})
	ecsContext := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{"X-Elastic-Mapping-Mode": {"ecs"}}),
	})
	noneContext := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{"X-Elastic-Mapping-Mode": {"none"}}),
	})
	multipleModesContext := client.NewContext(t.Context(), client.Info{
		Metadata: client.NewMetadata(map[string][]string{"X-Elastic-Mapping-Mode": {"otel", "ecs"}}),
	})

	defaultScope := pcommon.NewInstrumentationScope()
	ecsScope := pcommon.NewInstrumentationScope()
	ecsScope.Attributes().PutStr("elastic.mapping.mode", "ecs")
	bodymapScope := pcommon.NewInstrumentationScope()
	bodymapScope.Attributes().PutStr("elastic.mapping.mode", "bodymap")

	createLogs := func(scopes ...pcommon.InstrumentationScope) plog.Logs {
		logs := plog.NewLogs()
		resourceLog := logs.ResourceLogs().AppendEmpty()
		resourceLog.Resource().Attributes().PutStr("k", "v")
		for _, scope := range scopes {
			scopeLog := resourceLog.ScopeLogs().AppendEmpty()
			scope.CopyTo(scopeLog.Scope())
			scopeLog.LogRecords().AppendEmpty()
		}
		logs.MarkReadOnly()
		return logs
	}

	createMetrics := func(scopes ...pcommon.InstrumentationScope) pmetric.Metrics {
		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		resourceMetrics.Resource().Attributes().PutStr("k", "v")
		for _, scope := range scopes {
			scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
			scope.CopyTo(scopeMetrics.Scope())
			metric := scopeMetrics.Metrics().AppendEmpty()
			metric.SetName("metric.foo")
			metric.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(123)
		}
		metrics.MarkReadOnly()
		return metrics
	}

	createTraces := func(scopes ...pcommon.InstrumentationScope) ptrace.Traces {
		traces := ptrace.NewTraces()
		resourceSpans := traces.ResourceSpans().AppendEmpty()
		resourceSpans.Resource().Attributes().PutStr("k", "v")
		for _, scope := range scopes {
			scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
			scope.CopyTo(scopeSpans.Scope())
			scopeSpans.Spans().AppendEmpty()
		}
		traces.MarkReadOnly()
		return traces
	}

	// sortItemRequests sorts by _index, so we get consistent ordering for
	// the different scopes below. This is necessary since bulk requests
	// are produced concurrently for each mapping mode.
	sortItemRequests := func(items []itemRequest) {
		sort.Slice(items, func(i, j int) bool {
			return gjson.GetBytes(items[i].Action, "create._index").Str <
				gjson.GetBytes(items[j].Action, "create._index").Str
		})
	}

	setAllowedMappingModes := func(cfg *Config) {
		cfg.Mapping.AllowedModes = []string{"ecs", "otel"}
		cfg.Mapping.Mode = "otel"
	}

	type checkFunc func(_ *testing.T, doc []byte, signal string)
	checkOTelResource := func(t *testing.T, doc []byte, _ string) {
		t.Helper()
		assert.JSONEq(t, `{"k":"v"}`, gjson.GetBytes(doc, `resource.attributes`).Raw)
	}
	checkECSResource := func(t *testing.T, doc []byte, _ string) {
		t.Helper()
		assert.Equal(t, "v", gjson.GetBytes(doc, "k").Str)
	}

	testcases := []struct {
		name      string
		ctx       context.Context
		scopes    []pcommon.InstrumentationScope
		checks    []checkFunc
		expectErr string
	}{{
		name:   "otel",
		ctx:    otelContext,
		scopes: []pcommon.InstrumentationScope{defaultScope},
		checks: []checkFunc{checkOTelResource},
	}, {
		name:   "ecs",
		ctx:    ecsContext,
		scopes: []pcommon.InstrumentationScope{defaultScope},
		checks: []checkFunc{checkECSResource},
	}, {
		name:      "none",
		ctx:       noneContext,
		scopes:    []pcommon.InstrumentationScope{defaultScope},
		expectErr: `Permanent error: invalid context mapping mode: unsupported mapping mode "none", expected one of ["ecs" "otel"]`,
	}, {
		name:      "multiple modes",
		ctx:       multipleModesContext,
		scopes:    []pcommon.InstrumentationScope{defaultScope},
		expectErr: `Permanent error: expected one value for client metadata key "x-elastic-mapping-mode", got 2`,
	}, {
		name:   "ecs_scope",
		ctx:    t.Context(),
		scopes: []pcommon.InstrumentationScope{ecsScope},
		checks: []checkFunc{checkECSResource},
	}, {
		name:   "mixed_scopes",
		ctx:    t.Context(),
		scopes: []pcommon.InstrumentationScope{ecsScope, defaultScope},
		checks: []checkFunc{checkECSResource, checkOTelResource},
	}, {
		name:      "bodymap_scope",
		ctx:       otelContext, // scope overrides context
		scopes:    []pcommon.InstrumentationScope{bodymapScope},
		expectErr: `Permanent error: invalid scope mapping mode: unsupported mapping mode "bodymap", expected one of ["ecs" "otel"]`,
	}}

	t.Run("logs", func(t *testing.T) {
		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				logs := createLogs(tc.scopes...)
				exporter := newTestLogsExporter(t, server.URL, setAllowedMappingModes, func(cfg *Config) {
					// Set wait_for_result to be true so that errors are reported directly via Consume*
					cfg.QueueBatchConfig.Get().WaitForResult = true
				})
				err := exporter.ConsumeLogs(tc.ctx, logs)
				if tc.expectErr != "" {
					require.EqualError(t, err, tc.expectErr)
					return
				}
				require.NoError(t, err)
				items := rec.WaitItems(len(tc.scopes))
				sortItemRequests(items)
				for i, item := range items {
					tc.checks[i](t, item.Document, "logs")
				}
			})
		}
	})
	t.Run("metrics", func(t *testing.T) {
		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				metrics := createMetrics(tc.scopes...)
				exporter := newTestMetricsExporter(t, server.URL, setAllowedMappingModes, func(cfg *Config) {
					// Set wait_for_result to be true so that errors are reported directly via Consume*
					cfg.QueueBatchConfig.Get().WaitForResult = true
				})
				err := exporter.ConsumeMetrics(tc.ctx, metrics)
				if tc.expectErr != "" {
					require.EqualError(t, err, tc.expectErr)
					return
				}
				require.NoError(t, err)
				items := rec.WaitItems(len(tc.scopes))
				sortItemRequests(items)
				for i, item := range items {
					tc.checks[i](t, item.Document, "metrics")
				}
			})
		}
	})
	t.Run("profiles", func(t *testing.T) {
		// Profiles are only supported by otel mode, so just verify that
		// the metadata is picked up and invalid modes are rejected.
		exporter := newTestProfilesExporter(t, "https://testing.invalid", setAllowedMappingModes, func(cfg *Config) {
			// Set wait_for_result to be true so that errors are reported directly via Consume*
			cfg.QueueBatchConfig.Get().WaitForResult = true
		})
		err := exporter.ConsumeProfiles(noneContext, pprofile.NewProfiles())
		assert.EqualError(t, err,
			`Permanent error: invalid context mapping mode: unsupported mapping mode "none", expected one of ["ecs" "otel"]`,
		)

		err = exporter.ConsumeProfiles(multipleModesContext, pprofile.NewProfiles())
		assert.EqualError(t, err,
			`Permanent error: expected one value for client metadata key "x-elastic-mapping-mode", got 2`,
		)
	})
	t.Run("traces", func(t *testing.T) {
		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				rec := newBulkRecorder()
				server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
					rec.Record(docs)
					return itemsAllOK(docs)
				})

				traces := createTraces(tc.scopes...)
				exporter := newTestTracesExporter(t, server.URL, setAllowedMappingModes, func(cfg *Config) {
					// Set wait_for_result to be true so that errors are reported directly via Consume*
					cfg.QueueBatchConfig.Get().WaitForResult = true
				})
				err := exporter.ConsumeTraces(tc.ctx, traces)
				if tc.expectErr != "" {
					require.EqualError(t, err, tc.expectErr)
					return
				}
				require.NoError(t, err)
				items := rec.WaitItems(len(tc.scopes))
				sortItemRequests(items)
				for i, item := range items {
					tc.checks[i](t, item.Document, "traces")
				}
			})
		}
	})
}

// TestExporterAuth verifies that the Elasticsearch exporter supports
// confighttp.ClientConfig.Auth.
func TestExporterAuth(t *testing.T) {
	done := make(chan struct{}, 1)
	testauthID := component.NewID(component.MustNewType("authtest"))
	exporter := newUnstartedTestLogsExporter(t, "http://testing.invalid", func(cfg *Config) {
		cfg.Auth = configoptional.Some(configauth.Config{AuthenticatorID: testauthID})
	})
	err := exporter.Start(t.Context(), &mockHost{
		extensions: map[component.ID]component.Component{
			testauthID: newMockAuthClient(func(*http.Request) (*http.Response, error) {
				select {
				case done <- struct{}{}:
				default:
				}
				return nil, errors.New("nope")
			}),
		},
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exporter.Shutdown(t.Context()))
	}()

	mustSendLogRecords(t, exporter, plog.NewLogRecord())
	<-done
}

func TestExporterBatcher(t *testing.T) {
	var requests []*http.Request
	testauthID := component.NewID(component.MustNewType("authtest"))
	exporter := newUnstartedTestLogsExporter(t, "http://testing.invalid", func(cfg *Config) {
		cfg.QueueBatchConfig.GetOrInsertDefault()
		cfg.QueueBatchConfig.Get().WaitForResult = true
		cfg.QueueBatchConfig.Get().Batch = configoptional.Some(exporterhelper.BatchConfig{
			FlushTimeout: 200 * time.Millisecond,
			Sizer:        exporterhelper.RequestSizerTypeItems,
			MinSize:      8192,
			MaxSize:      10000,
		})
		cfg.Auth = configoptional.Some(configauth.Config{AuthenticatorID: testauthID})
		cfg.Retry.Enabled = false
	})
	err := exporter.Start(t.Context(), &mockHost{
		extensions: map[component.ID]component.Component{
			testauthID: newMockAuthClient(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req)
				return nil, errors.New("nope")
			}),
		},
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, exporter.Shutdown(t.Context()))
	}()

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.LogRecords().AppendEmpty().Body().SetStr("log record body")

	type key struct{}
	_ = exporter.ConsumeLogs(context.WithValue(t.Context(), key{}, "value1"), logs)
	_ = exporter.ConsumeLogs(context.WithValue(t.Context(), key{}, "value2"), logs)
	require.Len(t, requests, 2) // flushed immediately by Consume

	assert.Equal(t, "value1", requests[0].Context().Value(key{}))
	assert.Equal(t, "value2", requests[1].Context().Value(key{}))
}

func TestExporterSendingQueueContextPropogation(t *testing.T) {
	testCtxKey := component.NewID(component.MustNewType("testctxkey"))
	metadata := client.NewMetadata(map[string][]string{
		"key_1": {"val_1"},
		"key_2": {"val_2"},
	})
	configSetupFn := func(cfg *Config) {
		cfg.MetadataKeys = slices.Collect(metadata.Keys())
		cfg.Auth = configoptional.Some(configauth.Config{
			AuthenticatorID: testCtxKey,
		})
		// Configure sending queue with batching enabled. Batching configuration are
		// kept such that test can simulate batching and the batch matures on age.
		cfg.QueueBatchConfig.Get().WaitForResult = false
		cfg.QueueBatchConfig.Get().BlockOnOverflow = true
		cfg.QueueBatchConfig.Get().QueueSize = 100 // big enough to accommodate all requests
		cfg.QueueBatchConfig.Get().NumConsumers = 10
		batchCfg := cfg.QueueBatchConfig.Get().Batch.GetOrInsertDefault()
		batchCfg.FlushTimeout = 100 * time.Millisecond
		batchCfg.Sizer = exporterhelper.RequestSizerTypeItems
		batchCfg.MinSize = 100 // big enough to accommodate all requests
		batchCfg.MaxSize = 1000
	}
	setupTestHost := func(t *testing.T) (component.Host, *bulkRecorder) {
		t.Helper()

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})
		esURL, err := url.Parse(server.URL)
		require.NoError(t, err)
		return &mockHost{
			extensions: map[component.ID]component.Component{
				testCtxKey: newMockAuthClient(func(req *http.Request) (*http.Response, error) {
					info := client.FromContext(req.Context())
					for k := range metadata.Keys() {
						assert.Equal(t, metadata.Get(k), info.Metadata.Get(k))
					}
					req.Clone(req.Context())
					req.URL.Host = esURL.Host
					req.URL.Scheme = esURL.Scheme
					req.Host = ""
					return http.DefaultTransport.RoundTrip(req)
				}),
			},
		}, rec
	}

	t.Run("metrics", func(t *testing.T) {
		testHost, rec := setupTestHost(t)
		exporter := newUnstartedTestMetricsExporter(t, "https://ignored", configSetupFn)
		require.NoError(t, exporter.Start(t.Context(), testHost))
		defer func() {
			require.NoError(t, exporter.Shutdown(t.Context()))
		}()

		sendMetrics := func(name string) {
			metrics := pmetric.NewMetrics()
			resourceMetric := metrics.ResourceMetrics().AppendEmpty()
			scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()
			fooBarMetric := scopeMetric.Metrics().AppendEmpty()
			fooBarMetric.SetName(name)
			fooBarMetric.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(0)

			ctx := client.NewContext(t.Context(), client.Info{Metadata: metadata})
			mustSendMetricsWithCtx(ctx, t, exporter, metrics)
		}

		sendMetrics("foo.bar.1")
		sendMetrics("foo.bar.2")
		rec.WaitItems(1) // both metric should be within a single doc grouped together
	})

	t.Run("logs", func(t *testing.T) {
		testHost, rec := setupTestHost(t)
		exporter := newUnstartedTestLogsExporter(t, "https://ignored", configSetupFn)
		require.NoError(t, exporter.Start(t.Context(), testHost))
		defer func() {
			require.NoError(t, exporter.Shutdown(t.Context()))
		}()

		sendLogs := func(log string) {
			logs := plog.NewLogs()
			resourceLog := logs.ResourceLogs().AppendEmpty()
			scopeLog := resourceLog.ScopeLogs().AppendEmpty()
			fooBarLog := scopeLog.LogRecords().AppendEmpty()
			fooBarLog.Body().SetStr(log)

			ctx := client.NewContext(t.Context(), client.Info{Metadata: metadata})
			mustSendLogsWithCtx(ctx, t, exporter, logs)
		}

		sendLogs("log.1")
		sendLogs("log.2")
		rec.WaitItems(2) // 2 log documents are expected
	})

	t.Run("traces", func(t *testing.T) {
		testHost, rec := setupTestHost(t)
		exporter := newUnstartedTestTracesExporter(t, "https://ignored", configSetupFn)
		require.NoError(t, exporter.Start(t.Context(), testHost))
		defer func() {
			require.NoError(t, exporter.Shutdown(t.Context()))
		}()

		sendTraces := func(name string) {
			traces := ptrace.NewTraces()
			resourceSpan := traces.ResourceSpans().AppendEmpty()
			scopeSpan := resourceSpan.ScopeSpans().AppendEmpty()
			fooBarSpan := scopeSpan.Spans().AppendEmpty()
			fooBarSpan.SetName(name)

			ctx := client.NewContext(t.Context(), client.Info{Metadata: metadata})
			mustSendTracesWithCtx(ctx, t, exporter, traces)
		}

		sendTraces("span.1")
		sendTraces("span.2")
		rec.WaitItems(2) // 2 span documents are expected
	})

	t.Run("profiles", func(t *testing.T) {
		testHost, rec := setupTestHost(t)
		exporter := newUnstartedTestProfilesExporter(t, "https://ignored", configSetupFn)
		require.NoError(t, exporter.Start(t.Context(), testHost))
		defer func() {
			require.NoError(t, exporter.Shutdown(t.Context()))
		}()

		sendProfiles := func() {
			profiles := pprofile.NewProfiles()
			dic := profiles.Dictionary()
			resource := profiles.ResourceProfiles().AppendEmpty()
			scope := resource.ScopeProfiles().AppendEmpty()
			profile := scope.Profiles().AppendEmpty()

			dic.StringTable().Append("samples", "count", "cpu", "nanoseconds")
			st := profile.SampleType()
			st.SetTypeStrindex(0)
			st.SetUnitStrindex(1)
			pt := profile.PeriodType()
			pt.SetTypeStrindex(2)
			pt.SetUnitStrindex(3)

			a := dic.AttributeTable().AppendEmpty()
			a.SetKeyStrindex(4)
			dic.StringTable().Append("process.executable.build_id.htlhash")
			a.Value().SetStr("600DCAFE4A110000F2BF38C493F5FB92")
			a = dic.AttributeTable().AppendEmpty()
			a.SetKeyStrindex(5)
			dic.StringTable().Append("profile.frame.type")
			a.Value().SetStr("native")
			a = dic.AttributeTable().AppendEmpty()
			a.SetKeyStrindex(6)
			dic.StringTable().Append("host.id")
			a.Value().SetStr("localhost")

			profile.AttributeIndices().Append(2)

			sample := profile.Samples().AppendEmpty()
			sample.TimestampsUnixNano().Append(0)

			stack := dic.StackTable().AppendEmpty()
			stack.LocationIndices().Append(0)

			m := dic.MappingTable().AppendEmpty()
			m.AttributeIndices().Append(0)

			l := dic.LocationTable().AppendEmpty()
			l.SetMappingIndex(0)
			l.SetAddress(111)
			l.AttributeIndices().Append(1)

			ctx := client.NewContext(t.Context(), client.Info{Metadata: metadata})
			mustSendProfilesWithCtx(ctx, t, exporter, profiles)
		}

		sendProfiles()
		sendProfiles()
		rec.WaitItems(5) // 5 profile documents are expected in total
	})
}

func newTestTracesExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Traces {
	exp := newUnstartedTestTracesExporter(t, url, fns...)
	err := exp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context())))
	})
	return exp
}

func newUnstartedTestTracesExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Traces {
	f := NewFactory()
	cfg := withDefaultConfig(append([]func(*Config){func(cfg *Config) {
		cfg.Endpoints = []string{url}
		cfg.QueueBatchConfig.Get().NumConsumers = 1
		// Batch is configured by default so we can directly edit flush timeout
		cfg.QueueBatchConfig.Get().Batch.Get().FlushTimeout = 10 * time.Millisecond
	}}, fns...)...)
	require.NoError(t, xconfmap.Validate(cfg))
	exp, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	return exp
}

func newTestProfilesExporter(t *testing.T, url string, fns ...func(*Config)) xexporter.Profiles {
	exp := newUnstartedTestProfilesExporter(t, url, fns...)
	err := exp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background())) //nolint:usetesting
	})
	return exp
}

func newUnstartedTestProfilesExporter(t *testing.T, url string, fns ...func(*Config)) xexporter.Profiles {
	f := NewFactory().(xexporter.Factory)
	cfg := withDefaultConfig(append([]func(*Config){func(cfg *Config) {
		cfg.Endpoints = []string{url}
		cfg.QueueBatchConfig.Get().NumConsumers = 1
		// Batch is configured by default so we can directly edit flush timeout
		cfg.QueueBatchConfig.Get().Batch.Get().FlushTimeout = 10 * time.Millisecond
	}}, fns...)...)
	require.NoError(t, xconfmap.Validate(cfg))
	exp, err := f.CreateProfiles(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	return exp
}

func newTestMetricsExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Metrics {
	exp := newUnstartedTestMetricsExporter(t, url, fns...)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background())) //nolint:usetesting
	})
	return exp
}

func newUnstartedTestMetricsExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Metrics {
	f := NewFactory()
	cfg := withDefaultConfig(append([]func(*Config){func(cfg *Config) {
		cfg.Endpoints = []string{url}
		cfg.QueueBatchConfig.Get().NumConsumers = 1
		// Batch is configured by default so we can directly edit flush timeout
		cfg.QueueBatchConfig.Get().Batch.Get().FlushTimeout = 10 * time.Millisecond
	}}, fns...)...)
	require.NoError(t, xconfmap.Validate(cfg))
	exp, err := f.CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	return exp
}

func newTestLogsExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Logs {
	exp := newUnstartedTestLogsExporter(t, url, fns...)
	err := exp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.Background())) //nolint:usetesting
	})
	return exp
}

func newUnstartedTestLogsExporter(t *testing.T, url string, fns ...func(*Config)) exporter.Logs {
	f := NewFactory()
	cfg := withDefaultConfig(append([]func(*Config){func(cfg *Config) {
		cfg.Endpoints = []string{url}
		cfg.QueueBatchConfig.Get().NumConsumers = 1
		// Batch is defined as default configuration
		cfg.QueueBatchConfig.Get().Batch.Get().FlushTimeout = 10 * time.Millisecond
	}}, fns...)...)
	require.NoError(t, xconfmap.Validate(cfg))
	exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
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
	mustSendLogsWithCtx(t.Context(), t, exporter, logs)
}

func mustSendLogsWithCtx(
	ctx context.Context,
	t *testing.T,
	exporter exporter.Logs,
	logs plog.Logs,
) {
	if !exporter.Capabilities().MutatesData {
		logs.MarkReadOnly()
	}
	err := exporter.ConsumeLogs(ctx, logs)
	require.NoError(t, err)
}

func mustSendMetricSumDataPoints(t *testing.T, exporter exporter.Metrics, dataPoints ...pmetric.NumberDataPoint) {
	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetEmptySum()
	metric.SetName("sum")
	for _, dataPoint := range dataPoints {
		dataPoint.CopyTo(metric.Sum().DataPoints().AppendEmpty())
	}
	mustSendMetrics(t, exporter, metrics)
}

func mustSendMetricGaugeDataPoints(t *testing.T, exporter exporter.Metrics, dataPoints ...pmetric.NumberDataPoint) {
	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetEmptyGauge()
	metric.SetName("gauge")
	for _, dataPoint := range dataPoints {
		dataPoint.CopyTo(metric.Gauge().DataPoints().AppendEmpty())
	}
	mustSendMetrics(t, exporter, metrics)
}

func mustSendMetrics(t *testing.T, exporter exporter.Metrics, metrics pmetric.Metrics) {
	mustSendMetricsWithCtx(t.Context(), t, exporter, metrics)
}

func mustSendMetricsWithCtx(
	ctx context.Context,
	t *testing.T,
	exporter exporter.Metrics,
	metrics pmetric.Metrics,
) {
	if !exporter.Capabilities().MutatesData {
		metrics.MarkReadOnly()
	}
	err := exporter.ConsumeMetrics(ctx, metrics)
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
	mustSendTracesWithCtx(t.Context(), t, exporter, traces)
}

func mustSendTracesWithCtx(
	ctx context.Context,
	t *testing.T,
	exporter exporter.Traces,
	traces ptrace.Traces,
) {
	if !exporter.Capabilities().MutatesData {
		traces.MarkReadOnly()
	}
	err := exporter.ConsumeTraces(ctx, traces)
	require.NoError(t, err)
}

func mustSendProfilesWithCtx(
	ctx context.Context,
	t *testing.T,
	exporter xexporter.Profiles,
	profiles pprofile.Profiles,
) {
	if !exporter.Capabilities().MutatesData {
		profiles.MarkReadOnly()
	}
	err := exporter.ConsumeProfiles(ctx, profiles)
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

var (
	_ extension.Extension      = (*mockAuthClient)(nil)
	_ extensionauth.HTTPClient = (*mockAuthClient)(nil)
)

type mockAuthClient struct {
	component.StartFunc
	component.ShutdownFunc
	extensionauth.ClientRoundTripperFunc
}

func newMockAuthClient(f func(*http.Request) (*http.Response, error)) *mockAuthClient {
	return &mockAuthClient{ClientRoundTripperFunc: func(http.RoundTripper) (http.RoundTripper, error) {
		return roundTripperFunc(f), nil
	}}
}

func actionJSONToIndex(t *testing.T, actionJSON json.RawMessage) string {
	t.Helper()
	return actionGetValue(t, actionJSON, "_index")
}

func actionJSONToID(t *testing.T, actionJSON json.RawMessage) string {
	t.Helper()
	return actionGetValue(t, actionJSON, "_id")
}

func actionJSONToPipeline(t *testing.T, actionJSON json.RawMessage) string {
	t.Helper()
	return actionGetValue(t, actionJSON, "pipeline")
}

// actionGetValue assumes the actionJSON is an object that has a key
// of create whose value is another object and target represents one
// of the inner keys.  The value of the inner key must be a string.
func actionGetValue(t *testing.T, actionJSON json.RawMessage, target string) string {
	t.Helper()
	a := map[string]any{}

	err := json.Unmarshal(actionJSON, &a)
	require.NoErrorf(t, err, "error unmarshalling action: %s", err)

	create, prs := a["create"]
	require.Truef(t, prs, "create was not present in action")

	createMap, ok := create.(map[string]any)
	require.True(t, ok, "create was not a map[string]interface{}")

	v, prs := createMap[target]
	require.Truef(t, prs, "%s was not present in action.create", target)

	vString, ok := v.(string)
	require.True(t, ok, "the type of action.create.%s was not string", target)
	return vString
}
