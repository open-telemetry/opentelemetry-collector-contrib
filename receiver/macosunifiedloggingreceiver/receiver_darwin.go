// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package macosunifiedloggingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver"

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// unifiedLoggingReceiver uses exec.Command to run the native macOS `log` command
type unifiedLoggingReceiver struct {
	config   *Config
	logger   *zap.Logger
	consumer consumer.Logs
	cancel   context.CancelFunc
}

func newUnifiedLoggingReceiver(
	config *Config,
	logger *zap.Logger,
	consumer consumer.Logs,
) *unifiedLoggingReceiver {
	return &unifiedLoggingReceiver{
		config:   config,
		logger:   logger,
		consumer: consumer,
	}
}

func (r *unifiedLoggingReceiver) Start(ctx context.Context, _ component.Host) error {
	r.logger.Info("Starting macOS unified logging receiver")

	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	// Start reading logs in a goroutine
	go r.readLogs(ctx)

	return nil
}

func (r *unifiedLoggingReceiver) Shutdown(_ context.Context) error {
	r.logger.Info("Shutting down macOS unified logging receiver")
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

// readLogs runs the log command and processes output
func (r *unifiedLoggingReceiver) readLogs(ctx context.Context) {
	// Run immediately on startup
	if r.config.ArchivePath == "" {
		r.readFromLive(ctx)
	} else {
		r.readFromArchive(ctx)
	}
}

func (r *unifiedLoggingReceiver) readFromArchive(ctx context.Context) {
	resolvedPaths := r.config.resolvedArchivePaths
	r.logger.Info("Reading from archive mode", zap.Int("archive_count", len(resolvedPaths)))

	wg := &sync.WaitGroup{}
	for _, archivePath := range resolvedPaths {
		wg.Add(1)
		go func(archivePath string) {
			defer wg.Done()
			r.logger.Info("Processing archive", zap.String("path", archivePath))
			if _, err := r.runLogCommand(ctx, archivePath); err != nil {
				r.logger.Error("Failed to run log command for archive", zap.String("archive", archivePath), zap.Error(err))
				return
			}
		}(archivePath)
	}
	wg.Wait()
	r.logger.Info("Finished reading archive logs")
}

func (r *unifiedLoggingReceiver) readFromLive(ctx context.Context) {
	// Run immediately on startup
	_, err := r.runLogCommand(ctx, "")
	if err != nil {
		r.logger.Error("Failed to run log command", zap.Error(err))
		return
	}

	// For live mode, use exponential backoff based on whether logs are being actively written
	// We cannot safely reset the backoff while ticker is running (causes data race)
	// Instead, track interval manually and use time.After which creates a new timer each iteration
	minInterval := 100 * time.Millisecond
	maxInterval := r.config.MaxPollInterval
	currentInterval := time.Duration(0) // Start immediately

	// Run immediately on start, then use backoff for subsequent iterations
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(currentInterval):
		}

		// Run the log command
		count, err := r.runLogCommand(ctx, "")
		if err != nil {
			r.logger.Error("Failed to run log command", zap.Error(err))
		}

		// Adjust interval based on whether logs were found
		if count > 0 {
			r.logger.Debug("Logs found, resetting poll interval to minimum", zap.Int("count", count))
			currentInterval = minInterval
		} else {
			// If no logs, exponentially increase interval up to MaxPollInterval
			if currentInterval == 0 {
				// First iteration with no logs, start with min interval
				currentInterval = minInterval
			} else {
				nextInterval := currentInterval * 2
				currentInterval = min(nextInterval, maxInterval)
			}
		}
	}
}

// runLogCommand executes the log command and processes output
// Returns the number of logs processed
// archivePath should be empty string for live mode, or a specific archive path for archive mode
func (r *unifiedLoggingReceiver) runLogCommand(ctx context.Context, archivePath string) (int, error) {
	// Build the log command arguments
	args := r.buildLogCommandArgs(archivePath)

	r.logger.Info("Running log command", zap.Strings("args", args))

	// Create the command
	cmd := exec.CommandContext(ctx, "log", args...) // #nosec G204 - args are controlled by config

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start log command: %w", err)
	}

	// Ensure the process is properly cleaned up to avoid zombies
	defer func() {
		_ = cmd.Wait()
	}()

	// Read and process output line by line
	scanner := bufio.NewScanner(stdout)
	// Set a large buffer size for long log lines
	buf := make([]byte, 0, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, 10*1024*1024) // 10MB max

	var processedCount int
	isFirstLine := true
	// Skip the header line in text-based formats (default, syslog, compact)
	isTextFormat := r.config.Format == "default" || r.config.Format == "syslog" || r.config.Format == "compact"
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			err := cmd.Process.Kill()
			if err != nil {
				r.logger.Error("Failed to kill log command", zap.Error(err))
			}
			return processedCount, ctx.Err()
		default:
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			if isTextFormat && isFirstLine {
				isFirstLine = false
				continue
			}
			isFirstLine = false

			// Skip completion/status messages (applies to all formats)
			if isCompletionLine(line) {
				continue
			}

			// Parse and send the log entry
			if err := r.processLogLine(ctx, line); err != nil {
				r.logger.Warn("Failed to process log line",
					zap.Error(err))
				continue
			}
			processedCount++
		}
	}

	if err := scanner.Err(); err != nil {
		return processedCount, fmt.Errorf("error reading log output: %w", err)
	}

	r.logger.Debug("Processed logs", zap.Int("count", processedCount))
	return processedCount, nil
}

// buildLogCommandArgs constructs the arguments for the log command
// archivePath should be empty string for live mode, or a specific archive path for archive mode
func (r *unifiedLoggingReceiver) buildLogCommandArgs(archivePath string) []string {
	args := []string{"show"}

	// Add archive path if specified
	if archivePath != "" {
		args = append(args, "--archive", archivePath)
	}

	// Add style flag if format is not default
	if r.config.Format != "" && r.config.Format != "default" {
		args = append(args, "--style", r.config.Format)
	}

	// Add start time
	if r.config.StartTime != "" {
		args = append(args, "--start", r.config.StartTime)
	} else if r.config.MaxLogAge > 0 && archivePath == "" {
		// For live mode, calculate start time from max_log_age
		startTime := time.Now().Add(-r.config.MaxLogAge)
		args = append(args, "--start", startTime.Format("2006-01-02 15:04:05"))
	}

	// Add end time (archive mode only)
	if r.config.EndTime != "" && archivePath != "" {
		args = append(args, "--end", r.config.EndTime)
	}

	// Add predicate filter
	if r.config.Predicate != "" {
		args = append(args, "--predicate", r.config.Predicate)
	}

	return args
}

// processLogLine processes a log line and sends it to the consumer
func (r *unifiedLoggingReceiver) processLogLine(ctx context.Context, line []byte) error {
	// Convert to OTel plog
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecord := scopeLogs.LogRecords().AppendEmpty()

	// Put the entire line into the log body as a string
	logRecord.Body().SetStr(string(line))
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Parse timestamp and severity when using JSON formats
	if r.config.Format == "ndjson" || r.config.Format == "json" {
		var logEntry map[string]any
		if err := json.Unmarshal(line, &logEntry); err == nil {
			// Parse and set timestamp
			if ts, ok := logEntry["timestamp"].(string); ok {
				if t, err := time.Parse("2006-01-02 15:04:05.000000-0700", ts); err == nil {
					logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))
				}
			}

			// Set severity from messageType
			if msgType, ok := logEntry["messageType"].(string); ok {
				logRecord.SetSeverityText(msgType)
				logRecord.SetSeverityNumber(mapMessageTypeToSeverity(msgType))
			}
		}
	}

	// Send to consumer
	return r.consumer.ConsumeLogs(ctx, logs)
}

// mapMessageTypeToSeverity maps log messageType to OTel severity
func mapMessageTypeToSeverity(msgType string) plog.SeverityNumber {
	switch msgType {
	case "Error":
		return plog.SeverityNumberError
	case "Fault":
		return plog.SeverityNumberFatal
	case "Default", "Info":
		return plog.SeverityNumberInfo
	case "Debug":
		return plog.SeverityNumberDebug
	default:
		return plog.SeverityNumberUnspecified
	}
}

// isCompletionLine checks if a line is a completion/status message from the log command
// These lines should be filtered out (e.g., {"count":540659,"finished":1})
func isCompletionLine(line []byte) bool {
	// Trim whitespace
	trimmed := bytes.TrimSpace(line)

	// Check if line is empty
	if len(trimmed) == 0 {
		return false
	}

	// Check if line starts with "**" (typical completion message format)
	if bytes.HasPrefix(trimmed, []byte("**")) {
		return true
	}

	// Check for JSON completion format: {"count":N,"finished":1}
	if bytes.HasPrefix(trimmed, []byte("{")) && bytes.HasSuffix(trimmed, []byte("}")) {
		// Quick check for both "count" and "finished" fields
		if bytes.Contains(trimmed, []byte("\"count\"")) &&
			bytes.Contains(trimmed, []byte("\"finished\"")) {
			return true
		}
	}

	// Check for common completion keywords
	if bytes.Contains(trimmed, []byte("Processed")) &&
		(bytes.Contains(trimmed, []byte("entries")) || bytes.Contains(trimmed, []byte("done"))) {
		return true
	}

	return false
}
