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
	"math/rand/v2"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/appendblob"
	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type azureBlobExporter struct {
	config           *Config
	logger           *zap.Logger
	client           azblobClient
	signal           pipeline.Signal
	marshaller       *marshaller
	blobNameTemplate *blobNameTemplate
	timeLocation     *time.Location
	gzipWriterPool   *sync.Pool
	zstdWriterPool   *sync.Pool
}

type blobNameTemplate struct {
	metrics *template.Template
	logs    *template.Template
	traces  *template.Template
}

func getAttrStandalone(attrs pcommon.Map, key string) any {
	if val, ok := attrs.Get(key); ok {
		return val.AsRaw()
	}
	return nil
}

var tempFuncs = template.FuncMap{
	"getResourceSpanAttr": func(traces ptrace.Traces, rmIndex int, key string) any {
		if traces.ResourceSpans().Len() > 0 {
			rs := traces.ResourceSpans().At(rmIndex)
			return getAttrStandalone(rs.Resource().Attributes(), key)
		}
		return nil
	},
	"getResourceMetricAttr": func(metrics pmetric.Metrics, rmIndex int, key string) any {
		if metrics.ResourceMetrics().Len() > 0 {
			rm := metrics.ResourceMetrics().At(rmIndex)
			return getAttrStandalone(rm.Resource().Attributes(), key)
		}
		return nil
	},
	"getResourceLogAttr": func(logs plog.Logs, rlIndex int, key string) any {
		if logs.ResourceLogs().Len() > 0 {
			rl := logs.ResourceLogs().At(rlIndex)
			return getAttrStandalone(rl.Resource().Attributes(), key)
		}
		return nil
	},
	"getScopeSpanAttr": func(traces ptrace.Traces, rmIndex, ilsIndex int, key string) any {
		if traces.ResourceSpans().Len() > 0 {
			rs := traces.ResourceSpans().At(rmIndex)
			if rs.ScopeSpans().Len() > 0 {
				ils := rs.ScopeSpans().At(ilsIndex)
				return getAttrStandalone(ils.Scope().Attributes(), key)
			}
		}
		return nil
	},
	"getScopeMetricAttr": func(metrics pmetric.Metrics, rmIndex, ilsIndex int, key string) any {
		if metrics.ResourceMetrics().Len() > 0 {
			rm := metrics.ResourceMetrics().At(rmIndex)
			if rm.ScopeMetrics().Len() > 0 {
				ils := rm.ScopeMetrics().At(ilsIndex)
				return getAttrStandalone(ils.Scope().Attributes(), key)
			}
		}
		return nil
	},
	"getScopeLogAttr": func(logs plog.Logs, rlIndex, ilsIndex int, key string) any {
		if logs.ResourceLogs().Len() > 0 {
			rl := logs.ResourceLogs().At(rlIndex)
			if rl.ScopeLogs().Len() > 0 {
				ils := rl.ScopeLogs().At(ilsIndex)
				return getAttrStandalone(ils.Scope().Attributes(), key)
			}
		}
		return nil
	},
	"getSpan": func(traces ptrace.Traces, rmIndex, ilsIndex, spanIndex int) any {
		if traces.ResourceSpans().Len() > 0 {
			rs := traces.ResourceSpans().At(rmIndex)
			if rs.ScopeSpans().Len() > 0 {
				ils := rs.ScopeSpans().At(ilsIndex)
				if ils.Spans().Len() > 0 {
					return ils.Spans().At(spanIndex)
				}
			}
		}
		return ptrace.Span{}
	},
	"getMetric": func(metrics pmetric.Metrics, rmIndex, ilsIndex, metricIndex int) any {
		if metrics.ResourceMetrics().Len() > 0 {
			rm := metrics.ResourceMetrics().At(rmIndex)
			if rm.ScopeMetrics().Len() > 0 {
				ils := rm.ScopeMetrics().At(ilsIndex)
				if ils.Metrics().Len() > 0 {
					return ils.Metrics().At(metricIndex)
				}
			}
		}
		return pmetric.Metric{}
	},
	"getLogRecord": func(logs plog.Logs, rlIndex, ilsIndex, logIndex int) any {
		if logs.ResourceLogs().Len() > 0 {
			rl := logs.ResourceLogs().At(rlIndex)
			if rl.ScopeLogs().Len() > 0 {
				ils := rl.ScopeLogs().At(ilsIndex)
				if ils.LogRecords().Len() > 0 {
					return ils.LogRecords().At(logIndex)
				}
			}
		}
		return plog.LogRecord{}
	},
}

type azblobClient interface {
	UploadStream(ctx context.Context, containerName, blobName string, body io.Reader, o *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error)
	URL() string
	AppendBlock(ctx context.Context, containerName, blobName string, data []byte, o *appendblob.AppendBlockOptions) error
}

type azblobClientImpl struct {
	client *azblob.Client
}

func (c *azblobClientImpl) UploadStream(ctx context.Context, containerName, blobName string, body io.Reader, o *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error) {
	return c.client.UploadStream(ctx, containerName, blobName, body, o)
}

func (c *azblobClientImpl) URL() string {
	return c.client.URL()
}

func (c *azblobClientImpl) AppendBlock(ctx context.Context, containerName, blobName string, data []byte, o *appendblob.AppendBlockOptions) error {
	containerClient := c.client.ServiceClient().NewContainerClient(containerName)
	appendBlobClient := containerClient.NewAppendBlobClient(blobName)

	_, err := appendBlobClient.AppendBlock(ctx, newReadSeekCloserWrapper(data), o)
	if err == nil {
		return nil
	}

	// Handle BlobNotFound error by creating the blob and retrying
	var cerr *azcore.ResponseError
	if errors.As(err, &cerr) && cerr.ErrorCode == "BlobNotFound" {
		if _, err = appendBlobClient.Create(ctx, nil); err != nil {
			return fmt.Errorf("failed to create append blob: %w", err)
		}

		_, err = appendBlobClient.AppendBlock(ctx, newReadSeekCloserWrapper(data), o)
		if err != nil {
			return fmt.Errorf("failed to append block after creation: %w", err)
		}
		return nil
	}

	return fmt.Errorf("failed to append block: %w", err)
}

func newAzureBlobExporter(config *Config, logger *zap.Logger, signal pipeline.Signal) *azureBlobExporter {
	return &azureBlobExporter{
		config:           config,
		logger:           logger,
		signal:           signal,
		blobNameTemplate: &blobNameTemplate{},
		gzipWriterPool: &sync.Pool{
			New: func() any {
				// Create a new gzip writer that writes to a dummy buffer initially
				// It will be reset to the actual destination when used
				writer := gzip.NewWriter(io.Discard)
				return writer
			},
		},
		zstdWriterPool: &sync.Pool{
			New: func() any {
				// Create a new zstd writer that writes to a dummy buffer initially
				// It will be reset to the actual destination when used
				writer, err := zstd.NewWriter(io.Discard)
				if err != nil {
					logger.Error("failed to create zstd writer for pool, falling back to on-demand creation",
						zap.Error(err))
					// Return nil on error - sync.Pool will handle this gracefully
					return nil
				}
				return writer
			},
		},
	}
}

func randomInRange(low, hi int) int {
	return low + rand.IntN(hi-low)
}

func (e *azureBlobExporter) start(_ context.Context, host component.Host) error {
	var err error

	// create marshaller
	e.marshaller, err = newMarshaller(e.config, host)
	if err != nil {
		return err
	}

	// create client based on auth type
	authType := e.config.Auth.Type
	azblobClient := &azblobClientImpl{}
	e.client = azblobClient
	switch authType {
	case ConnectionString:
		azblobClient.client, err = azblob.NewClientFromConnectionString(e.config.Auth.ConnectionString, nil)
		if err != nil {
			return fmt.Errorf("failed to create client from connection string: %w", err)
		}
	case ServicePrincipal:
		cred, err := azidentity.NewClientSecretCredential(
			e.config.Auth.TenantID,
			e.config.Auth.ClientID,
			e.config.Auth.ClientSecret,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create service principal credential: %w", err)
		}
		azblobClient.client, err = azblob.NewClient(e.config.URL, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create client with service principal: %w", err)
		}
	case SystemManagedIdentity:
		cred, err := azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			return fmt.Errorf("failed to create system managed identity credential: %w", err)
		}
		azblobClient.client, err = azblob.NewClient(e.config.URL, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create client with system managed identity: %w", err)
		}
	case UserManagedIdentity:
		cred, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(e.config.Auth.ClientID),
		})
		if err != nil {
			return fmt.Errorf("failed to create user managed identity credential: %w", err)
		}
		azblobClient.client, err = azblob.NewClient(e.config.URL, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create client with user managed identity: %w", err)
		}
	case WorkloadIdentity:
		cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientID:      e.config.Auth.ClientID,
			TenantID:      e.config.Auth.TenantID,
			TokenFilePath: e.config.Auth.FederatedTokenFile,
		})
		if err != nil {
			return fmt.Errorf("failed to create workload identity credential: %w", err)
		}
		azblobClient.client, err = azblob.NewClient(e.config.URL, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create client with workload identity: %w", err)
		}
	default:
		return fmt.Errorf("unsupported authentication type: %s", authType)
	}

	if e.config.BlobNameFormat.TemplateEnabled {
		// pre-parse templates to catch error early
		e.blobNameTemplate = &blobNameTemplate{}
		var err error

		e.blobNameTemplate.metrics, err = template.New("metrics").Funcs(tempFuncs).Parse(e.config.BlobNameFormat.MetricsFormat)
		if err != nil {
			return fmt.Errorf("failed to parse metrics blob name template: %w", err)
		}

		e.blobNameTemplate.logs, err = template.New("logs").Funcs(tempFuncs).Parse(e.config.BlobNameFormat.LogsFormat)
		if err != nil {
			return fmt.Errorf("failed to parse logs blob name template: %w", err)
		}

		e.blobNameTemplate.traces, err = template.New("traces").Funcs(tempFuncs).Parse(e.config.BlobNameFormat.TracesFormat)
		if err != nil {
			return fmt.Errorf("failed to parse traces blob name template: %w", err)
		}
	}

	if tz := strings.TrimSpace(e.config.BlobNameFormat.Timezone); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Errorf("failed to load timezone: %w", err)
		}
		e.timeLocation = loc
	} else {
		e.timeLocation = nil
	}

	return nil
}

func (e *azureBlobExporter) generateBlobNameWithCompression(signal pipeline.Signal, telemetryData any) (string, error) {
	blobName, err := e.generateBlobName(signal, telemetryData)
	if err != nil {
		return "", err
	}

	// Append compression extension if configured. This must be done after generateBlobName
	// so that the base name (including serial number etc) is generated first.
	switch e.config.Compression {
	case configcompression.TypeGzip:
		blobName += ".gz"
	case configcompression.TypeZstd:
		blobName += ".zst"
	}

	return blobName, nil
}

func (e *azureBlobExporter) generateBlobName(signal pipeline.Signal, telemetryData any) (string, error) {
	// Get current time
	now := time.Now()

	if e.timeLocation != nil {
		now = now.In(e.timeLocation)
	}

	var format string
	var tmpl *template.Template
	switch signal {
	case pipeline.SignalMetrics:
		format = e.config.BlobNameFormat.MetricsFormat
		tmpl = e.blobNameTemplate.metrics
	case pipeline.SignalLogs:
		format = e.config.BlobNameFormat.LogsFormat
		tmpl = e.blobNameTemplate.logs
	case pipeline.SignalTraces:
		format = e.config.BlobNameFormat.TracesFormat
		tmpl = e.blobNameTemplate.traces
	default:
		return "", fmt.Errorf("unsupported signal type: %v", signal)
	}
	var blobName string

	// if template enabled, parse and apply template. if met error, fallback to default blob name format
	if e.config.BlobNameFormat.TemplateEnabled {
		// Parse and apply template with telemetry data
		var buf bytes.Buffer
		err := tmpl.Execute(&buf, telemetryData)
		if err != nil {
			e.logger.Warn("Failed to execute blob name template, using default blob name format", zap.Error(err))
		} else {
			blobName = buf.String()
			format = blobName
		}
	}

	if !e.config.BlobNameFormat.SerialNumEnabled {
		// No serial number enabled, return the formatted blob name
		return e.parseTimeInBlobName(now, format), nil
	}

	if e.config.BlobNameFormat.SerialNumBeforeExtension {
		// Append a random number and do so before the file extension if there is one
		ext := filepath.Ext(format)
		formatWithoutExt := strings.TrimSuffix(format, ext)
		randInt := randomInRange(0, int(e.config.BlobNameFormat.SerialNumRange))
		blobName = fmt.Sprintf("%s_%d%s", e.parseTimeInBlobName(now, formatWithoutExt), randInt, ext)
	} else {
		// Appends the random number after any potential file extension to minimize performance impact when high throughput
		blobName = fmt.Sprintf("%s_%d", e.parseTimeInBlobName(now, format), randomInRange(0, int(e.config.BlobNameFormat.SerialNumRange)))
	}

	return blobName, nil
}

func (e *azureBlobExporter) parseTimeInBlobName(now time.Time, format string) string {
	if !e.config.BlobNameFormat.TimeParserEnabled {
		return format
	}

	ranges := e.config.BlobNameFormat.TimeParserRanges
	if len(ranges) == 0 {
		// No ranges specified, parse entire string
		return now.Format(format)
	}

	// Parse only specified ranges
	result := []byte(format)
	for _, r := range ranges {
		start, end, err := parseRange(r)
		if err != nil {
			e.logger.Warn("Invalid time_parser_range, skipping", zap.String("range", r), zap.Error(err))
			continue
		}
		if start >= len(format) {
			continue
		}
		if end > len(format) {
			end = len(format)
		}
		parsed := now.Format(format[start:end])
		// Replace the range in result
		newResult := make([]byte, 0, len(result)-end+start+len(parsed))
		newResult = append(newResult, result[:start]...)
		newResult = append(newResult, parsed...)
		newResult = append(newResult, result[end:]...)
		result = newResult
	}
	return string(result)
}

func parseRange(r string) (int, int, error) {
	parts := strings.SplitN(r, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format: %s", r)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start index: %w", err)
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end index: %w", err)
	}
	if start < 0 || end < start {
		return 0, 0, fmt.Errorf("invalid range: start=%d end=%d", start, end)
	}
	return start, end, nil
}

func (*azureBlobExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *azureBlobExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Marshal the metrics data
	data, err := e.marshaller.marshalMetrics(md)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	return e.consumeData(ctx, md, data, pipeline.SignalMetrics)
}

func (e *azureBlobExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Marshal the logs data
	data, err := e.marshaller.marshalLogs(ld)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	return e.consumeData(ctx, ld, data, pipeline.SignalLogs)
}

func (e *azureBlobExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Marshal the trace data
	data, err := e.marshaller.marshalTraces(td)
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	return e.consumeData(ctx, td, data, pipeline.SignalTraces)
}

func (e *azureBlobExporter) consumeData(ctx context.Context, telemetryData any, data []byte, signal pipeline.Signal) error {
	// Generate a unique blob name
	blobName, err := e.generateBlobNameWithCompression(signal, telemetryData)
	if err != nil {
		return fmt.Errorf("failed to generate blobname: %w", err)
	}

	// Compress the content if compression is configured (note: for append_blob, compression is applied to each block)
	compressedData, err := e.compressContent(data)
	if err != nil {
		return fmt.Errorf("failed to compress content: %w", err)
	}
	data = compressedData

	var containerName string
	switch signal {
	case pipeline.SignalMetrics:
		containerName = e.config.Container.Metrics
	case pipeline.SignalLogs:
		containerName = e.config.Container.Logs
	case pipeline.SignalTraces:
		containerName = e.config.Container.Traces
	default:
		return fmt.Errorf("unsupported signal type: %v", signal)
	}

	if e.config.AppendBlob.Enabled {
		// Add separator if configured
		if e.config.AppendBlob.Separator != "" {
			data = append(data, []byte(e.config.AppendBlob.Separator)...)
		}
		err = e.client.AppendBlock(ctx, containerName, blobName, data, nil)
	} else {
		blobContentReader := bytes.NewReader(data)
		_, err = e.client.UploadStream(ctx, containerName, blobName, blobContentReader, nil)
	}

	if err != nil {
		return fmt.Errorf("failed to upload data: %w", err)
	}

	e.logger.Debug("Successfully exported data to Azure Blob Storage",
		zap.String("account", e.client.URL()),
		zap.String("container", containerName),
		zap.String("blob", blobName),
		zap.Int("size", len(data)))

	return nil
}

// compressContent compresses the data using the configured compression type.
// It uses a pool for reuse of compressors to reduce GC pressure.
func (e *azureBlobExporter) compressContent(raw []byte) ([]byte, error) {
	switch e.config.Compression {
	case configcompression.TypeGzip:
		return compress[*gzip.Writer](e.gzipWriterPool, raw, func(w io.Writer) (*gzip.Writer, error) {
			return gzip.NewWriter(w), nil
		})
	case configcompression.TypeZstd:
		return compress[*zstd.Encoder](e.zstdWriterPool, raw, func(w io.Writer) (*zstd.Encoder, error) {
			return zstd.NewWriter(w)
		})
	default:
		return raw, nil
	}
}

type poolItem interface {
	io.WriteCloser
	Reset(io.Writer)
}

func compress[T poolItem](pool *sync.Pool, raw []byte, newItem func(io.Writer) (T, error)) ([]byte, error) {
	if pool == nil {
		return nil, errors.New("unexpected: compress pool is nil")
	}

	content := bytes.NewBuffer(nil)

	// Get writer from pool or create new one
	pooled := pool.Get()
	var zipper T
	var fromPool bool
	if pooled != nil {
		if w, ok := pooled.(T); ok {
			zipper = w
			zipper.Reset(content)
			fromPool = true
		}
	}
	if !fromPool {
		var err error
		zipper, err = newItem(content)
		if err != nil {
			return nil, err
		}
	}

	// Write the data
	if _, err := zipper.Write(raw); err != nil {
		// Always close to release resources, but ignore close error on write failure
		_ = zipper.Close()
		return nil, err
	}

	// Close the writer
	if err := zipper.Close(); err != nil {
		return nil, err
	}

	// Only return the writer to the pool after successful write and close
	pool.Put(zipper)

	return content.Bytes(), nil
}

func newReadSeekCloserWrapper(data []byte) *readSeekCloserWrapper {
	return &readSeekCloserWrapper{bytes.NewReader(data)}
}

// readSeekCloserWrapper wraps a bytes.Reader to implement io.ReadSeekCloser
type readSeekCloserWrapper struct {
	*bytes.Reader
}

func (readSeekCloserWrapper) Close() error {
	return nil
}
