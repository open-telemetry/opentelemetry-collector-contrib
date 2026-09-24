// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// TestPartitionUploadsGroupsSequentially verifies the exporter's scheduling
// contract: groups within one export request are uploaded one at a time, so
// concurrency is governed only by the sending queue's num_consumers. It runs
// inside a synctest bubble with a bubble-local gate and no exporterhelper queue,
// so synctest.Wait proves quiescence rather than relying on timing.
func TestPartitionUploadsGroupsSequentially(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)
		exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
		require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))

		started := make(chan string, 3)
		release := make(chan struct{})
		client := &mockAzBlobClient{url: "http://mock"}
		client.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				started <- args.String(2)
				<-release
			}).Return(nil)
		exporter.client = client

		done := make(chan error, 1)
		go func() {
			done <- exporter.ConsumeLogs(t.Context(), generateLogsWithActivities("a", "b", "c"))
		}()

		for _, want := range []string{"a.json", "b.json", "c.json"} {
			synctest.Wait()
			require.Len(t, started, 1, "a second upload must not start while one is in flight")
			require.Equal(t, want, <-started)
			release <- struct{}{}
		}
		synctest.Wait()
		require.NoError(t, <-done)
		client.AssertNumberOfCalls(t, "AppendBlock", 3)
	})
}

// TestPartitionGroupsUseOwnUploadTime verifies that each blob written by one
// export request resolves time-based name segments when that blob is
// uploaded, so a group uploaded after a time boundary is not written to the
// previous time bucket.
func TestPartitionGroupsUseOwnUploadTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}/15`, true)
		cfg.BlobNameFormat.TimeParserEnabled = true
		// Parse only the hour, not the resource-specific prefix.
		cfg.BlobNameFormat.TimeParserRanges = []string{"2-4"}
		exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
		require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))

		var names []string
		client := &mockAzBlobClient{url: "http://mock"}
		client.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				names = append(names, args.String(2))
				// Each upload advances the bubble's fake clock by an hour.
				time.Sleep(time.Hour)
			}).Return(nil)
		exporter.client = client

		start := time.Now()
		require.NoError(t, exporter.ConsumeLogs(t.Context(), generateLogsWithActivities("a", "b")))

		assert.Equal(t, []string{
			"a/" + start.Format("15"),
			"b/" + start.Add(time.Hour).Format("15"),
		}, names)
	})
}

func TestPartitionCancellationPreservesUnsentLogs(t *testing.T) {
	for _, tc := range []struct {
		name          string
		cancelBefore  bool
		failUpload    bool
		wantCalls     int
		wantRetryLogs []string
	}{
		{"before_upload", true, false, 0, []string{"log for a", "log for b", "log for c"}},
		{"after_success", false, false, 1, []string{"log for b", "log for c"}},
		{"during_failure", false, true, 1, []string{"log for a", "log for b", "log for c"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)
			exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
			require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			client := &mockAzBlobClient{url: "http://mock"}
			var uploadErr error
			if tc.failUpload {
				uploadErr = context.Canceled
			}
			client.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).
				Run(func(mock.Arguments) { cancel() }).
				Return(uploadErr)
			exporter.client = client
			if tc.cancelBefore {
				cancel()
			}

			logs := generateLogsWithActivities("a", "b", "c")
			err := exporter.ConsumeLogs(ctx, logs)
			require.ErrorIs(t, err, context.Canceled)
			client.AssertNumberOfCalls(t, "AppendBlock", tc.wantCalls)
			retryLogs := logs
			if tc.cancelBefore {
				require.Equal(t, context.Canceled, err, "a whole-request error leaves all input available for retry")
			} else {
				var partial consumererror.Logs
				require.ErrorAs(t, err, &partial)
				retryLogs = partial.Data()
			}
			data, err := (&plog.JSONMarshaler{}).MarshalLogs(retryLogs)
			require.NoError(t, err)
			assert.Equal(t, tc.wantRetryLogs, uploadedLogBodies(t, [][]byte{data}))
			assert.Equal(t, 3, logs.LogRecordCount(), "partitioning must not mutate the input")

			retryClient := newRecordingAzBlobClient(nil)
			exporter.client = retryClient
			require.NoError(t, exporter.ConsumeLogs(t.Context(), retryLogs))
			assert.Len(t, retryClient.uploads, len(tc.wantRetryLogs))
		})
	}
}

func TestConsumeCanceledBeforePartitioning(t *testing.T) {
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalMetrics, pipeline.SignalTraces} {
		t.Run(signal.String(), func(t *testing.T) {
			input := newPartitionSignalInput(signal, []int{1, 1, 2})
			cfg := newPartitionTestConfig("logs.json", true)
			exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), signal)
			require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
			client := newRecordingAzBlobClient(nil)
			exporter.client = client
			var renders int
			tmpl, err := template.New("count_renders").Funcs(template.FuncMap{
				"countRender": func() string {
					renders++
					return "logs.json"
				},
			}).Parse("{{ countRender }}")
			require.NoError(t, err)
			exporter.blobNameTemplate = &blobNameTemplate{logs: tmpl, metrics: tmpl, traces: tmpl}
			before, err := input.marshal()
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			require.ErrorIs(t, input.consume(ctx, exporter), context.Canceled)
			assert.Zero(t, renders, "canceled requests must not evaluate resource templates")
			assert.Empty(t, client.uploads)
			after, err := input.marshal()
			require.NoError(t, err)
			assert.JSONEq(t, string(before), string(after))
		})
	}
}

func TestPartitionKeepsRenderedName(t *testing.T) {
	cfg := newPartitionTestConfig("{{ .LogRecordCount }}.json", true)
	cfg.AppendBlob.Enabled = false
	exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
	client := newRecordingAzBlobClient(nil)
	exporter.client = client

	logs := generateLogsWithActivities("a", "b", "c")
	logs.ResourceLogs().At(2).ScopeLogs().At(0).LogRecords().AppendEmpty().
		Body().SetStr("second log for c")
	require.NoError(t, exporter.ConsumeLogs(t.Context(), logs))

	// The singleton counts are 1, 1, 2. Combining the first two resources must
	// not change their destination from 1.json to 2.json.
	assert.Len(t, client.uploads, 2, "expected two distinct blob destinations")
	assert.Len(t, client.uploads["1.json"], 1)
	assert.Len(t, client.uploads["2.json"], 1)
	assert.Equal(t, []string{"log for a", "log for b"},
		uploadedLogBodies(t, client.uploads["1.json"]))
	assert.Equal(t, []string{"log for c", "second log for c"},
		uploadedLogBodies(t, client.uploads["2.json"]))
}

func TestPartitionLaterResourceTemplateErrorFallsBack(t *testing.T) {
	const nameTemplate = `{{ index (getResourceLogAttr . 0 "parts") 1 }}.json`
	cfg := newPartitionTestConfig(nameTemplate, true)
	core, observed := observer.New(zap.WarnLevel)
	exporter := newAzureBlobExporter(cfg, zap.New(core), pipeline.SignalLogs)
	require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
	client := newRecordingAzBlobClient(nil)
	exporter.client = client

	logs := generateLogsWithActivities("a", "b")
	first := logs.ResourceLogs().At(0).Resource().Attributes().PutEmptySlice("parts")
	first.AppendEmpty().SetStr("prefix")
	first.AppendEmpty().SetStr("tenant-a")
	second := logs.ResourceLogs().At(1).Resource().Attributes().PutEmptySlice("parts")
	second.AppendEmpty().SetStr("prefix")
	require.NoError(t, exporter.ConsumeLogs(t.Context(), logs))

	// Only the second resource fails. Retrying the template on the original
	// payload would hide that error by reading the first resource again.
	assert.Len(t, client.uploads, 1)
	assert.Empty(t, client.uploads["tenant-a.json"])
	assert.Len(t, client.uploads[nameTemplate], 1)
	assert.Equal(t, []string{"log for a", "log for b"},
		uploadedLogBodies(t, client.uploads[nameTemplate]))
	assert.Equal(t, 1, observed.FilterMessage(
		"Failed to execute blob name template, using default blob name format",
	).Len())
}

func TestPartitionRenderedNamesAcrossSignals(t *testing.T) {
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalMetrics, pipeline.SignalTraces} {
		for _, counts := range [][]int{{1, 1, 2}, {1, 1}} {
			t.Run(fmt.Sprintf("%s/%v", signal, counts), func(t *testing.T) {
				input := newPartitionSignalInput(signal, counts)
				cfg := newPartitionTestConfig(input.countTemplate, true)
				cfg.BlobNameFormat.MetricsFormat = input.countTemplate
				cfg.BlobNameFormat.TracesFormat = input.countTemplate
				cfg.AppendBlob.Enabled = false
				exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), signal)
				require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
				client := newRecordingAzBlobClient(nil)
				exporter.client = client

				before, err := input.marshal()
				require.NoError(t, err)
				wantFirst, err := input.marshal(0, 1)
				require.NoError(t, err)
				require.NoError(t, input.consume(t.Context(), exporter))
				after, err := input.marshal()
				require.NoError(t, err)
				require.JSONEq(t, string(before), string(after), "export must not mutate the input")
				require.Len(t, client.uploads["1.json"], 1)
				assert.JSONEq(t, string(wantFirst), string(client.uploads["1.json"][0]))
				if len(counts) == 3 {
					require.Len(t, client.uploads, 2)
					require.Len(t, client.uploads["2.json"], 1)
					wantSecond, err := input.marshal(2)
					require.NoError(t, err)
					assert.JSONEq(t, string(wantSecond), string(client.uploads["2.json"][0]))
				} else {
					require.Len(t, client.uploads, 1)
				}
			})
		}
	}
}

func TestPartitionFallbackAcrossSignals(t *testing.T) {
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalMetrics, pipeline.SignalTraces} {
		t.Run(signal.String(), func(t *testing.T) {
			input := newPartitionSignalInput(signal, []int{1, 1})
			first := input.attributes[0].PutEmptySlice("parts")
			first.AppendEmpty().SetStr("prefix")
			first.AppendEmpty().SetStr("tenant-a")
			input.attributes[1].PutEmptySlice("parts").AppendEmpty().SetStr("prefix")
			cfg := newPartitionTestConfig(input.failingTemplate, true)
			cfg.BlobNameFormat.MetricsFormat = input.failingTemplate
			cfg.BlobNameFormat.TracesFormat = input.failingTemplate
			core, observed := observer.New(zap.WarnLevel)
			exporter := newAzureBlobExporter(cfg, zap.New(core), signal)
			require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
			client := newRecordingAzBlobClient(nil)
			exporter.client = client

			before, err := input.marshal()
			require.NoError(t, err)
			require.NoError(t, input.consume(t.Context(), exporter))
			after, err := input.marshal()
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after), "fallback must not mutate the input")
			require.Len(t, client.uploads, 1)
			require.Len(t, client.uploads[input.failingTemplate], 1)
			assert.JSONEq(t, string(before), string(client.uploads[input.failingTemplate][0]))
			assert.Equal(t, 1, observed.FilterMessage(
				"Failed to execute blob name template, using default blob name format",
			).Len())
		})
	}
}

func TestPartitionPartialFailureCarriesOnlyFailedData(t *testing.T) {
	for _, signal := range []pipeline.Signal{pipeline.SignalLogs, pipeline.SignalMetrics, pipeline.SignalTraces} {
		t.Run(signal.String(), func(t *testing.T) {
			input := newPartitionSignalInput(signal, []int{1, 2})
			cfg := newPartitionTestConfig(input.countTemplate, true)
			cfg.BlobNameFormat.MetricsFormat = input.countTemplate
			cfg.BlobNameFormat.TracesFormat = input.countTemplate
			exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), signal)
			require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
			client := newRecordingAzBlobClient(map[string]int{"2.json": 1})
			exporter.client = client

			consumeErr := input.consume(t.Context(), exporter)
			require.Error(t, consumeErr)

			// The error must carry only the failed resource, as the
			// exporterhelper retry sender extracts it via OnError.
			wantRetry, err := input.marshal(1)
			require.NoError(t, err)
			var gotRetry []byte
			switch signal {
			case pipeline.SignalLogs:
				var partial consumererror.Logs
				require.ErrorAs(t, consumeErr, &partial)
				gotRetry, err = (&plog.JSONMarshaler{}).MarshalLogs(partial.Data())
			case pipeline.SignalMetrics:
				var partial consumererror.Metrics
				require.ErrorAs(t, consumeErr, &partial)
				gotRetry, err = (&pmetric.JSONMarshaler{}).MarshalMetrics(partial.Data())
			case pipeline.SignalTraces:
				var partial consumererror.Traces
				require.ErrorAs(t, consumeErr, &partial)
				gotRetry, err = (&ptrace.JSONMarshaler{}).MarshalTraces(partial.Data())
			}
			require.NoError(t, err)
			assert.JSONEq(t, string(wantRetry), string(gotRetry))

			wantUploaded, err := input.marshal(0)
			require.NoError(t, err)
			require.Len(t, client.uploads["1.json"], 1)
			assert.JSONEq(t, string(wantUploaded), string(client.uploads["1.json"][0]))
			assert.Empty(t, client.uploads["2.json"])
		})
	}
}

func TestPartitionPreservesNameFormatting(t *testing.T) {
	for _, compression := range []configcompression.Type{"", configcompression.TypeGzip, configcompression.TypeZstd} {
		for _, serialBeforeExtension := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/before_extension=%t", compression, serialBeforeExtension), func(t *testing.T) {
				cfg := newPartitionTestConfig(`{{ .LogRecordCount }}/2006.json`, true)
				cfg.AppendBlob.Enabled = false
				cfg.Compression = compression
				cfg.BlobNameFormat.TimeParserEnabled = true
				// Parse only the year, not the resource-specific count prefix.
				cfg.BlobNameFormat.TimeParserRanges = []string{"2-6"}
				cfg.BlobNameFormat.SerialNumEnabled = true
				cfg.BlobNameFormat.SerialNumRange = 1
				cfg.BlobNameFormat.SerialNumBeforeExtension = serialBeforeExtension
				exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
				require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
				client := newRecordingAzBlobClient(nil)
				exporter.client = client
				input := newPartitionSignalInput(pipeline.SignalLogs, []int{1, 1, 2})
				startYear := time.Now().Format("2006")
				require.NoError(t, input.consume(t.Context(), exporter))
				yearPattern := "(" + startYear + "|" + time.Now().Format("2006") + ")"

				require.Len(t, client.uploads, 2)
				suffix := ""
				switch compression {
				case configcompression.TypeGzip:
					suffix = `\.gz`
				case configcompression.TypeZstd:
					suffix = `\.zst`
				}
				for name, payloads := range client.uploads {
					pattern := `^[12]/` + yearPattern + `\.json_0` + suffix + `$`
					if serialBeforeExtension {
						pattern = `^[12]/` + yearPattern + `_0\.json` + suffix + `$`
					}
					assert.Regexp(t, pattern, name)
					assert.Len(t, payloads, 1)
				}
			})
		}
	}
}

type partitionSignalInput struct {
	countTemplate   string
	failingTemplate string
	attributes      []pcommon.Map
	consume         func(context.Context, *azureBlobExporter) error
	marshal         func(resourceIndices ...int) ([]byte, error)
}

func newPartitionSignalInput(signal pipeline.Signal, counts []int) partitionSignalInput {
	var input partitionSignalInput
	switch signal {
	case pipeline.SignalLogs:
		logs := plog.NewLogs()
		for i, count := range counts {
			resource := logs.ResourceLogs().AppendEmpty()
			resource.Resource().Attributes().PutInt("resource-index", int64(i))
			input.attributes = append(input.attributes, resource.Resource().Attributes())
			records := resource.ScopeLogs().AppendEmpty().LogRecords()
			for j := range count {
				records.AppendEmpty().Body().SetStr(fmt.Sprintf("log-%d-%d", i, j))
			}
		}
		input.countTemplate = "{{ .LogRecordCount }}.json"
		input.failingTemplate = `{{ index (getResourceLogAttr . 0 "parts") 1 }}.json`
		input.consume = func(ctx context.Context, exporter *azureBlobExporter) error {
			return exporter.ConsumeLogs(ctx, logs)
		}
		input.marshal = func(indices ...int) ([]byte, error) {
			marshaler := plog.JSONMarshaler{}
			if len(indices) == 0 {
				return marshaler.MarshalLogs(logs)
			}
			selected := plog.NewLogs()
			for _, i := range indices {
				logs.ResourceLogs().At(i).CopyTo(selected.ResourceLogs().AppendEmpty())
			}
			return marshaler.MarshalLogs(selected)
		}
	case pipeline.SignalMetrics:
		metrics := pmetric.NewMetrics()
		for i, count := range counts {
			resource := metrics.ResourceMetrics().AppendEmpty()
			resource.Resource().Attributes().PutInt("resource-index", int64(i))
			input.attributes = append(input.attributes, resource.Resource().Attributes())
			records := resource.ScopeMetrics().AppendEmpty().Metrics()
			for j := range count {
				metric := records.AppendEmpty()
				metric.SetName(fmt.Sprintf("metric-%d-%d", i, j))
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(int64(i*100 + j))
			}
		}
		input.countTemplate = "{{ .MetricCount }}.json"
		input.failingTemplate = `{{ index (getResourceMetricAttr . 0 "parts") 1 }}.json`
		input.consume = func(ctx context.Context, exporter *azureBlobExporter) error {
			return exporter.ConsumeMetrics(ctx, metrics)
		}
		input.marshal = func(indices ...int) ([]byte, error) {
			marshaler := pmetric.JSONMarshaler{}
			if len(indices) == 0 {
				return marshaler.MarshalMetrics(metrics)
			}
			selected := pmetric.NewMetrics()
			for _, i := range indices {
				metrics.ResourceMetrics().At(i).CopyTo(selected.ResourceMetrics().AppendEmpty())
			}
			return marshaler.MarshalMetrics(selected)
		}
	case pipeline.SignalTraces:
		traces := ptrace.NewTraces()
		for i, count := range counts {
			resource := traces.ResourceSpans().AppendEmpty()
			resource.Resource().Attributes().PutInt("resource-index", int64(i))
			input.attributes = append(input.attributes, resource.Resource().Attributes())
			records := resource.ScopeSpans().AppendEmpty().Spans()
			for j := range count {
				records.AppendEmpty().SetName(fmt.Sprintf("span-%d-%d", i, j))
			}
		}
		input.countTemplate = "{{ .SpanCount }}.json"
		input.failingTemplate = `{{ index (getResourceSpanAttr . 0 "parts") 1 }}.json`
		input.consume = func(ctx context.Context, exporter *azureBlobExporter) error {
			return exporter.ConsumeTraces(ctx, traces)
		}
		input.marshal = func(indices ...int) ([]byte, error) {
			marshaler := ptrace.JSONMarshaler{}
			if len(indices) == 0 {
				return marshaler.MarshalTraces(traces)
			}
			selected := ptrace.NewTraces()
			for _, i := range indices {
				traces.ResourceSpans().At(i).CopyTo(selected.ResourceSpans().AppendEmpty())
			}
			return marshaler.MarshalTraces(selected)
		}
	}
	return input
}
