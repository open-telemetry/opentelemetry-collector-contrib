// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter"

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

type azureEventHubsExporter struct {
	config     *Config
	logger     *zap.Logger
	client     eventHubProducerClient
	signal     pipeline.Signal
	marshaller marshaller
}

// newExporter creates a new Azure Event Hubs exporter
func newExporter(config *Config, set component.TelemetrySettings, signal pipeline.Signal) (*azureEventHubsExporter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	marshaller, err := createMarshaller(config)
	if err != nil {
		return nil, err
	}

	exp := &azureEventHubsExporter{
		config:     config,
		logger:     set.Logger,
		signal:     signal,
		marshaller: marshaller,
	}

	return exp, nil
}

func createMarshaller(config *Config) (marshaller, error) {
	switch config.FormatType {
	case formatTypeJSON:
		return newJSONMarshaller(), nil
	case formatTypeProto:
		return newProtoMarshaller(), nil
	default:
		return nil, fmt.Errorf("unsupported format type: %s", config.FormatType)
	}
}

func (e *azureEventHubsExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.createEventHubsClient()
	if err != nil {
		return fmt.Errorf("failed to create Event Hubs client: %w", err)
	}
	e.client = client
	return nil
}

func (e *azureEventHubsExporter) shutdown(ctx context.Context) error {
	if e.client != nil {
		return e.client.Close(ctx)
	}
	return nil
}

func (e *azureEventHubsExporter) createEventHubsClient() (eventHubProducerClient, error) {
	var eventHubName string
	switch e.signal {
	case pipeline.SignalTraces:
		eventHubName = e.config.EventHub.Traces
	case pipeline.SignalLogs:
		eventHubName = e.config.EventHub.Logs
	case pipeline.SignalMetrics:
		eventHubName = e.config.EventHub.Metrics
	default:
		return nil, fmt.Errorf("unsupported signal type: %v", e.signal)
	}

	var azureClient *azeventhubs.ProducerClient
	var err error

	switch e.config.Auth.Type {
	case ConnectionString:
		azureClient, err = azeventhubs.NewProducerClientFromConnectionString(
			e.config.Auth.ConnectionString,
			eventHubName,
			nil,
		)
	case DefaultCredentials:
		cred, credErr := azidentity.NewDefaultAzureCredential(nil)
		if credErr != nil {
			return nil, fmt.Errorf("failed to create default credentials: %w", credErr)
		}
		azureClient, err = azeventhubs.NewProducerClient(
			e.config.Namespace,
			eventHubName,
			cred,
			nil,
		)
	case SystemManagedIdentity:
		cred, credErr := azidentity.NewManagedIdentityCredential(nil)
		if credErr != nil {
			return nil, fmt.Errorf("failed to create managed identity credential: %w", credErr)
		}
		azureClient, err = azeventhubs.NewProducerClient(
			e.config.Namespace,
			eventHubName,
			cred,
			nil,
		)
	case UserManagedIdentity:
		cred, credErr := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(e.config.Auth.ClientID),
		})
		if credErr != nil {
			return nil, fmt.Errorf("failed to create user managed identity credential: %w", credErr)
		}
		azureClient, err = azeventhubs.NewProducerClient(
			e.config.Namespace,
			eventHubName,
			cred,
			nil,
		)
	case ServicePrincipal:
		cred, credErr := azidentity.NewClientSecretCredential(
			e.config.Auth.TenantID,
			e.config.Auth.ClientID,
			e.config.Auth.ClientSecret,
			nil,
		)
		if credErr != nil {
			return nil, fmt.Errorf("failed to create service principal credential: %w", credErr)
		}
		azureClient, err = azeventhubs.NewProducerClient(
			e.config.Namespace,
			eventHubName,
			cred,
			nil,
		)
	case WorkloadIdentity:
		cred, credErr := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			TenantID:      e.config.Auth.TenantID,
			ClientID:      e.config.Auth.ClientID,
			TokenFilePath: e.config.Auth.FederatedTokenFile,
		})
		if credErr != nil {
			return nil, fmt.Errorf("failed to create workload identity credential: %w", credErr)
		}
		azureClient, err = azeventhubs.NewProducerClient(
			e.config.Namespace,
			eventHubName,
			cred,
			nil,
		)
	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", e.config.Auth.Type)
	}

	if err != nil {
		return nil, err
	}

	// Wrap the Azure SDK client to implement our interface
	return &azureEventHubProducerClientWrapper{client: azureClient}, nil
}

func (e *azureEventHubsExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	data, err := e.marshaller.MarshalTraces(td)
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	partitionKey := e.generatePartitionKey(td.ResourceSpans())
	return e.sendEvent(ctx, data, partitionKey)
}

func (e *azureEventHubsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	data, err := e.marshaller.MarshalLogs(ld)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	partitionKey := e.generatePartitionKeyFromLogs(ld.ResourceLogs())
	return e.sendEvent(ctx, data, partitionKey)
}

func (e *azureEventHubsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	data, err := e.marshaller.MarshalMetrics(md)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	partitionKey := e.generatePartitionKeyFromMetrics(md.ResourceMetrics())
	return e.sendEvent(ctx, data, partitionKey)
}

func (e *azureEventHubsExporter) sendEvent(ctx context.Context, data []byte, partitionKey string) error {
	if len(data) > e.config.MaxEventSize {
		return fmt.Errorf("event size %d exceeds maximum allowed size %d", len(data), e.config.MaxEventSize)
	}

	eventData := &azeventhubs.EventData{
		Body: data,
	}

	var options *azeventhubs.EventDataBatchOptions
	if partitionKey != "" {
		options = &azeventhubs.EventDataBatchOptions{
			PartitionKey: &partitionKey,
		}
	}

	// Create a batch with a single event
	batch, err := e.client.NewEventDataBatch(ctx, options)
	if err != nil {
		return fmt.Errorf("failed to create event batch: %w", err)
	}

	err = batch.AddEventData(eventData, nil)
	if err != nil {
		return fmt.Errorf("failed to add event to batch: %w", err)
	}

	err = e.client.SendEventDataBatch(ctx, batch, nil)
	if err != nil {
		return fmt.Errorf("failed to send event batch: %w", err)
	}

	return nil
}

func (e *azureEventHubsExporter) generatePartitionKey(resourceSpans ptrace.ResourceSpansSlice) string {
	switch e.config.PartitionKey.Source {
	case "static":
		return e.config.PartitionKey.Value
	case "resource_attribute":
		if resourceSpans.Len() > 0 {
			attrs := resourceSpans.At(0).Resource().Attributes()
			if val, ok := attrs.Get(e.config.PartitionKey.Value); ok {
				return val.AsString()
			}
		}
		return ""
	case "trace_id":
		if resourceSpans.Len() > 0 {
			scopeSpans := resourceSpans.At(0).ScopeSpans()
			if scopeSpans.Len() > 0 {
				spans := scopeSpans.At(0).Spans()
				if spans.Len() > 0 {
					return spans.At(0).TraceID().String()
				}
			}
		}
		return ""
	case "span_id":
		if resourceSpans.Len() > 0 {
			scopeSpans := resourceSpans.At(0).ScopeSpans()
			if scopeSpans.Len() > 0 {
				spans := scopeSpans.At(0).Spans()
				if spans.Len() > 0 {
					return spans.At(0).SpanID().String()
				}
			}
		}
		return ""
	case "random":
		return e.generateRandomPartitionKey()
	default:
		return ""
	}
}

func (e *azureEventHubsExporter) generatePartitionKeyFromLogs(resourceLogs plog.ResourceLogsSlice) string {
	switch e.config.PartitionKey.Source {
	case "static":
		return e.config.PartitionKey.Value
	case "resource_attribute":
		if resourceLogs.Len() > 0 {
			attrs := resourceLogs.At(0).Resource().Attributes()
			if val, ok := attrs.Get(e.config.PartitionKey.Value); ok {
				return val.AsString()
			}
		}
		return ""
	case "trace_id":
		if resourceLogs.Len() > 0 {
			scopeLogs := resourceLogs.At(0).ScopeLogs()
			if scopeLogs.Len() > 0 {
				logRecords := scopeLogs.At(0).LogRecords()
				if logRecords.Len() > 0 {
					return logRecords.At(0).TraceID().String()
				}
			}
		}
		return ""
	case "random":
		return e.generateRandomPartitionKey()
	default:
		return ""
	}
}

func (e *azureEventHubsExporter) generatePartitionKeyFromMetrics(resourceMetrics pmetric.ResourceMetricsSlice) string {
	switch e.config.PartitionKey.Source {
	case "static":
		return e.config.PartitionKey.Value
	case "resource_attribute":
		if resourceMetrics.Len() > 0 {
			attrs := resourceMetrics.At(0).Resource().Attributes()
			if val, ok := attrs.Get(e.config.PartitionKey.Value); ok {
				return val.AsString()
			}
		}
		return ""
	case "random":
		return e.generateRandomPartitionKey()
	default:
		return ""
	}
}

func (e *azureEventHubsExporter) generateRandomPartitionKey() string {
	// Generate a hash-based partition key for consistent distribution
	h := fnv.New32a()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	h.Write(randomBytes)
	return hex.EncodeToString(h.Sum(nil))
}
