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
		Auth: &Authentication{
			Type:             ConnectionString,
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
		},
		Container: &TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: &BlobNameFormat{
			MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
			LogsFormat:     "2006/01/02/logs_15_04_05.json",
			TracesFormat:   "2006/01/02/traces_15_04_05.json",
			SerialNumRange: 10000,
			Params:         map[string]string{},
		},
		FormatType: "json",
		Encodings:  &Encodings{},
	}

	me := newAzureBlobExporter(c, logger, pipeline.SignalMetrics)
	assert.NotNil(t, me)
	assert.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))

	le := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NotNil(t, le)
	assert.NoError(t, le.start(context.Background(), componenttest.NewNopHost()))

	te := newAzureBlobExporter(c, logger, pipeline.SignalTraces)
	assert.NotNil(t, te)
	assert.NoError(t, te.start(context.Background(), componenttest.NewNopHost()))
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
			assert.NoError(t, azureBlobExporter.start(context.Background(), componenttest.NewNopHost()))
			azureBlobExporter.client = getMockAzBlobClient()

			metrics := testdata.GenerateMetricsTwoMetrics()
			assert.NoError(t, azureBlobExporter.ConsumeMetrics(context.Background(), metrics))
		})
		t.Run(tt.id.String()+"-logs", func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			azureBlobExporter := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			assert.NoError(t, azureBlobExporter.start(context.Background(), componenttest.NewNopHost()))
			azureBlobExporter.client = getMockAzBlobClient()

			logs := testdata.GenerateLogsTwoLogRecordsSameResource()
			assert.NoError(t, azureBlobExporter.ConsumeLogs(context.Background(), logs))
		})
		t.Run(tt.id.String()+"-traces", func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			azureBlobExporter := newAzureBlobExporter(cfg.(*Config), zaptest.NewLogger(t), pipeline.SignalMetrics)
			assert.NoError(t, azureBlobExporter.start(context.Background(), componenttest.NewNopHost()))
			azureBlobExporter.client = getMockAzBlobClient()

			traces := testdata.GenerateTracesTwoSpansSameResource()
			assert.NoError(t, azureBlobExporter.ConsumeTraces(context.Background(), traces))
		})
	}
}

func TestGenerateBlobName(t *testing.T) {
	t.Parallel()

	c := &Config{
		BlobNameFormat: &BlobNameFormat{
			MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
			LogsFormat:     "2006/01/02/logs_15_04_05.json",
			TracesFormat:   "2006/01/02/traces_15_04_05.json",
			SerialNumRange: 10000,
			Params:         map[string]string{},
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	now := time.Now()
	metricsBlobName, err := ae.generateBlobName(pipeline.SignalMetrics)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(metricsBlobName, now.Format(c.BlobNameFormat.MetricsFormat)))

	logsBlobName, err := ae.generateBlobName(pipeline.SignalLogs)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(logsBlobName, now.Format(c.BlobNameFormat.LogsFormat)))

	tracesBlobName, err := ae.generateBlobName(pipeline.SignalTraces)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(tracesBlobName, now.Format(c.BlobNameFormat.TracesFormat)))
}

func TestGenerateBlobNameSerialNumBefore(t *testing.T) {
	t.Parallel()

	c := &Config{
		BlobNameFormat: &BlobNameFormat{
			MetricsFormat:            "2006/01/02/metrics_15_04_05.json",
			LogsFormat:               "2006/01/02/logs_15_04_05.json",
			TracesFormat:             "2006/01/02/traces_15_04_05", // no extension
			SerialNumRange:           10000,
			SerialNumBeforeExtension: true,
			Params:                   map[string]string{},
		},
	}

	ae := newAzureBlobExporter(c, zaptest.NewLogger(t), pipeline.SignalMetrics)

	assertFormat := func(blobName string, format string) {
		ext := filepath.Ext(format)
		formatWithoutExt := strings.TrimSuffix(format, ext)
		assert.True(t, strings.HasPrefix(blobName, formatWithoutExt))
		assert.True(t, strings.HasSuffix(blobName, ext))
	}

	now := time.Now()
	metricsBlobName, err := ae.generateBlobName(pipeline.SignalMetrics)
	assert.NoError(t, err)
	assertFormat(metricsBlobName, now.Format(c.BlobNameFormat.MetricsFormat))

	logsBlobName, err := ae.generateBlobName(pipeline.SignalLogs)
	assert.NoError(t, err)
	assertFormat(logsBlobName, now.Format(c.BlobNameFormat.LogsFormat))

	tracesBlobName, err := ae.generateBlobName(pipeline.SignalTraces)
	assert.NoError(t, err)
	assertFormat(tracesBlobName, now.Format(c.BlobNameFormat.TracesFormat))
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

func (_m *mockAzBlobClient) UploadStream(ctx context.Context, containerName string, blobName string, body io.Reader, o *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error) {
	args := _m.Called(ctx, containerName, blobName, body, o)
	return args.Get(0).(azblob.UploadStreamResponse), args.Error(1)
}

func (_m *mockAzBlobClient) AppendBlock(ctx context.Context, containerName string, blobName string, data []byte, o *appendblob.AppendBlockOptions) error {
	args := _m.Called(ctx, containerName, blobName, data, o)
	return args.Error(0)
}

func TestExporterAppendBlob(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := &Config{
		Auth: &Authentication{
			Type:             ConnectionString,
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
		},
		Container: &TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: &BlobNameFormat{
			MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
			LogsFormat:     "2006/01/02/logs_15_04_05.json",
			TracesFormat:   "2006/01/02/traces_15_04_05.json",
			SerialNumRange: 10000,
		},
		FormatType: formatTypeJSON,
		AppendBlob: &AppendBlob{
			Enabled:   true,
			Separator: "\n",
		},
		Encodings: &Encodings{},
	}

	ae := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NoError(t, ae.start(context.Background(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ae.client = mockClient

	logs := testdata.GenerateLogsTwoLogRecordsSameResource()
	err := ae.ConsumeLogs(context.Background(), logs)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Test append blob disabled
	c.AppendBlob.Enabled = false
	ae = newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NoError(t, ae.start(context.Background(), componenttest.NewNopHost()))
	mockClient = &mockAzBlobClient{url: "http://mock"}
	mockClient.On("UploadStream", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).Return(azblob.UploadStreamResponse{}, nil)
	ae.client = mockClient

	err = ae.ConsumeLogs(context.Background(), logs)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestExporterAppendBlobError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := &Config{
		Auth: &Authentication{
			Type:             ConnectionString,
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=fakeaccount;AccountKey=ZmFrZWtleQ==;EndpointSuffix=core.windows.net",
		},
		Container: &TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: &BlobNameFormat{
			MetricsFormat:  "2006/01/02/metrics_15_04_05.json",
			LogsFormat:     "2006/01/02/logs_15_04_05.json",
			TracesFormat:   "2006/01/02/traces_15_04_05.json",
			SerialNumRange: 10000,
		},
		FormatType: formatTypeJSON,
		AppendBlob: &AppendBlob{
			Enabled:   true,
			Separator: "\n",
		},
		Encodings: &Encodings{},
	}

	ae := newAzureBlobExporter(c, logger, pipeline.SignalLogs)
	assert.NoError(t, ae.start(context.Background(), componenttest.NewNopHost()))

	mockClient := &mockAzBlobClient{url: "http://mock"}
	mockClient.On("AppendBlock", mock.Anything, "logs", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("append error"))
	ae.client = mockClient

	logs := testdata.GenerateLogsTwoLogRecordsSameResource()
	err := ae.ConsumeLogs(context.Background(), logs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload data: append error")
	mockClient.AssertExpectations(t)
}
