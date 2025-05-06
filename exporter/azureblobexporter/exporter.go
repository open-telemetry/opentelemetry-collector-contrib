// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/appendblob"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type azureBlobExporter struct {
	config     *Config
	logger     *zap.Logger
	client     azblobClient
	signal     pipeline.Signal
	marshaller *marshaller
}

type azblobClient interface {
	UploadStream(ctx context.Context, containerName string, blobName string, body io.Reader, o *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error)
	URL() string
	AppendBlock(ctx context.Context, containerName string, blobName string, data []byte, o *appendblob.AppendBlockOptions) error
}

type azblobClientImpl struct {
	client *azblob.Client
}

func (c *azblobClientImpl) UploadStream(ctx context.Context, containerName string, blobName string, body io.Reader, o *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error) {
	return c.client.UploadStream(ctx, containerName, blobName, body, o)
}

func (c *azblobClientImpl) URL() string {
	return c.client.URL()
}

func (c *azblobClientImpl) AppendBlock(ctx context.Context, containerName string, blobName string, data []byte, o *appendblob.AppendBlockOptions) error {
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
		config: config,
		logger: logger,
		signal: signal,
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
			nil)
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
	default:
		return fmt.Errorf("unsupported authentication type: %s", authType)
	}
	return nil
}

func (e *azureBlobExporter) generateBlobName(signal pipeline.Signal) (string, error) {
	// Get current time
	now := time.Now()

	var format string
	switch signal {
	case pipeline.SignalMetrics:
		format = e.config.BlobNameFormat.MetricsFormat
	case pipeline.SignalLogs:
		format = e.config.BlobNameFormat.LogsFormat
	case pipeline.SignalTraces:
		format = e.config.BlobNameFormat.TracesFormat
	default:
		return "", fmt.Errorf("unsupported signal type: %v", signal)
	}
	var blobName string
	if e.config.BlobNameFormat.SerialNumBeforeExtension {
		// Append a random number and do so before the file extension if there is one
		ext := filepath.Ext(format)
		formatWithoutExt := strings.TrimSuffix(format, ext)
		randInt := randomInRange(0, int(e.config.BlobNameFormat.SerialNumRange))
		blobName = fmt.Sprintf("%s_%d%s", now.Format(formatWithoutExt), randInt, ext)
	} else {
		// Appends the random number after any potential file extension to minimize performance impact when high throughput
		blobName = fmt.Sprintf("%s_%d", now.Format(format), randomInRange(0, int(e.config.BlobNameFormat.SerialNumRange)))
	}
	return blobName, nil
}

func (e *azureBlobExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *azureBlobExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Marshal the metrics data
	data, err := e.marshaller.marshalMetrics(md)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	return e.consumeData(ctx, data, pipeline.SignalMetrics)
}

func (e *azureBlobExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Marshal the logs data
	data, err := e.marshaller.marshalLogs(ld)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	return e.consumeData(ctx, data, pipeline.SignalLogs)
}

func (e *azureBlobExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Marshal the trace data
	data, err := e.marshaller.marshalTraces(td)
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	return e.consumeData(ctx, data, pipeline.SignalTraces)
}

func (e *azureBlobExporter) consumeData(ctx context.Context, data []byte, signal pipeline.Signal) error {
	// Generate a unique blob name
	blobName, err := e.generateBlobName(signal)
	if err != nil {
		return fmt.Errorf("failed to generate blobname: %w", err)
	}

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

	if e.config.AppendBlob != nil && e.config.AppendBlob.Enabled {
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

func newReadSeekCloserWrapper(data []byte) *readSeekCloserWrapper {
	return &readSeekCloserWrapper{bytes.NewReader(data)}
}

// readSeekCloserWrapper wraps a bytes.Reader to implement io.ReadSeekCloser
type readSeekCloserWrapper struct {
	*bytes.Reader
}

func (r readSeekCloserWrapper) Close() error {
	return nil
}
