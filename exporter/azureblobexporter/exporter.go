// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
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
	switch authType {
	case ConnectionString:
		e.client, err = azblob.NewClientFromConnectionString(e.config.Auth.ConnectionString, nil)
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
		e.client, err = azblob.NewClient(e.config.URL, cred, nil)
		if err != nil {
			return fmt.Errorf("failed to create client with service principal: %w", err)
		}
	case SystemManagedIdentity:
		cred, err := azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			return fmt.Errorf("failed to create system managed identity credential: %w", err)
		}
		e.client, err = azblob.NewClient(e.config.URL, cred, nil)
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
		e.client, err = azblob.NewClient(e.config.URL, cred, nil)
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
	return fmt.Sprintf("%s_%d", now.Format(format), randomInRange(1, int(e.config.BlobNameFormat.SerialNumRange))), nil
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

	blobContentReader := bytes.NewReader(data)
	_, err = e.client.UploadStream(ctx, e.config.Container.Traces, blobName, blobContentReader, nil)
	if err != nil {
		return fmt.Errorf("failed to upload traces data: %w", err)
	}

	e.logger.Debug("Successfully exported data to Azure Blob Storage",
		zap.String("account", e.client.URL()),
		zap.String("container", e.config.Container.Traces),
		zap.String("blob", blobName),
		zap.Int("size", len(data)))

	return nil
}
