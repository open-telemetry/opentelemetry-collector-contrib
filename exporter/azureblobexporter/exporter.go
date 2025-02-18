// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"text/template"
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
	config           *Config
	logger           *zap.Logger
	client           *azblob.Client
	signal           pipeline.Signal
	marshaller       *marshaller
	blobNameTemplate *template.Template
}

var fileExtensionMap = map[string]string{
	formatTypeJSON:  "json",
	formatTypeProto: "pb",
}

func newAzureBlobExporter(config *Config, logger *zap.Logger, signal pipeline.Signal) *azureBlobExporter {
	azBlobExporter := &azureBlobExporter{
		config: config,
		logger: logger,
		signal: signal,
	}
	return azBlobExporter
}

func randomInRange(low, hi int) int {
	return low + rand.Intn(hi-low)
}

func (e *azureBlobExporter) start(_ context.Context, host component.Host) error {
	var err error

	// create marshaller
	e.marshaller, err = newMarshaller(e.config, host)
	if err != nil {
		return err
	}

	// create blob name template
	switch e.signal {
	case pipeline.SignalMetrics:
		e.blobNameTemplate, err = template.New("blobName").Parse(e.config.BlobNameFormat.MetricsFormat)
	case pipeline.SignalLogs:
		e.blobNameTemplate, err = template.New("blobName").Parse(e.config.BlobNameFormat.LogsFormat)
	case pipeline.SignalTraces:
		e.blobNameTemplate, err = template.New("blobName").Parse(e.config.BlobNameFormat.TracesFormat)
	default:
		return fmt.Errorf("unsupported signal type: %v", e.signal)
	}

	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
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

func (e *azureBlobExporter) generateBlobName(data map[string]interface{}) (string, error) {
	// Get current time
	now := time.Now()

	// Execute the template
	var result bytes.Buffer
	if err := e.blobNameTemplate.Execute(&result, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	blobStr := result.String()
	return now.Format(blobStr), nil
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

	// Generate a unique blob name
	params := map[string]interface{}{
		"FileExtension": fileExtensionMap[e.config.FormatType],
		"SerialNum":     randomInRange(1, int(e.config.BlobNameFormat.SerialNumRange)),
	}
	blobName, err := e.generateBlobName(params)
	if err != nil {
		return fmt.Errorf("failed to generate blobname: %w", err)
	}

	_, err = e.client.UploadStream(ctx, e.config.Container.Metrics, blobName, bytes.NewReader(data), nil)
	if err != nil {
		return fmt.Errorf("failed to upload metrics data: %w", err)
	}

	e.logger.Info("Successfully exported metrics to Azure Blob Storage",
		zap.String("container", e.config.Container.Metrics),
		zap.String("blob", blobName),
		zap.Int("size", len(data)))

	return nil
}

func (e *azureBlobExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Marshal the metrics data
	data, err := e.marshaller.marshalLogs(ld)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	// Generate a unique blob name
	params := map[string]interface{}{
		"FileExtension": fileExtensionMap[e.config.FormatType],
		"SerialNum":     randomInRange(1, int(e.config.BlobNameFormat.SerialNumRange)),
	}
	blobName, err := e.generateBlobName(params)
	if err != nil {
		return fmt.Errorf("failed to generate blobname: %w", err)
	}

	_, err = e.client.UploadStream(ctx, e.config.Container.Logs, blobName, bytes.NewReader(data), nil)
	if err != nil {
		return fmt.Errorf("failed to upload logs data: %w", err)
	}

	e.logger.Info("Successfully exported logs to Azure Blob Storage",
		zap.String("container", e.config.Container.Logs),
		zap.String("blob", blobName),
		zap.Int("size", len(data)))

	return nil
}

func (e *azureBlobExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Marshal the metrics data
	data, err := e.marshaller.marshalTraces(td)
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	// Generate a unique blob name
	params := map[string]interface{}{
		"FileExtension": fileExtensionMap[e.config.FormatType],
		"SerialNum":     randomInRange(1, int(e.config.BlobNameFormat.SerialNumRange)),
	}
	blobName, err := e.generateBlobName(params)
	if err != nil {
		return fmt.Errorf("failed to generate blobname: %w", err)
	}

	blobContentReader := bytes.NewReader(data)
	_, err = e.client.UploadStream(context.TODO(), e.config.Container.Traces, blobName, blobContentReader, nil)
	if err != nil {
		return fmt.Errorf("failed to upload traces data: %w", err)
	}

	e.logger.Info("Successfully exported traces to Azure Blob Storage",
		zap.String("container", e.config.Container.Traces),
		zap.String("blob", blobName),
		zap.Int("size", len(data)))

	return nil
}
