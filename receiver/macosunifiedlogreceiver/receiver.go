// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedlogreceiver"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
)

// macosUnifiedLogReceiver implements receiver.Logs for macOS Unified Logging traceV3 files
type macosUnifiedLogReceiver struct {
	config   *Config
	set      receiver.Settings
	consumer consumer.Logs
	obsrecv  *receiverhelper.ObsReport

	// File consumer for watching and reading traceV3 files
	fileConsumer *fileconsumer.Manager

	// Encoding extension for decoding traceV3 files
	encodingExt encoding.LogsUnmarshalerExtension

	// Context for cancellation
	cancel context.CancelFunc

	// Map to track processed files
	processedFiles map[string]struct{}
	mu             sync.Mutex
}

// newMacOSUnifiedLogReceiver creates a new macOS Unified Logging receiver
func newMacOSUnifiedLogReceiver(
	config *Config,
	set receiver.Settings,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	return &macosUnifiedLogReceiver{
		config:         config,
		set:            set,
		consumer:       consumerretry.NewLogs(config.RetryOnFailure, set.Logger, nextConsumer),
		obsrecv:        obsrecv,
		processedFiles: make(map[string]struct{}),
	}, nil
}

// Start implements receiver.Logs
func (r *macosUnifiedLogReceiver) Start(ctx context.Context, host component.Host) error {
	r.set.Logger.Info("Starting macOS Unified Log receiver")

	// Load the encoding extension
	encodingExt, err := r.loadEncodingExtension(host)
	if err != nil {
		r.set.Logger.Error("Failed to load encoding extension", zap.Error(err))
		return err
	}
	r.encodingExt = encodingExt
	r.set.Logger.Info("Encoding extension loaded successfully")

	// Create the file consumer
	fileConsumer, err := r.createFileConsumer(ctx)
	if err != nil {
		r.set.Logger.Error("Failed to create file consumer", zap.Error(err))
		return err
	}
	r.fileConsumer = fileConsumer
	r.set.Logger.Info("File consumer created successfully")

	// Start the file consumer
	err = r.fileConsumer.Start(nil)
	if err != nil {
		r.set.Logger.Error("Failed to start file consumer", zap.Error(err))
		return err
	}
	r.set.Logger.Info("File consumer started successfully")

	return nil
}

// Shutdown implements receiver.Logs
func (r *macosUnifiedLogReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}

	if r.fileConsumer != nil {
		if err := r.fileConsumer.Stop(); err != nil {
			r.set.Logger.Error("Error stopping file consumer", zap.Error(err))
			return err
		}
	}

	r.set.Logger.Info("macOS Unified Logging receiver stopped")
	return nil
}

// loadEncodingExtension loads the specified encoding extension from the host
func (r *macosUnifiedLogReceiver) loadEncodingExtension(host component.Host) (encoding.LogsUnmarshalerExtension, error) {
	// Parse encoding name as component ID
	var encodingID component.ID
	if err := encodingID.UnmarshalText([]byte(r.config.Encoding)); err != nil {
		return nil, fmt.Errorf("invalid encoding ID %q: %w", r.config.Encoding, err)
	}

	// Get extension from host
	extensions := host.GetExtensions()
	ext, ok := extensions[encodingID]
	if !ok {
		return nil, fmt.Errorf("encoding extension %q not found", r.config.Encoding)
	}

	// Cast to logs unmarshaler extension
	logsUnmarshaler, ok := ext.(encoding.LogsUnmarshalerExtension)
	if !ok {
		return nil, errors.New("extension is not a logs unmarshaler")
	}

	return logsUnmarshaler, nil
}

// createFileConsumer creates and configures the file consumer for traceV3 files
func (r *macosUnifiedLogReceiver) createFileConsumer(_ context.Context) (*fileconsumer.Manager, error) {
	r.set.Logger.Info("Creating file consumer", zap.Any("config", r.config))

	// Get the file consumer configuration
	fcConfig := r.config.getFileConsumerConfig()

	// Create file consumer with our custom emit callback and no encoding (we handle it via extension)
	fileConsumer, err := fcConfig.Build(
		r.set.TelemetrySettings,
		r.consumeTraceV3Tokens,
	)
	if err != nil {
		r.set.Logger.Error("Failed to create file consumer", zap.Error(err))
		return nil, fmt.Errorf("failed to create file consumer: %w", err)
	}

	r.set.Logger.Info("File consumer created successfully")
	return fileConsumer, nil
}

// consumeTraceV3Tokens processes tokens (file chunks) from the file consumer
func (r *macosUnifiedLogReceiver) consumeTraceV3Tokens(ctx context.Context, tokens [][]byte, attributes map[string]any, _ int64, _ []int64) error {
	obsrecvCtx := r.obsrecv.StartLogsOp(ctx)

	// Debug: Log all available attributes
	r.set.Logger.Debug("Received attributes", zap.Any("attributes", attributes))
	r.set.Logger.Info("Processing traceV3 tokens", zap.Int("tokenCount", len(tokens)), zap.Any("attributes", attributes))

	// Get the file path from attributes
	filePath := "unknown"
	if fp, ok := attributes["log.file.path"]; ok {
		filePath = fmt.Sprintf("%v", fp)
	}

	// Check if we've already processed this file
	r.mu.Lock()
	if _, exists := r.processedFiles[filePath]; exists {
		r.mu.Unlock()
		// File already processed, skip
		r.obsrecv.EndLogsOp(obsrecvCtx, "macos_unified_log", 0, nil)
		return nil
	}
	// Mark this file as processed
	r.processedFiles[filePath] = struct{}{}
	r.mu.Unlock()

	// Calculate total size of all tokens for this file
	totalSize := 0
	for _, token := range tokens {
		totalSize += len(token)
	}

	// Process each token (file chunk) through the encoding extension
	var allLogs plog.Logs
	totalLogRecords := 0

	for i, token := range tokens {
		r.set.Logger.Debug("Processing token", zap.Int("tokenIndex", i), zap.Int("tokenSize", len(token)))

		// Use the encoding extension to decode the binary token
		decodedLogs, err := r.encodingExt.UnmarshalLogs(token)
		if err != nil {
			r.set.Logger.Error("Failed to decode token with encoding extension",
				zap.Error(err), zap.Int("tokenIndex", i), zap.Int("tokenSize", len(token)))
			continue
		}

		// Add file metadata as resource attributes to all decoded logs
		for j := 0; j < decodedLogs.ResourceLogs().Len(); j++ {
			resourceLogs := decodedLogs.ResourceLogs().At(j)
			resource := resourceLogs.Resource()
			resource.Attributes().PutStr("log.file.path", filePath)
			resource.Attributes().PutStr("log.file.format", "macos_unified_log_tracev3")
		}

		// If this is the first token, initialize allLogs
		if i == 0 {
			allLogs = decodedLogs
		} else {
			// Append additional logs to the first set
			for j := 0; j < decodedLogs.ResourceLogs().Len(); j++ {
				decodedResourceLogs := decodedLogs.ResourceLogs().At(j)
				decodedResourceLogs.CopyTo(allLogs.ResourceLogs().AppendEmpty())
			}
		}

		totalLogRecords += decodedLogs.LogRecordCount()
	}

	// If no logs were created, create a fallback entry
	if totalLogRecords == 0 {
		allLogs = plog.NewLogs()
		resourceLogs := allLogs.ResourceLogs().AppendEmpty()
		resource := resourceLogs.Resource()
		resource.Attributes().PutStr("log.file.path", filePath)
		resource.Attributes().PutStr("log.file.format", "macos_unified_log_tracev3")

		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		logRecord := scopeLogs.LogRecords().AppendEmpty()
		r.setLogRecordAttributes(&logRecord, totalSize, len(tokens))
		logRecord.Body().SetStr(fmt.Sprintf("No logs decoded from traceV3 file: %s (size: %d bytes, tokens: %d)", filePath, totalSize, len(tokens)))
		totalLogRecords = 1
	}

	// Send all decoded logs to the consumer
	err := r.consumer.ConsumeLogs(ctx, allLogs)
	if err != nil {
		r.set.Logger.Error("Failed to consume logs", zap.Error(err))
	}

	r.set.Logger.Info("Successfully processed traceV3 file",
		zap.String("filePath", filePath),
		zap.Int("totalSize", totalSize),
		zap.Int("tokens", len(tokens)),
		zap.Int("logRecords", totalLogRecords))

	r.obsrecv.EndLogsOp(obsrecvCtx, "macos_unified_log", totalLogRecords, err)

	return nil
}

func (*macosUnifiedLogReceiver) setLogRecordAttributes(logRecord *plog.LogRecord, totalSize, lenTokens int) {
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText("INFO")

	// Add file information as attributes
	logRecord.Attributes().PutInt("file.total.size", int64(totalSize))
	logRecord.Attributes().PutInt("file.token.count", int64(lenTokens))
	logRecord.Attributes().PutStr("receiver.name", "macosunifiedlog")
}
