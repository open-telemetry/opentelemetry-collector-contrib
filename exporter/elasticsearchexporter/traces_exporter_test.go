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
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestTracesExporter_New(t *testing.T) {
	type validate func(*testing.T, *elasticsearchTracesExporter, error)

	success := func(t *testing.T, exporter *elasticsearchTracesExporter, err error) {
		require.Nil(t, err)
		require.NotNil(t, exporter)
	}
	successWithInternalModel := func(expectedModel *encodeModel) validate {
		return func(t *testing.T, exporter *elasticsearchTracesExporter, err error) {
			assert.Nil(t, err)
			assert.EqualValues(t, expectedModel, exporter.model)
		}
	}

	failWith := func(want error) validate {
		return func(t *testing.T, exporter *elasticsearchTracesExporter, err error) {
			require.Nil(t, exporter)
			require.NotNil(t, err)
			if !errors.Is(err, want) {
				t.Fatalf("Expected error '%v', but got '%v'", want, err)
			}
		}
	}

	failWithMessage := func(msg string) validate {
		return func(t *testing.T, exporter *elasticsearchTracesExporter, err error) {
			require.Nil(t, exporter)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), msg)
		}
	}

	tests := map[string]struct {
		config *Config
		want   validate
		env    map[string]string
	}{
		"no endpoint": {
			config: withDefaultConfig(),
			want:   failWith(errConfigNoEndpoint),
		},
		"create from default config with ELASTICSEARCH_URL environment variable": {
			config: withDefaultConfig(),
			want:   success,
			env:    map[string]string{defaultElasticsearchEnvName: "localhost:9200"},
		},
		"create from default with endpoints": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"test:9200"}
			}),
			want: success,
		},
		"create with cloudid": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.CloudID = "foo:YmFyLmNsb3VkLmVzLmlvJGFiYzEyMyRkZWY0NTY="
			}),
			want: success,
		},
		"create with invalid cloudid": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.CloudID = "invalid"
			}),
			want: failWithMessage("cannot parse CloudID"),
		},
		"fail if endpoint and cloudid are set": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"test:9200"}
				cfg.CloudID = "foo:YmFyLmNsb3VkLmVzLmlvJGFiYzEyMyRkZWY0NTY="
			}),
			want: failWithMessage("Addresses and CloudID are set"),
		},
		"create with custom dedup and dedot values": {
			config: withDefaultConfig(func(cfg *Config) {
				cfg.Endpoints = []string{"test:9200"}
				cfg.Mapping.Dedot = false
				cfg.Mapping.Dedup = true
			}),
			want: successWithInternalModel(&encodeModel{dedot: false, dedup: true}),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			env := test.env
			if len(env) == 0 {
				env = map[string]string{defaultElasticsearchEnvName: ""}
			}

			for k, v := range env {
				t.Setenv(k, v)
			}

			exporter, err := newTracesExporter(zap.NewNop(), test.config)
			if exporter != nil {
				defer func() {
					require.NoError(t, exporter.Shutdown(context.TODO()))
				}()
			}

			test.want(t, exporter, err)
		})
	}
}

func TestExporter_PushTraceRecord(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/14759")
	}

	t.Run("publish with success", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL)
		mustSendTraces(t, exporter, `{"message": "test1"}`)
		mustSendTraces(t, exporter, `{"message": "test1"}`)

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
			assert.Nil(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.Nil(t, err)

			create := jsonVal["create"].(map[string]any)

			expected := fmt.Sprintf("%s%s%s", prefix, index, suffix)
			assert.Equal(t, expected, create["_index"].(string))

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.TracesIndex = index
			cfg.TracesDynamicIndex.Enabled = true
		})

		mustSendTracesWithAttributes(t, exporter,
			map[string]string{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			map[string]string{
				indexPrefix: prefix,
			},
		)

		rec.WaitItems(1)
	})

	t.Run("publish with logstash format index", func(t *testing.T) {
		var defaultCfg Config

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.Nil(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.Nil(t, err)

			create := jsonVal["create"].(map[string]any)

			assert.Equal(t, strings.Contains(create["_index"].(string), defaultCfg.TracesIndex), true)

			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.LogstashFormat.Enabled = true
			cfg.TracesIndex = "not-used-index"
			defaultCfg = *cfg
		})

		mustSendTracesWithAttributes(t, exporter, nil, nil)

		rec.WaitItems(1)
	})

	t.Run("publish with logstash format index and dynamic index enabled ", func(t *testing.T) {
		var (
			prefix = "resprefix-"
			suffix = "-attrsuffix"
			index  = "someindex"
		)

		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)

			data, err := docs[0].Action.MarshalJSON()
			assert.Nil(t, err)

			jsonVal := map[string]any{}
			err = json.Unmarshal(data, &jsonVal)
			assert.Nil(t, err)

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

		mustSendTracesWithAttributes(t, exporter,
			map[string]string{
				indexPrefix: "attrprefix-",
				indexSuffix: suffix,
			},
			map[string]string{
				indexPrefix: prefix,
			},
		)
		rec.WaitItems(1)
	})

	t.Run("retry http request", func(t *testing.T) {
		failures := 0
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			if failures == 0 {
				failures++
				return nil, &httpTestError{message: "oops"}
			}

			rec.Record(docs)
			return itemsAllOK(docs)
		})

		exporter := newTestTracesExporter(t, server.URL)
		mustSendTraces(t, exporter, `{"message": "test1"}`)

		rec.WaitItems(1)
	})

	t.Run("no retry", func(t *testing.T) {
		configurations := map[string]func(string) *Config{
			"max_requests limited": withTestExporterConfig(func(cfg *Config) {
				cfg.Retry.MaxRequests = 1
				cfg.Retry.InitialInterval = 1 * time.Millisecond
				cfg.Retry.MaxInterval = 10 * time.Millisecond
			}),
			"retry.enabled is false": withTestExporterConfig(func(cfg *Config) {
				cfg.Retry.Enabled = false
				cfg.Retry.MaxRequests = 10
				cfg.Retry.InitialInterval = 1 * time.Millisecond
				cfg.Retry.MaxInterval = 10 * time.Millisecond
			}),
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
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				for name, configurer := range configurations {
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						attempts := &atomic.Int64{}
						server := newESTestServer(t, handler(attempts))

						testConfig := configurer(server.URL)
						exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) { *cfg = *testConfig })
						mustSendTraces(t, exporter, `{"message": "test1"}`)

						time.Sleep(200 * time.Millisecond)
						assert.Equal(t, int64(1), attempts.Load())
					})
				}
			})
		}
	})

	t.Run("do not retry invalid request", func(t *testing.T) {
		attempts := &atomic.Int64{}
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			attempts.Add(1)
			return nil, &httpTestError{message: "oops", status: http.StatusBadRequest}
		})

		exporter := newTestTracesExporter(t, server.URL)
		mustSendTraces(t, exporter, `{"message": "test1"}`)

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

		exporter := newTestTracesExporter(t, server.URL)
		mustSendTraces(t, exporter, `{"message": "test1"}`)

		rec.WaitItems(1)
	})

	t.Run("do not retry bad item", func(t *testing.T) {
		attempts := &atomic.Int64{}
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			attempts.Add(1)
			return itemsReportStatus(docs, http.StatusBadRequest)
		})

		exporter := newTestTracesExporter(t, server.URL)
		mustSendTraces(t, exporter, `{"message": "test1"}`)

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

				var idxInfo struct{ Idx int }
				if err := json.Unmarshal(doc.Document, &idxInfo); err != nil {
					panic(err)
				}

				if idxInfo.Idx == retryIdx {
					if attempts[retryIdx] == 0 {
						resp[i].Status = http.StatusTooManyRequests
					} else {
						defer wg.Done()
					}
				}
				attempts[idxInfo.Idx]++
			}
			return resp, nil
		})

		exporter := newTestTracesExporter(t, server.URL, func(cfg *Config) {
			cfg.Flush.Interval = 50 * time.Millisecond
			cfg.Retry.InitialInterval = 1 * time.Millisecond
			cfg.Retry.MaxInterval = 10 * time.Millisecond
		})
		mustSendTraces(t, exporter, `{"message": "test1", "idx": 0}`)
		mustSendTraces(t, exporter, `{"message": "test2", "idx": 1}`)
		mustSendTraces(t, exporter, `{"message": "test3", "idx": 2}`)

		wg.Wait() // <- this blocks forever if the trace is not retried

		assert.Equal(t, [3]int{1, 2, 1}, attempts)
	})
}
func newTestLogsExporter(t *testing.T, url string, fns ...func(*Config)) *elasticsearchLogsExporter {
	exporter, err := newLogsExporter(zaptest.NewLogger(t), withTestTracesExporterConfig(fns...)(url))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(context.TODO()))
	})
	return exporter
}

func newTestTracesExporter(t *testing.T, url string, fns ...func(*Config)) *elasticsearchTracesExporter {
	exporter, err := newTracesExporter(zaptest.NewLogger(t), withTestTracesExporterConfig(fns...)(url))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(context.TODO()))
	})
	return exporter
}

func withTestTracesExporterConfig(fns ...func(*Config)) func(string) *Config {
	return func(url string) *Config {
		var configMods []func(*Config)
		configMods = append(configMods, func(cfg *Config) {
			cfg.Endpoints = []string{url}
			cfg.NumWorkers = 1
			cfg.Flush.Interval = 10 * time.Millisecond
		})
		configMods = append(configMods, fns...)
		return withDefaultConfig(configMods...)
	}
}

func mustSendTraces(t *testing.T, exporter *elasticsearchTracesExporter, contents string) {
	err := pushDocuments(context.TODO(), zap.L(), exporter.index, []byte(contents), exporter.bulkIndexer, exporter.maxAttempts)
	require.NoError(t, err)
}

// send trace with span & resource attributes
func mustSendTracesWithAttributes(t *testing.T, exporter *elasticsearchTracesExporter, attrMp map[string]string, resMp map[string]string) {
	traces := newTracesWithAttributeAndResourceMap(attrMp, resMp)
	resSpans := traces.ResourceSpans().At(0)
	span := resSpans.ScopeSpans().At(0).Spans().At(0)
	scope := resSpans.ScopeSpans().At(0).Scope()

	err := exporter.pushTraceRecord(context.TODO(), resSpans.Resource(), span, scope)
	require.NoError(t, err)
}
