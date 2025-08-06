// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedlogreceiver"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
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
		config:   config,
		set:      set,
		consumer: consumerretry.NewLogs(config.RetryOnFailure, set.Logger, nextConsumer),
		obsrecv:  obsrecv,
	}, nil
}

// Start implements receiver.Logs
func (r *macosUnifiedLogReceiver) Start(ctx context.Context, host component.Host) error {
	// Load the encoding extension now that we have access to the host
	encodingExt, err := r.loadEncodingExtension(host)
	if err != nil {
		return fmt.Errorf("failed to load encoding extension: %w", err)
	}
	r.encodingExt = encodingExt

	// Create a cancellable context for the receiver
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	// Set up file consumer to watch traceV3 files
	fileConsumer, err := r.createFileConsumer(ctx)
	if err != nil {
		return fmt.Errorf("failed to create file consumer: %w", err)
	}
	r.fileConsumer = fileConsumer

	// Start the file consumer with no persister (in-memory only)
	if err := r.fileConsumer.Start(nil); err != nil {
		return fmt.Errorf("failed to start file consumer: %w", err)
	}

	r.set.Logger.Info("macOS Unified Logging receiver started")
	return nil
}

// Shutdown implements receiver.Logs
func (r *macosUnifiedLogReceiver) Shutdown(ctx context.Context) error {
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
func (r *macosUnifiedLogReceiver) createFileConsumer(ctx context.Context) (*fileconsumer.Manager, error) {
	// Create file consumer with our custom emit callback and no encoding (we handle it via extension)
	fcConfig := r.config.getFileConsumerConfig()
	return fcConfig.Build(
		r.set.TelemetrySettings,
		r.consumeTraceV3Tokens,
	)
}

// consumeTraceV3Tokens processes tokens (file chunks) from the file consumer
func (r *macosUnifiedLogReceiver) consumeTraceV3Tokens(ctx context.Context, tokens [][]byte, attributes map[string]any, lastRecordNumber int64, offsets []int64) error {
	obsrecvCtx := r.obsrecv.StartLogsOp(ctx)

	// For traceV3 files, we expect each token to be the entire file content
	// since traceV3 files are binary and should be processed as a whole
	for _, token := range tokens {
		// Use the encoding extension to unmarshal the traceV3 file content
		logs, err := r.encodingExt.UnmarshalLogs(token)
		if err != nil {
			r.set.Logger.Error("Failed to unmarshal traceV3 token",
				zap.String("file", fmt.Sprintf("%v", attributes["log.file.path"])),
				zap.Error(err))
			r.obsrecv.EndLogsOp(obsrecvCtx, "macos_unified_log", 0, err)
			continue
		}

		// Add file metadata to logs
		r.addFileMetadata(logs, attributes)

		// Send logs to the next consumer
		logRecordCount := logs.LogRecordCount()
		err = r.consumer.ConsumeLogs(ctx, logs)
		if err != nil {
			r.set.Logger.Error("Failed to consume logs", zap.Error(err))
		}

		r.obsrecv.EndLogsOp(obsrecvCtx, "macos_unified_log", logRecordCount, err)
	}

	return nil
}

// addFileMetadata adds file-related metadata to the logs
func (r *macosUnifiedLogReceiver) addFileMetadata(logs plog.Logs, attributes map[string]any) {
	// Add file metadata as resource attributes
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		resourceAttrs := resourceLogs.Resource().Attributes()

		// Add file path if available
		if filePath, ok := attributes["log.file.path"]; ok {
			resourceAttrs.PutStr("log.file.path", fmt.Sprintf("%v", filePath))
		}

		resourceAttrs.PutStr("log.file.format", "macos_unified_log_tracev3")
	}
}
