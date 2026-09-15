// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

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

				require.NoError(t, input.consume(t.Context(), exporter))
				require.Len(t, client.uploads["1.json"], 1)
				if len(counts) == 3 {
					require.Len(t, client.uploads, 2)
					require.Len(t, client.uploads["2.json"], 1)
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

			require.NoError(t, input.consume(t.Context(), exporter))
			require.Len(t, client.uploads, 1)
			require.Len(t, client.uploads[input.failingTemplate], 1)
			assert.Equal(t, 1, observed.FilterMessage(
				"Failed to execute blob name template, using default blob name format",
			).Len())
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
}

func newPartitionSignalInput(signal pipeline.Signal, counts []int) partitionSignalInput {
	var input partitionSignalInput
	switch signal {
	case pipeline.SignalLogs:
		logs := plog.NewLogs()
		for _, count := range counts {
			resource := logs.ResourceLogs().AppendEmpty()
			input.attributes = append(input.attributes, resource.Resource().Attributes())
			records := resource.ScopeLogs().AppendEmpty().LogRecords()
			for range count {
				records.AppendEmpty().Body().SetStr("log")
			}
		}
		input.countTemplate = "{{ .LogRecordCount }}.json"
		input.failingTemplate = `{{ index (getResourceLogAttr . 0 "parts") 1 }}.json`
		input.consume = func(ctx context.Context, exporter *azureBlobExporter) error {
			return exporter.ConsumeLogs(ctx, logs)
		}
	case pipeline.SignalMetrics:
		metrics := pmetric.NewMetrics()
		for _, count := range counts {
			resource := metrics.ResourceMetrics().AppendEmpty()
			input.attributes = append(input.attributes, resource.Resource().Attributes())
			records := resource.ScopeMetrics().AppendEmpty().Metrics()
			for range count {
				metric := records.AppendEmpty()
				metric.SetName("metric")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
			}
		}
		input.countTemplate = "{{ .MetricCount }}.json"
		input.failingTemplate = `{{ index (getResourceMetricAttr . 0 "parts") 1 }}.json`
		input.consume = func(ctx context.Context, exporter *azureBlobExporter) error {
			return exporter.ConsumeMetrics(ctx, metrics)
		}
	case pipeline.SignalTraces:
		traces := ptrace.NewTraces()
		for _, count := range counts {
			resource := traces.ResourceSpans().AppendEmpty()
			input.attributes = append(input.attributes, resource.Resource().Attributes())
			records := resource.ScopeSpans().AppendEmpty().Spans()
			for range count {
				records.AppendEmpty().SetName("span")
			}
		}
		input.countTemplate = "{{ .SpanCount }}.json"
		input.failingTemplate = `{{ index (getResourceSpanAttr . 0 "parts") 1 }}.json`
		input.consume = func(ctx context.Context, exporter *azureBlobExporter) error {
			return exporter.ConsumeTraces(ctx, traces)
		}
	}
	return input
}
