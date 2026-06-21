// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubexporter"

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// producerClient is the subset of azeventhubs.ProducerClient used by the exporter.
// Defined as an interface to allow test injection.
type producerClient interface {
	NewEventDataBatch(ctx context.Context, options *azeventhubs.EventDataBatchOptions) (*azeventhubs.EventDataBatch, error)
	SendEventDataBatch(ctx context.Context, batch *azeventhubs.EventDataBatch, options *azeventhubs.SendEventDataBatchOptions) error
	Close(ctx context.Context) error
}

type azureEventHubExporter struct {
	config   *Config
	producer producerClient
	logger   *zap.Logger
	// doSend is the function called by Consume* methods to dispatch a single
	// (partitionKey, body) pair. It defaults to e.send and can be replaced in
	// tests to capture output without a live Event Hub connection.
	doSend func(ctx context.Context, partitionKey string, body []byte) error
}

func newExporter(config *Config, logger *zap.Logger) *azureEventHubExporter {
	e := &azureEventHubExporter{
		config: config,
		logger: logger,
	}
	e.doSend = e.send
	return e
}

func (e *azureEventHubExporter) start(_ context.Context, host component.Host) error {
	if e.config.Auth != nil {
		if e.config.Connection != "" {
			e.logger.Warn("both 'auth' and 'connection' are specified; 'connection' will be ignored")
		}
		ext, ok := host.GetExtensions()[*e.config.Auth]
		if !ok {
			return fmt.Errorf("failed to resolve auth extension %q", *e.config.Auth)
		}
		credential, ok := ext.(azcore.TokenCredential)
		if !ok {
			return fmt.Errorf("extension %q does not implement azcore.TokenCredential", *e.config.Auth)
		}
		producer, err := azeventhubs.NewProducerClient(
			e.config.EventHub.Namespace,
			e.config.EventHub.Name,
			credential,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create Event Hub producer client: %w", err)
		}
		e.producer = producer
		return nil
	}

	// EventHub.Name overrides any EntityPath embedded in the connection string.
	producer, err := azeventhubs.NewProducerClientFromConnectionString(
		e.config.Connection,
		e.config.EventHub.Name,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create Event Hub producer client from connection string: %w", err)
	}
	e.producer = producer
	return nil
}

func (e *azureEventHubExporter) shutdown(ctx context.Context) error {
	if e.producer != nil {
		return e.producer.Close(ctx)
	}
	return nil
}

// ConsumeLogs splits the batch according to the configured partition strategy and
// sends each partition group as a separate Event Hub message.
func (e *azureEventHubExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for partitionKey, partialLogs := range e.partitionLogs(ld) {
		body, err := (&plog.JSONMarshaler{}).MarshalLogs(partialLogs)
		if err != nil {
			return fmt.Errorf("failed to marshal logs: %w", err)
		}
		if err = e.doSend(ctx, partitionKey, body); err != nil {
			return err
		}
	}
	return nil
}

// ConsumeMetrics splits the batch according to the configured partition strategy and
// sends each partition group as a separate Event Hub message.
func (e *azureEventHubExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for partitionKey, partialMetrics := range e.partitionMetrics(md) {
		body, err := (&pmetric.JSONMarshaler{}).MarshalMetrics(partialMetrics)
		if err != nil {
			return fmt.Errorf("failed to marshal metrics: %w", err)
		}
		if err = e.doSend(ctx, partitionKey, body); err != nil {
			return err
		}
	}
	return nil
}

// ConsumeTraces splits the batch according to the configured partition strategy and
// sends each partition group as a separate Event Hub message.
func (e *azureEventHubExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for partitionKey, partialTraces := range e.partitionTraces(td) {
		body, err := (&ptrace.JSONMarshaler{}).MarshalTraces(partialTraces)
		if err != nil {
			return fmt.Errorf("failed to marshal traces: %w", err)
		}
		if err = e.doSend(ctx, partitionKey, body); err != nil {
			return err
		}
	}
	return nil
}

// partitionLogs returns an iterator that yields (partitionKey, partial Logs) pairs.
// The partitioning strategy is determined by the config flags.
func (e *azureEventHubExporter) partitionLogs(ld plog.Logs) iter.Seq2[string, plog.Logs] {
	return func(yield func(string, plog.Logs) bool) {
		if e.config.PartitionLogsByResourceAttributes {
			// One message per resource; key = hash of resource attributes.
			newLogs := plog.NewLogs()
			target := newLogs.ResourceLogs().AppendEmpty()
			for _, resourceLogs := range ld.ResourceLogs().All() {
				hash := pdatautil.MapHash(resourceLogs.Resource().Attributes())
				resourceLogs.CopyTo(target)
				if !yield(string(hash[:]), newLogs) {
					return
				}
			}
			return
		}
		if e.config.PartitionLogsByTraceID {
			// One message per trace ID found in log records; key = trace ID hex string.
			for _, l := range batchpersignal.SplitLogs(ld) {
				traceID := l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID()
				key := traceutil.TraceIDToHexOrEmptyString(traceID)
				if !yield(key, l) {
					return
				}
			}
			return
		}
		// No partitioning: send the whole batch as a single message with no key.
		yield("", ld)
	}
}

// partitionMetrics returns an iterator that yields (partitionKey, partial Metrics) pairs.
func (e *azureEventHubExporter) partitionMetrics(md pmetric.Metrics) iter.Seq2[string, pmetric.Metrics] {
	return func(yield func(string, pmetric.Metrics) bool) {
		if e.config.PartitionMetricsByResourceAttributes {
			// One message per resource; key = hash of resource attributes.
			newMetrics := pmetric.NewMetrics()
			target := newMetrics.ResourceMetrics().AppendEmpty()
			for _, resourceMetrics := range md.ResourceMetrics().All() {
				hash := pdatautil.MapHash(resourceMetrics.Resource().Attributes())
				resourceMetrics.CopyTo(target)
				if !yield(string(hash[:]), newMetrics) {
					return
				}
			}
			return
		}
		yield("", md)
	}
}

// partitionTraces returns an iterator that yields (partitionKey, partial Traces) pairs.
func (e *azureEventHubExporter) partitionTraces(td ptrace.Traces) iter.Seq2[string, ptrace.Traces] {
	return func(yield func(string, ptrace.Traces) bool) {
		if e.config.PartitionTracesByID {
			// One message per trace; key = trace ID hex string.
			// batchpersignal.SplitTraces guarantees exactly one trace ID per returned value.
			for _, t := range batchpersignal.SplitTraces(td) {
				key := traceutil.TraceIDToHexOrEmptyString(
					t.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID(),
				)
				if !yield(key, t) {
					return
				}
			}
			return
		}
		yield("", td)
	}
}

// send creates an Event Hub batch optionally pinned to a partition key and sends it.
// An empty partitionKey means Event Hubs will pick the partition (round-robin).
func (e *azureEventHubExporter) send(ctx context.Context, partitionKey string, body []byte) error {
	var opts *azeventhubs.EventDataBatchOptions
	if partitionKey != "" {
		opts = &azeventhubs.EventDataBatchOptions{PartitionKey: &partitionKey}
	}

	batch, err := e.producer.NewEventDataBatch(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to create event data batch: %w", err)
	}

	if err = batch.AddEventData(&azeventhubs.EventData{Body: body}, nil); err != nil {
		if errors.Is(err, azeventhubs.ErrEventDataTooLarge) {
			return fmt.Errorf("telemetry payload (%d bytes) exceeds the maximum Event Hub message size: reduce upstream batch size", len(body))
		}
		return fmt.Errorf("failed to add event data to batch: %w", err)
	}

	if err = e.producer.SendEventDataBatch(ctx, batch, nil); err != nil {
		return fmt.Errorf("failed to send event data batch: %w", err)
	}
	return nil
}
