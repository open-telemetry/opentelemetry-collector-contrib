// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/appendblob"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
	metricsBlobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(metricsBlobName, now.Format(c.BlobNameFormat.MetricsFormat)))

	logsBlobName, err := ae.generateBlobName(pipeline.SignalLogs, nil)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(logsBlobName, now.Format(c.BlobNameFormat.LogsFormat)))

	tracesBlobName, err := ae.generateBlobName(pipeline.SignalTraces, nil)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(tracesBlobName, now.Format(c.BlobNameFormat.TracesFormat)))
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
	metricsBlobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
	assert.NoError(t, err)
	assertFormat(metricsBlobName, now.Format(c.BlobNameFormat.MetricsFormat))

	logsBlobName, err := ae.generateBlobName(pipeline.SignalLogs, nil)
	assert.NoError(t, err)
	assertFormat(logsBlobName, now.Format(c.BlobNameFormat.LogsFormat))

	tracesBlobName, err := ae.generateBlobName(pipeline.SignalTraces, nil)
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
	metricsBlobName, err := ae.generateBlobName(pipeline.SignalMetrics, metrics)
	assert.NoError(t, err)
	assert.Contains(t, metricsBlobName, "test-metrics-service")
	assert.Contains(t, metricsBlobName, "metrics.json")

	// Test logs
	logs := testdata.GenerateLogsTwoLogRecordsSameResource()
	logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("scope.name", "test-scope")
	logsBlobName, err := ae.generateBlobName(pipeline.SignalLogs, logs)
	assert.NoError(t, err)
	assert.Contains(t, logsBlobName, "test-scope")
	assert.Contains(t, logsBlobName, "logs.json")

	// Test traces
	traces := testdata.GenerateTracesTwoSpansSameResource()
	traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetName("test-span")
	tracesBlobName, err := ae.generateBlobName(pipeline.SignalTraces, traces)
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

	blobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
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

	blobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
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

	blobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
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

	blobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
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

	blobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
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
	blobName, err := ae.generateBlobName(pipeline.SignalMetrics, nil)
	require.NoError(t, err)
	// With all ranges invalid or out of bounds, the format should remain unchanged
	assert.Equal(t, layout, blobName)
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
