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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"
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

	me, createErr := newAzureBlobExporter(c, logger, pipeline.SignalMetrics)
	require.NoError(t, createErr)
	assert.NotNil(t, me)
	assert.NoError(t, me.start(t.Context(), componenttest.NewNopHost()))

	le, createErr := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	require.NoError(t, createErr)
	assert.NotNil(t, le)
	assert.NoError(t, le.start(t.Context(), componenttest.NewNopHost()))

	te, createErr := newAzureBlobExporter(c, logger, pipeline.SignalTraces)
	require.NoError(t, createErr)
	assert.NotNil(t, te)
	assert.NoError(t, te.start(t.Context(), componenttest.NewNopHost()))
}

func TestNewExporterInvalidCompression(t *testing.T) {
	t.Parallel()

	c := &Config{
		Compression:         configcompression.TypeGzip,
		CompressionSettings: configcompression.CompressionParams{Level: 10},
	}
	ae, err := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.ErrorContains(t, err, "failed to create compression writer")
	require.Nil(t, ae)
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
			azureBlobExporter, createErr := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			require.NoError(t, createErr)
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
			azureBlobExporter, createErr := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			require.NoError(t, createErr)
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
			azureBlobExporter, createErr := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			require.NoError(t, createErr)
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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)
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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)
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

	ae, createErr := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	require.NoError(t, createErr)
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
	ae, createErr = newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	require.NoError(t, createErr)
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

	ae, createErr := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	require.NoError(t, createErr)
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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
	require.NoError(t, createErr)

	// Should not error, just skip invalid ranges and log warnings
	blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	// With all ranges invalid or out of bounds, the format should remain unchanged
	assert.Equal(t, layout, blobName)
}

func TestCompression(t *testing.T) {
	t.Parallel()

	testData := bytes.Repeat([]byte(`{"test":"data","value":42}`+"\n"), 64)

	tests := []struct {
		name            string
		compression     configcompression.Type
		level           configcompression.Level
		expectExtension string
		storeOnly       bool
	}{
		{name: "gzip", compression: configcompression.TypeGzip, level: 1, expectExtension: ".gz"},
		{name: "gzip level zero", compression: configcompression.TypeGzip, expectExtension: ".gz", storeOnly: true},
		{name: "zstd", compression: configcompression.TypeZstd, expectExtension: ".zst"},
		{name: "zlib", compression: configcompression.TypeZlib, level: configcompression.DefaultCompressionLevel, expectExtension: ".zlib"},
		{name: "deflate", compression: configcompression.TypeDeflate, level: configcompression.DefaultCompressionLevel, expectExtension: ".deflate"},
		{name: "snappy", compression: configcompression.TypeSnappy, expectExtension: ".snappy"},
		{name: "framed snappy", compression: configcompression.TypeSnappyFramed, expectExtension: ".snappy"},
		{name: "lz4", compression: configcompression.TypeLz4, expectExtension: ".lz4"},
		{name: "unset compression"},
		{name: "explicit none", compression: "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Config{
				BlobNameFormat: BlobNameFormat{
					MetricsFormat: "metrics.json",
					TemplateScope: TemplateScopeRequest,
				},
				Container:           TelemetryConfig{Metrics: "metrics"},
				Compression:         tt.compression,
				CompressionSettings: configcompression.CompressionParams{Level: tt.level},
				FormatType:          formatTypeJSON,
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fake;AccountKey=ZmFrZQ==;EndpointSuffix=core.windows.net",
				},
			}
			require.NoError(t, c.Validate())

			ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)
			require.NoError(t, createErr)
			require.Equal(t, tt.compression.IsCompressed(), ae.writer != nil)
			require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

			client := &mockAzBlobClient{}
			client.On("UploadStream", mock.Anything, "metrics", "metrics.json"+tt.expectExtension, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					body, err := io.ReadAll(args.Get(3).(io.Reader))
					require.NoError(t, err)
					if tt.storeOnly {
						assert.Greater(t, len(body), len(testData), "level zero should store the payload with format overhead")
					} else if tt.compression.IsCompressed() {
						assert.Less(t, len(body), len(testData), "the configured level should compress the repetitive payload")
					}
					assert.Equal(t, testData, decompressData(t, body, tt.compression))
				}).Return(azblob.UploadStreamResponse{}, nil).Once()
			ae.client = client
			blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalMetrics, nil)
			require.NoError(t, err)
			require.NoError(t, ae.consumeData(t.Context(), blobName, testData, pipeline.SignalMetrics))
			client.AssertExpectations(t)
		})
	}
}

func decompressData(t *testing.T, compressedData []byte, compression configcompression.Type) []byte {
	t.Helper()
	switch compression {
	case configcompression.TypeGzip:
		reader, err := gzip.NewReader(bytes.NewReader(compressedData))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		return decompressed
	case configcompression.TypeZstd:
		reader, err := zstd.NewReader(bytes.NewReader(compressedData))
		require.NoError(t, err)
		defer reader.Close()
		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)
		return decompressed
	default:
		if compression == configcompression.TypeSnappyFramed {
			compression = configcompression.TypeSnappy
		}
		reader, err := xcompression.NewReaderFactory().NewReader(compression, configcompression.CompressionParams{})
		require.NoError(t, err)
		var decoded bytes.Buffer
		_, err = reader.DecompressBytes(&decoded, compressedData)
		require.NoError(t, err)
		return decoded.Bytes()
	}
}

func TestAppendBlobCompression(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		compression configcompression.Type
		suffix      string
	}{
		{compression: configcompression.TypeGzip, suffix: ".gz"},
		{compression: configcompression.TypeZstd, suffix: ".zst"},
	} {
		t.Run(string(tc.compression), func(t *testing.T) {
			t.Parallel()

			c := createDefaultConfig().(*Config)
			c.Auth.ConnectionString = "DefaultEndpointsProtocol=https;AccountName=fake;AccountKey=ZmFrZQ==;EndpointSuffix=core.windows.net"
			c.AppendBlob = AppendBlob{Enabled: true, Separator: "\n"}
			c.BlobNameFormat = BlobNameFormat{LogsFormat: "logs.json", TemplateScope: TemplateScopeRequest}
			c.Compression = tc.compression
			require.NoError(t, c.Validate())

			ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
			require.NoError(t, createErr)
			require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))
			var blob bytes.Buffer
			client := &mockAzBlobClient{}
			client.On("AppendBlock", mock.Anything, "logs", "logs.json"+tc.suffix, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) { _, _ = blob.Write(args.Get(3).([]byte)) }).Return(nil).Twice()
			ae.client = client
			blobName, err := ae.generateBlobNameWithCompression(pipeline.SignalLogs, nil)
			require.NoError(t, err)
			for _, payload := range []string{"first", "second"} {
				require.NoError(t, ae.consumeData(t.Context(), blobName, []byte(payload), pipeline.SignalLogs))
			}
			client.AssertExpectations(t)
			assert.Equal(t, "first\nsecond\n", string(decompressData(t, blob.Bytes(), tc.compression)))
		})
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
			TemplateScope:     TemplateScopeResource,
			SerialNumEnabled:  false,
			SerialNumRange:    10000,
			TimeParserEnabled: false,
		},
		FormatType: formatTypeJSON,
		AppendBlob: AppendBlob{
			Enabled:   true,
			Separator: "\n",
		},
		Encodings: Encodings{},
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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	client := newRecordingAzBlobClient(nil)
	ae.client = client

	// Resource entries for two activities, interleaved, plus a second entry for
	// the first activity to verify grouping rather than one upload per entry.
	logs := generateLogsWithActivities("activity-a", "activity-b", "activity-a")
	require.NoError(t, ae.ConsumeLogs(t.Context(), logs))

	require.Len(t, client.uploads, 2)
	require.Len(t, client.uploads["activity-a.json"], 1)
	require.Len(t, client.uploads["activity-b.json"], 1)
	unmarshaler := plog.JSONUnmarshaler{}
	logsA, err := unmarshaler.UnmarshalLogs(bytes.TrimSuffix(client.uploads["activity-a.json"][0], []byte("\n")))
	require.NoError(t, err)
	assert.Equal(t, 2, logsA.ResourceLogs().Len())
	for i := 0; i < logsA.ResourceLogs().Len(); i++ {
		val, attrOK := logsA.ResourceLogs().At(i).Resource().Attributes().Get("activity-id")
		require.True(t, attrOK)
		assert.Equal(t, "activity-a", val.Str())
	}

	logsB, err := unmarshaler.UnmarshalLogs(bytes.TrimSuffix(client.uploads["activity-b.json"][0], []byte("\n")))
	require.NoError(t, err)
	assert.Equal(t, 1, logsB.ResourceLogs().Len())
	val, ok := logsB.ResourceLogs().At(0).Resource().Attributes().Get("activity-id")
	require.True(t, ok)
	assert.Equal(t, "activity-b", val.Str())
}

func TestConsumeLogsSingleUploadWhenBlobNamesMatch(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
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

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", "static/logs.json", mock.Anything, mock.Anything).Return(nil)
	ae.client = mockClient

	logs := generateLogsWithActivities("activity-a", "activity-b")
	require.NoError(t, ae.ConsumeLogs(t.Context(), logs))

	mockClient.AssertNumberOfCalls(t, "AppendBlock", 1)
}

func TestConsumeLogsPartitionFallsBackOnTemplateError(t *testing.T) {
	// A template that fails at execution time (invalid field access on the root
	// object) must fall back to a single upload with the default name format.
	c := newPartitionTestConfig("{{ .NoSuchField }}", true)

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
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
// blob. It is safe for concurrent use by multiple export requests.
type recordingAzBlobClient struct {
	mu                sync.Mutex
	remainingFailures map[string]int
	uploads           map[string][][]byte
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
	defer c.mu.Unlock()
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

func TestConsumeLogsPartialFailureRetriesOnlyFailedGroups(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
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

	// Retrying a known failed group must not replay successful groups.
	assert.Equal(t, []string{"log for activity-a"}, uploadedLogBodies(t, client.uploads["activity-a.json"]))
	assert.Equal(t, []string{"log for activity-b"}, uploadedLogBodies(t, client.uploads["activity-b.json"]))
	require.Len(t, client.uploads, 2)
}

// TestPartitionWithExporterHelperRetry wires the exporter behind the real
// exporterhelper queue and retry senders and verifies that, after a partial
// failure, only the failed group is retried and no successful group is
// uploaded twice.
func TestPartitionWithExporterHelperRetry(t *testing.T) {
	cfg := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)
	defaults := createDefaultConfig().(*Config)
	cfg.TimeoutSettings = defaults.TimeoutSettings
	cfg.BackOffConfig = defaults.BackOffConfig
	cfg.BackOffConfig.InitialInterval = time.Millisecond
	cfg.BackOffConfig.MaxInterval = 10 * time.Millisecond
	qCfg := exporterhelper.NewDefaultQueueConfig()
	qCfg.Batch = configoptional.None[exporterhelper.BatchConfig]()
	// Block ConsumeLogs until the queued request, including retries, completes.
	qCfg.WaitForResult = true
	cfg.QueueSettings = configoptional.Some(qCfg)

	ae, createErr := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
	le, err := exporterhelper.NewLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg,
		ae.ConsumeLogs,
		exporterhelper.WithStart(ae.start),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings))
	require.NoError(t, err)
	require.NoError(t, le.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, le.Shutdown(t.Context())) }()
	client := newRecordingAzBlobClient(map[string]int{"activity-b.json": 1})
	ae.client = client

	logs := generateLogsWithActivities("activity-a", "activity-b", "activity-c")
	require.NoError(t, le.ConsumeLogs(t.Context(), logs))

	client.mu.Lock()
	defer client.mu.Unlock()
	require.Zero(t, client.remainingFailures["activity-b.json"])
	require.Len(t, client.uploads, 3)
	for _, activity := range []string{"activity-a", "activity-b", "activity-c"} {
		payloads := client.uploads[activity+".json"]
		require.Len(t, payloads, 1, "the retry chain must not replay a successful group")
		assert.Equal(t, []string{"log for " + activity}, uploadedLogBodies(t, payloads))
	}
}

func TestConsumeLogsAllGroupsUploadedDespiteFailure(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
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

func TestConsumeLogsManyGroups(t *testing.T) {
	c := newPartitionTestConfig(`{{ getResourceLogAttr . 0 "activity-id" }}.json`, true)

	ae, createErr := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
	require.NoError(t, ae.start(t.Context(), componenttest.NewNopHost()))

	client := newRecordingAzBlobClient(nil)
	ae.client = client

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

	ae, createErr := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, createErr)
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
