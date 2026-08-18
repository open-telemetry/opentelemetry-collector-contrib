// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/appendblob"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestNewExporter(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
		},
		Container: TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
			LogsFormat:     "2006/01/02/logs_15_04_05.json",
			TracesFormat:   "2006/01/02/traces_15_04_05.json",
			SerialNumRange: 10000,
			Params:         map[string]string{},
		},
		FormatType: "json",
		Encodings:  Encodings{},
	}

	me := newAzureBlobExporter(c, logger, pipeline.SignalMetrics)
	assert.NotNil(t, me)
	assert.NoError(t, me.start(t.Context(), componenttest.NewNopHost()))

	le := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NotNil(t, le)
	assert.NoError(t, le.start(t.Context(), componenttest.NewNopHost()))

	te := newAzureBlobExporter(c, logger, pipeline.SignalTraces)
	assert.NotNil(t, te)
	assert.NoError(t, te.start(t.Context(), componenttest.NewNopHost()))
}

func TestExporterConsumeTelemetry(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "sp"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "smi"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "umi"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "conn-string"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String()+"-metrics", func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			azureBlobExporter := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			assert.NoError(t, azureBlobExporter.start(t.Context(), componenttest.NewNopHost()))
			azureBlobExporter.client = getMockAzBlobClient()

			metrics := testdata.GenerateMetricsTwoMetrics()
			assert.NoError(t, azureBlobExporter.ConsumeMetrics(t.Context(), metrics))
		})
		t.Run(tt.id.String()+"-logs", func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			azureBlobExporter := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			assert.NoError(t, azureBlobExporter.start(t.Context(), componenttest.NewNopHost()))
			azureBlobExporter.client = getMockAzBlobClient()

			logs := testdata.GenerateLogsTwoLogRecordsSameResource()
			assert.NoError(t, azureBlobExporter.ConsumeLogs(t.Context(), logs))
		})
		t.Run(tt.id.String()+"-traces", func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			azureBlobExporter := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			assert.NoError(t, azureBlobExporter.start(t.Context(), componenttest.NewNopHost()))
			azureBlobExporter.client = getMockAzBlobClient()

			traces := testdata.GenerateTracesTwoSpansSameResource()
			assert.NoError(t, azureBlobExporter.ConsumeTraces(t.Context(), traces))
		})
	}
}

func TestGenerateBlobName(t *testing.T) {
	t.Parallel()

	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     "2006/01/02/metrics_15_04_05.json",
			LogsFormat:        "2006/01/02/logs_15_04_05.json",
			TracesFormat:      "2006/01/02/traces_15_04_05.json",
			SerialNumEnabled:  true,
			SerialNumRange:    10000,
			TimeParserEnabled: true,
			Params:            map[string]string{},
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	now := time.Now()
	metricsBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(metricsBlobName, now.Format(c.BlobNameFormat.MetricsFormat)))

	logsBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalLogs, nil)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(logsBlobName, now.Format(c.BlobNameFormat.LogsFormat)))

	tracesBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalTraces, nil)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(tracesBlobName, now.Format(c.BlobNameFormat.TracesFormat)))
}

func TestGenerateBlobNameTimezoneSpecificLocation(t *testing.T) {
	t.Parallel()

	const tzName = "America/New_York"
	loc, err := time.LoadLocation(tzName)
	require.NoError(t, err)

	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     "2006/01/02/metrics_15_04_05.json",
			LogsFormat:        "2006/01/02/logs_15_04_05.json",
			TracesFormat:      "2006/01/02/traces_15_04_05.json",
			SerialNumEnabled:  true,
			SerialNumRange:    10000,
			Params:            map[string]string{},
			TimeParserEnabled: true,
			Timezone:          tzName,
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	ae.timeLocation = loc

	before := time.Now().In(loc)
	metricsBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	after := time.Now().In(loc)

	lastUnderscore := strings.LastIndex(metricsBlobName, "_")
	require.NotEqual(t, -1, lastUnderscore, "expected serial number separator in blob name")
	prefix := metricsBlobName[:lastUnderscore]

	parsed, err := time.ParseInLocation(c.BlobNameFormat.MetricsFormat, prefix, loc)
	require.NoError(t, err)

	assert.False(t, parsed.Before(before.Add(-time.Second)))
	assert.False(t, parsed.After(after.Add(time.Second)))
}

func TestGenerateBlobNameSerialNumBefore(t *testing.T) {
	t.Parallel()

	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:            "2006/01/02/metrics_15_04_05.json",
			LogsFormat:               "2006/01/02/logs_15_04_05.json",
			TracesFormat:             "2006/01/02/traces_15_04_05", // no extension
			SerialNumEnabled:         true,
			SerialNumRange:           10000,
			SerialNumBeforeExtension: true,
			TimeParserEnabled:        true,
			Params:                   map[string]string{},
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	assertFormat := func(blobName, format string) {
		ext := filepath.Ext(format)
		formatWithoutExt := strings.TrimSuffix(format, ext)
		assert.True(t, strings.HasPrefix(blobName, formatWithoutExt))
		assert.True(t, strings.HasSuffix(blobName, ext))
	}

	now := time.Now()
	metricsBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	assert.NoError(t, err)
	assertFormat(metricsBlobName, now.Format(c.BlobNameFormat.MetricsFormat))

	logsBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalLogs, nil)
	assert.NoError(t, err)
	assertFormat(logsBlobName, now.Format(c.BlobNameFormat.LogsFormat))

	tracesBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalTraces, nil)
	assert.NoError(t, err)
	assertFormat(tracesBlobName, now.Format(c.BlobNameFormat.TracesFormat))
}

func TestGenerateBlobNameWithTemplate(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	c := cfg.(*Config)
	c.BlobNameFormat = BlobNameFormat{
		TemplateEnabled: true,
		MetricsFormat:   `{{ getResourceMetricAttr . 0 "service.name" }}/2006/01/02/metrics.json`,
		LogsFormat:      `{{ getScopeLogAttr . 0 0 "scope.name" }}/2006/01/02/logs.json`,
		TracesFormat:    `{{ (getSpan . 0 0 0).Name }}/2006/01/02/traces.json`,
		SerialNumRange:  10000,
	}
	c.Auth.ConnectionString = "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net"

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	err := ae.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Test metrics
	metrics := testdata.GenerateMetricsTwoMetrics()
	metrics.ResourceMetrics().At(0).Resource().Attributes().PutStr("service.name", "test-metrics-service")
	metricsBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, metrics)
	assert.NoError(t, err)
	assert.Contains(t, metricsBlobName, "test-metrics-service")
	assert.Contains(t, metricsBlobName, "metrics.json")

	// Test logs
	logs := testdata.GenerateLogsTwoLogRecordsSameResource()
	logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("scope.name", "test-scope")
	logsBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalLogs, logs)
	assert.NoError(t, err)
	assert.Contains(t, logsBlobName, "test-scope")
	assert.Contains(t, logsBlobName, "logs.json")

	// Test traces
	traces := testdata.GenerateTracesTwoSpansSameResource()
	traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetName("test-span")
	tracesBlobName, err := ae.generateBlobNameWithCompression(pipeline.SignalTraces, traces)
	assert.NoError(t, err)
	assert.Contains(t, tracesBlobName, "test-span")
	assert.Contains(t, tracesBlobName, "traces.json")
}

func getMockAzBlobClient() *mockAzBlobClient {
	mockAzBlobClient := &mockAzBlobClient{
		url: "https://fakeaccount.blob.core.windows.net/",
	}
	mockAzBlobClient.On("UploadStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(azblob.UploadStreamResponse{}, nil)
	return mockAzBlobClient
}

// mockTransportChannel is an autogenerated mock type for the transportChannel type
type mockAzBlobClient struct {
	mock.Mock
	url string
}

func (_m *mockAzBlobClient) URL() string {
	return _m.url
}

func (_m *mockAzBlobClient) UploadStream(ctx context.Context, containerName, blobName string, body io.Reader, o *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error) {
	args := _m.Called(ctx, containerName, blobName, body, o)
	return args.Get(0).(azblob.UploadStreamResponse), args.Error(1)
}

func (_m *mockAzBlobClient) AppendBlock(ctx context.Context, containerName, blobName string, data []byte, o *appendblob.AppendBlockOptions) error {
	args := _m.Called(ctx, containerName, blobName, data, o)
	return args.Error(0)
}

func TestExporterAppendBlob(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
		},
		Container: TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
			LogsFormat:     "2006/01/02/logs_15_04_05.json",
			TracesFormat:   "2006/01/02/traces_15_04_05.json",
			SerialNumRange: 10000,
		},
		FormatType: formatTypeJSON,
		AppendBlob: AppendBlob{
			Enabled:   true,
			Separator: "\n",
		},
		Encodings: Encodings{},
	}

	ae := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ae.client = mockClient

	logs := testdata.GenerateLogsTwoLogRecordsSameResource()
	err := ae.ConsumeLogs(t.Context(), logs)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Test append blob disabled
	c.AppendBlob.Enabled = false
	ae = newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))
	mockClient = &mockAzBlobClient{url: "http://mock"}
	mockClient.On("UploadStream", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).Return(azblob.UploadStreamResponse{}, nil)
	ae.client = mockClient

	err = ae.ConsumeLogs(t.Context(), logs)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestExporterAppendBlobError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
		},
		Container: TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
			LogsFormat:     "2006/01/02/logs_15_04_05.json",
			TracesFormat:   "2006/01/02/traces_15_04_05.json",
			SerialNumRange: 10000,
		},
		FormatType: formatTypeJSON,
		AppendBlob: AppendBlob{
			Enabled:   true,
			Separator: "\n",
		},
		Encodings: Encodings{},
	}

	ae := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("append error"))
	ae.client = mockClient

	logs := testdata.GenerateLogsTwoLogRecordsSameResource()
	err := ae.ConsumeLogs(t.Context(), logs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload data: append error")
	mockClient.AssertExpectations(t)
}

func TestGenerateBlobNameSerialNumberDisabled(t *testing.T) {
	t.Parallel()

	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     "static/metrics.json",
			SerialNumEnabled:  false,
			SerialNumRange:    100,
			TimeParserEnabled: true,
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	assert.Equal(t, "static/metrics.json", blobName)
}

func TestGenerateBlobNameTimeParserDisabled(t *testing.T) {
	t.Parallel()

	layout := "2006/01/02/metrics_15_04_05.json"
	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     layout,
			SerialNumEnabled:  false,
			SerialNumRange:    100,
			TimeParserEnabled: false,
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	assert.Equal(t, layout, blobName)
}

func TestGenerateBlobNameTimeParserDisabledWithSerialNumber(t *testing.T) {
	t.Parallel()

	layout := "2006/01/02/metrics_15_04_05.json"
	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:            layout,
			SerialNumEnabled:         true,
			SerialNumRange:           100,
			SerialNumBeforeExtension: false,
			TimeParserEnabled:        false,
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(blobName, layout+"_"))
}

func TestGenerateBlobNameWithTimeParserRanges(t *testing.T) {
	t.Parallel()

	// Format: "prefix_2006/01/02_suffix.json"
	// Indices: 0-6 = "prefix_", 7-17 = "2006/01/02", 17-29 = "_suffix.json"
	layout := "prefix_2006/01/02_suffix.json"
	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     layout,
			SerialNumEnabled:  false,
			TimeParserEnabled: true,
			TimeParserRanges:  []string{"7-17"},
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	// The prefix and suffix should remain unchanged, only 7-17 (date part) should be parsed
	assert.True(t, strings.HasPrefix(blobName, "prefix_"))
	assert.True(t, strings.HasSuffix(blobName, "_suffix.json"))
	// Should not contain the literal "2006" since it was parsed
	assert.NotContains(t, blobName, "2006")
}

func TestGenerateBlobNameWithMultipleTimeParserRanges(t *testing.T) {
	t.Parallel()

	// Format with two date sections
	// "2006-01-02_static_15:04:05.json"
	// Range 0-10 parses the date, range 18-26 parses the time
	layout := "2006-01-02_static_15:04:05.json"
	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     layout,
			SerialNumEnabled:  false,
			TimeParserEnabled: true,
			TimeParserRanges:  []string{"0-10", "18-26"},
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	// "static" should remain, but date/time parts should be parsed
	assert.Contains(t, blobName, "_static_")
	// Should not contain the literal "2006" or "15:04:05" since they were parsed
	assert.NotContains(t, blobName, "2006")
}

func TestGenerateBlobNameWithInvalidTimeParserRange(t *testing.T) {
	t.Parallel()

	layout := "2006/01/02/metrics.json"
	c := &Config{
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     layout,
			SerialNumEnabled:  false,
			TimeParserEnabled: true,
			TimeParserRanges:  []string{"invalid", "5-3", "100-200"},
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	// Should not error, just skip invalid ranges and log warnings
	blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	// With all ranges invalid or out of bounds, the format should remain unchanged
	assert.Equal(t, layout, blobName)
}

func TestCompression(t *testing.T) {
	t.Parallel()

	testData := []byte(`{"test":"data","value":42}`)

	tests := []struct {
		name            string
		compression     configcompression.Type
		expectExtension string
	}{
		{
			name:            "gzip compression",
			compression:     configcompression.TypeGzip,
			expectExtension: ".gz",
		},
		{
			name:            "zstd compression",
			compression:     configcompression.TypeZstd,
			expectExtension: ".zst",
		},
		{
			name:            "no compression",
			compression:     "",
			expectExtension: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				BlobNameFormat: BlobNameFormat{
					MetricsFormat:    "2006/01/02/metrics.json",
					SerialNumEnabled: false,
				},
				Compression: tt.compression,
				FormatType:  "json",
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fake;AccountKey=ZmFrZQ==;EndpointSuffix=core.windows.net",
				},
			}

			ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
			require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

			compressedData, err := ae.compressContent(testData)
			require.NoError(t, err)

			switch tt.compression {
			case "":
				assert.Equal(t, testData, compressedData, "Uncompressed content should match original")
			case configcompression.TypeGzip, configcompression.TypeZstd:
				decompressed, closer := decompressData(t, compressedData, tt.compression)
				if closer != nil {
					defer closer()
				}
				assert.Equal(t, testData, decompressed, "Decompressed content should match original")
				// Note: for very small payloads like our test data, compression may not reduce size due to header overhead.
				// We still verify correctness via roundtrip.
			}

			// Test filename includes correct extension
			blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
			require.NoError(t, err)
			if tt.expectExtension != "" {
				assert.True(t, strings.HasSuffix(blobName, tt.expectExtension),
					"Generated blob name should end with %q, got %q", tt.expectExtension, blobName)
			} else {
				assert.False(t, strings.HasSuffix(blobName, ".gz") || strings.HasSuffix(blobName, ".zst"))
			}
		})
	}
}

// decompressData decompresses data based on the compression type and returns the decompressed data
// along with a closer function that should be deferred (or nil if no special closing needed)
func decompressData(t *testing.T, compressedData []byte, compression configcompression.Type) ([]byte, func()) {
	t.Helper()
	switch compression {
	case configcompression.TypeGzip:
		reader, err := gzip.NewReader(bytes.NewReader(compressedData))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)
		return decompressed, func() {
			closeErr := reader.Close()
			require.NoError(t, closeErr)
		}
	case configcompression.TypeZstd:
		reader, err := zstd.NewReader(bytes.NewReader(compressedData))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)
		return decompressed, func() {
			reader.Close()
		}
	default:
		t.Fatalf("Unsupported compression type: %s", compression)
		return nil, nil
	}
}

func TestParseRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		maxLen    int
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{
			name:      "valid range",
			input:     "0-10",
			maxLen:    20,
			wantStart: 0,
			wantEnd:   10,
			wantErr:   false,
		},
		{
			name:      "valid range at end",
			input:     "15-25",
			maxLen:    30,
			wantStart: 15,
			wantEnd:   25,
			wantErr:   false,
		},
		{
			name:    "invalid format - no dash",
			input:   "1020",
			maxLen:  20,
			wantErr: true,
		},
		{
			name:    "invalid format - not a number start",
			input:   "abc-10",
			maxLen:  20,
			wantErr: true,
		},
		{
			name:    "invalid format - not a number end",
			input:   "0-xyz",
			maxLen:  20,
			wantErr: true,
		},
		{
			name:    "invalid range - negative start",
			input:   "-5-10",
			maxLen:  20,
			wantErr: true,
		},
		{
			name:    "invalid range - end less than start",
			input:   "10-5",
			maxLen:  20,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseRange(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStart, start)
				assert.Equal(t, tt.wantEnd, end)
			}
		})
	}
}

func newPartitionTestConfig(logsFormat string, templateEnabled bool) *Config {
	return &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
		},
		Container: TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     `{{ getResourceMetricAttr . 0 "service.name" }}.json`,
			LogsFormat:        logsFormat,
			TracesFormat:      `{{ getResourceSpanAttr . 0 "service.name" }}.json`,
			TemplateEnabled:   templateEnabled,
			SerialNumEnabled:  false,
			SerialNumRange:    10000,
			TimeParserEnabled: false,
		},
		FormatType: formatTypeJSON,
		AppendBlob: AppendBlob{
			Enabled:   true,
			Separator: "\n",
		},
		Encodings:            Encodings{},
		MaxConcurrentUploads: 10,
	}
}

func generateLogsWithActivities(activities ...string) plog.Logs {
	logs := plog.NewLogs()
	for _, activity := range activities {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("activity-id", activity)
		lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.Body().SetStr("log for " + activity)
	}
	return logs
}

func TestConsumeLogsPartitionsByRenderedBlobName(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	uploads := make(map[string]plog.Logs)
	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			blobName := args.String(2)
			data := bytes.TrimSuffix(args.Get(3).([]byte), []byte("\n"))
			unmarshaler := plog.JSONUnmarshaler{}
			logs, err := unmarshaler.UnmarshalLogs(data)
			require.NoError(t, err)
			uploads[blobName] = logs
		}).
		Return(nil)
	ae.client = mockClient

	// Resource entries for two activities, interleaved, plus a second entry for
	// the first activity to verify grouping rather than one upload per entry.
	logs := generateLogsWithActivities("activity-a", "activity-b", "activity-a")
	require.NoError(t, ae.ConsumeLogs(t.Context(), logs))

	mockClient.AssertNumberOfCalls(t, "AppendBlock", 2)
	require.Len(t, uploads, 2)

	logsA, ok := uploads["activity-a.json"]
	require.True(t, ok, "expected an upload for activity-a.json, got %v", uploads)
	assert.Equal(t, 2, logsA.ResourceLogs().Len())
	for i := 0; i < logsA.ResourceLogs().Len(); i++ {
		val, attrOK := logsA.ResourceLogs().At(i).Resource().Attributes().Get("activity-id")
		require.True(t, attrOK)
		assert.Equal(t, "activity-a", val.Str())
	}

	logsB, ok := uploads["activity-b.json"]
	require.True(t, ok, "expected an upload for activity-b.json, got %v", uploads)
	assert.Equal(t, 1, logsB.ResourceLogs().Len())
	val, ok := logsB.ResourceLogs().At(0).Resource().Attributes().Get("activity-id")
	require.True(t, ok)
	assert.Equal(t, "activity-b", val.Str())
}

func TestConsumeLogsSingleUploadWhenBlobNamesMatch(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", "activity-a.json", mock.Anything, mock.Anything).Return(nil)
	ae.client = mockClient

	logs := generateLogsWithActivities("activity-a", "activity-a")
	require.NoError(t, ae.ConsumeLogs(t.Context(), logs))

	mockClient.AssertNumberOfCalls(t, "AppendBlock", 1)
}

func TestConsumeLogsSingleUploadWhenTemplateDisabled(t *testing.T) {
	c := newPartitionTestConfig("static/logs.json", false)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", "static/logs.json", mock.Anything, mock.Anything).Return(nil)
	ae.client = mockClient

	logs := generateLogsWithActivities("activity-a", "activity-b")
	require.NoError(t, ae.ConsumeLogs(t.Context(), logs))

	mockClient.AssertNumberOfCalls(t, "AppendBlock", 1)
}

func TestConsumeMetricsPartitionsByRenderedBlobName(t *testing.T) {
	c := newPartitionTestConfig("logs.json", true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	var blobNames []string
	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "metrics", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			blobNames = append(blobNames, args.String(2))
		}).
		Return(nil)
	ae.client = mockClient

	metrics := pmetric.NewMetrics()
	for _, svc := range []string{"svc-a", "svc-b"} {
		rm := metrics.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", svc)
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("metric for " + svc)
	}
	require.NoError(t, ae.ConsumeMetrics(t.Context(), metrics))

	mockClient.AssertNumberOfCalls(t, "AppendBlock", 2)
	assert.ElementsMatch(t, []string{"svc-a.json", "svc-b.json"}, blobNames)
}

func TestConsumeTracesPartitionsByRenderedBlobName(t *testing.T) {
	c := newPartitionTestConfig("logs.json", true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalTraces)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	var blobNames []string
	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "traces", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			blobNames = append(blobNames, args.String(2))
		}).
		Return(nil)
	ae.client = mockClient

	traces := ptrace.NewTraces()
	for _, svc := range []string{"svc-a", "svc-b"} {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", svc)
		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("span for " + svc)
	}
	require.NoError(t, ae.ConsumeTraces(t.Context(), traces))

	mockClient.AssertNumberOfCalls(t, "AppendBlock", 2)
	assert.ElementsMatch(t, []string{"svc-a.json", "svc-b.json"}, blobNames)
}

func TestConsumeLogsPartitionFallsBackOnTemplateError(t *testing.T) {
	// A template that fails at execution time (invalid field access on the root
	// object) must fall back to a single upload with the default name format.
	c := newPartitionTestConfig("{{ .NoSuchField }}", true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", "{{ .NoSuchField }}", mock.Anything, mock.Anything).Return(nil)
	ae.client = mockClient

	logs := generateLogsWithActivities("activity-a", "activity-b")
	require.NoError(t, ae.ConsumeLogs(t.Context(), logs))

	mockClient.AssertNumberOfCalls(t, "AppendBlock", 1)
}

// recordingAzBlobClient is a stateful azblobClient that records every uploaded
// payload per blob name and can be programmed to fail a number of times per
// blob. It is safe for concurrent use, matching the concurrent group uploads.
type recordingAzBlobClient struct {
	mu                sync.Mutex
	remainingFailures map[string]int
	uploads           map[string][][]byte
	inflight          int
	maxInflight       int
}

func newRecordingAzBlobClient(failures map[string]int) *recordingAzBlobClient {
	return &recordingAzBlobClient{
		remainingFailures: failures,
		uploads:           make(map[string][][]byte),
	}
}

func (*recordingAzBlobClient) URL() string { return "http://mock" }

func (c *recordingAzBlobClient) record(blobName string, data []byte) error {
	c.mu.Lock()
	c.inflight++
	if c.inflight > c.maxInflight {
		c.maxInflight = c.inflight
	}
	c.mu.Unlock()
	// Hold the upload open briefly so concurrent uploads overlap observably.
	time.Sleep(time.Millisecond)
	c.mu.Lock()
	defer func() {
		c.inflight--
		c.mu.Unlock()
	}()
	if c.remainingFailures[blobName] > 0 {
		c.remainingFailures[blobName]--
		return errors.New("injected failure for " + blobName)
	}
	c.uploads[blobName] = append(c.uploads[blobName], bytes.Clone(data))
	return nil
}

func (c *recordingAzBlobClient) UploadStream(_ context.Context, _, blobName string, body io.Reader, _ *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return azblob.UploadStreamResponse{}, err
	}
	return azblob.UploadStreamResponse{}, c.record(blobName, data)
}

func (c *recordingAzBlobClient) AppendBlock(_ context.Context, _, blobName string, data []byte, _ *appendblob.AppendBlockOptions) error {
	return c.record(blobName, data)
}

// uploadedLogBodies decodes every recorded payload for a blob and returns the
// log record bodies it contains, in upload order.
func uploadedLogBodies(t *testing.T, payloads [][]byte) []string {
	t.Helper()
	var bodies []string
	unmarshaler := plog.JSONUnmarshaler{}
	for _, payload := range payloads {
		logs, err := unmarshaler.UnmarshalLogs(bytes.TrimSuffix(payload, []byte("\n")))
		require.NoError(t, err)
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			sls := logs.ResourceLogs().At(i).ScopeLogs()
			for j := 0; j < sls.Len(); j++ {
				lrs := sls.At(j).LogRecords()
				for k := 0; k < lrs.Len(); k++ {
					bodies = append(bodies, lrs.At(k).Body().Str())
				}
			}
		}
	}
	return bodies
}

func TestConsumeLogsPartialFailureRetriesExactlyOnce(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	// activity-b fails once, then succeeds; activity-a always succeeds.
	client := newRecordingAzBlobClient(map[string]int{"activity-b.json": 1})
	ae.client = client

	logs := generateLogsWithActivities("activity-a", "activity-b")

	// First attempt: partial failure.
	err := ae.ConsumeLogs(t.Context(), logs)
	require.Error(t, err)

	// The error must carry only the failed group's data, exactly as the
	// exporterhelper retry sender extracts it via OnError.
	var logsErr consumererror.Logs
	require.ErrorAs(t, err, &logsErr)
	retryData := logsErr.Data()
	require.Equal(t, 1, retryData.ResourceLogs().Len())
	val, ok := retryData.ResourceLogs().At(0).Resource().Attributes().Get("activity-id")
	require.True(t, ok)
	assert.Equal(t, "activity-b", val.Str())

	// Second attempt with only the retry data, as the retry sender would send.
	require.NoError(t, ae.ConsumeLogs(t.Context(), retryData))

	// Exactly-once: each log message appears exactly once, in its own blob.
	assert.Equal(t, []string{"log for activity-a"}, uploadedLogBodies(t, client.uploads["activity-a.json"]))
	assert.Equal(t, []string{"log for activity-b"}, uploadedLogBodies(t, client.uploads["activity-b.json"]))
	require.Len(t, client.uploads, 2)
}

func TestConsumeLogsAllGroupsUploadedDespiteFailure(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	// Middle group fails: the other groups must still be uploaded.
	client := newRecordingAzBlobClient(map[string]int{"activity-b.json": 1})
	ae.client = client

	logs := generateLogsWithActivities("activity-a", "activity-b", "activity-c")
	err := ae.ConsumeLogs(t.Context(), logs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected failure for activity-b.json")

	assert.Equal(t, []string{"log for activity-a"}, uploadedLogBodies(t, client.uploads["activity-a.json"]))
	assert.Equal(t, []string{"log for activity-c"}, uploadedLogBodies(t, client.uploads["activity-c.json"]))
	assert.Empty(t, client.uploads["activity-b.json"])
}

func TestConsumeMetricsPartialFailureCarriesOnlyFailedData(t *testing.T) {
	c := newPartitionTestConfig("logs.json", true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	client := newRecordingAzBlobClient(map[string]int{"svc-b.json": 1})
	ae.client = client

	metrics := pmetric.NewMetrics()
	for _, svc := range []string{"svc-a", "svc-b"} {
		rm := metrics.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", svc)
		rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric for " + svc)
	}

	err := ae.ConsumeMetrics(t.Context(), metrics)
	require.Error(t, err)

	var metricsErr consumererror.Metrics
	require.ErrorAs(t, err, &metricsErr)
	retryData := metricsErr.Data()
	require.Equal(t, 1, retryData.ResourceMetrics().Len())
	val, ok := retryData.ResourceMetrics().At(0).Resource().Attributes().Get("service.name")
	require.True(t, ok)
	assert.Equal(t, "svc-b", val.Str())
}

func TestConsumeTracesPartialFailureCarriesOnlyFailedData(t *testing.T) {
	c := newPartitionTestConfig("logs.json", true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalTraces)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	client := newRecordingAzBlobClient(map[string]int{"svc-b.json": 1})
	ae.client = client

	traces := ptrace.NewTraces()
	for _, svc := range []string{"svc-a", "svc-b"} {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", svc)
		rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("span for " + svc)
	}

	err := ae.ConsumeTraces(t.Context(), traces)
	require.Error(t, err)

	var tracesErr consumererror.Traces
	require.ErrorAs(t, err, &tracesErr)
	retryData := tracesErr.Data()
	require.Equal(t, 1, retryData.ResourceSpans().Len())
	val, ok := retryData.ResourceSpans().At(0).Resource().Attributes().Get("service.name")
	require.True(t, ok)
	assert.Equal(t, "svc-b", val.Str())
}

func TestConsumeLogsManyGroupsConcurrentUploads(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	client := newRecordingAzBlobClient(nil)
	ae.client = client

	// More groups than maxConcurrentUploads to exercise the semaphore.
	activities := make([]string, 0, 25)
	for i := range 25 {
		activities = append(activities, fmt.Sprintf("activity-%02d", i))
	}
	logs := generateLogsWithActivities(activities...)
	require.NoError(t, ae.ConsumeLogs(t.Context(), logs))

	require.Len(t, client.uploads, 25)
	for _, activity := range activities {
		assert.Equal(t, []string{"log for " + activity}, uploadedLogBodies(t, client.uploads[activity+".json"]))
	}
}

// TestPartitionWithQueueBatching wires the exporter behind the real
// exporterhelper queue batcher, exactly as createLogsExporter does, and
// verifies that requests merged by the batcher are partitioned back out to
// their own blobs with no mixed or duplicated log records.
func TestPartitionWithQueueBatching(t *testing.T) {
	cfg := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)
	qCfg := exporterhelper.NewDefaultQueueConfig()
	// Merge exactly the six records sent below into one request. The flush
	// timeout is long so the batch is released by min_size, deterministically.
	qCfg.Batch = configoptional.Some(exporterhelper.BatchConfig{
		Sizer:        exporterhelper.RequestSizerTypeItems,
		MinSize:      6,
		FlushTimeout: 10 * time.Second,
	})
	cfg.QueueSettings = configoptional.Some(qCfg)

	ae := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
	le, err := exporterhelper.NewLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg,
		ae.ConsumeLogs,
		exporterhelper.WithStart(ae.start),
		exporterhelper.WithQueue(cfg.QueueSettings))
	require.NoError(t, err)

	require.NoError(t, le.Start(t.Context(), componenttest.NewNopHost()))
	client := newRecordingAzBlobClient(nil)
	ae.client = client

	// Six single-record payloads, two per activity, interleaved. Each becomes
	// its own queue request; the batcher merges them before ConsumeLogs runs.
	for i := range 2 {
		for _, activity := range []string{"activity-a", "activity-b", "activity-c"} {
			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("activity-id", activity)
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(fmt.Sprintf("log %d for %s", i, activity))
			require.NoError(t, le.ConsumeLogs(t.Context(), logs))
		}
	}
	require.NoError(t, le.Shutdown(t.Context()))

	client.mu.Lock()
	defer client.mu.Unlock()
	require.Len(t, client.uploads, 3)
	for _, activity := range []string{"activity-a", "activity-b", "activity-c"} {
		payloads := client.uploads[activity+".json"]
		// One upload per blob proves the batcher merged the six requests before
		// partitioning; without merging there would be two appends per blob.
		require.Len(t, payloads, 1, "expected one merged upload for %s", activity)
		bodies := uploadedLogBodies(t, payloads)
		require.ElementsMatch(t,
			[]string{"log 0 for " + activity, "log 1 for " + activity},
			bodies, "blob for %s must hold exactly its two records", activity)
	}
}

func TestConsumeLogsHonorsMaxConcurrentUploads(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)
	c.MaxConcurrentUploads = 1

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	client := newRecordingAzBlobClient(nil)
	ae.client = client

	activities := make([]string, 0, 8)
	for i := range 8 {
		activities = append(activities, fmt.Sprintf("activity-%d", i))
	}
	require.NoError(t, ae.ConsumeLogs(t.Context(), generateLogsWithActivities(activities...)))

	client.mu.Lock()
	defer client.mu.Unlock()
	require.Len(t, client.uploads, 8)
	assert.Equal(t, 1, client.maxInflight, "uploads must be serialized when max_concurrent_uploads is 1")
}
