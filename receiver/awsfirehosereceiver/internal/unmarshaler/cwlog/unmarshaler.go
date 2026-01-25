// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
)

const (
	TypeStr = "cwlogs"

	attributeAWSCloudWatchLogGroupName  = "aws.cloudwatch.log_group_name"
	attributeAWSCloudWatchLogStreamName = "aws.cloudwatch.log_stream_name"
)

var (
	errInvalidRecords   = errors.New("record format invalid")
	errMissingOwner     = errors.New("cloudwatch log record is missing owner field")
	errMissingLogGroup  = errors.New("cloudwatch log record is missing logGroup field")
	errMissingLogStream = errors.New("cloudwatch log record is missing logStream field")
)

// resourceGroupKey represents the combination of account ID and region used for grouping log events.
type resourceGroupKey struct {
	accountID string
	region    string
}

// logRecordData holds the parsed data from a CloudWatch log event.
type logRecordData struct {
	id        string
	timestamp int64
	message   string
}

// groupedLogEvent pairs a resourceGroupKey with its corresponding logRecordData for processing.
type groupedLogEvent struct {
	key  resourceGroupKey
	data logRecordData
}

// Unmarshaler for the CloudWatch Log JSON record format.
type Unmarshaler struct {
	logger    *zap.Logger
	buildInfo component.BuildInfo
	gzipPool  sync.Pool
}

var _ plog.Unmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger, buildInfo component.BuildInfo) *Unmarshaler {
	return &Unmarshaler{logger: logger, buildInfo: buildInfo}
}

// UnmarshalLogs deserializes the given record as CloudWatch Logs events
// into a plog.Logs, grouping logs by owner (account ID), log group, and
// log stream. Logs are assumed to be gzip-compressed as specified at
// https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html.
func (u *Unmarshaler) UnmarshalLogs(compressedRecord []byte) (plog.Logs, error) {
	var err error
	r, ok := u.gzipPool.Get().(*gzip.Reader)
	if !ok {
		r, err = gzip.NewReader(bytes.NewReader(compressedRecord))
	} else {
		err = r.Reset(bytes.NewReader(compressedRecord))
	}
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to decompress record: %w", err)
	}
	defer u.gzipPool.Put(r)

	data, err := io.ReadAll(r)
	if err != nil {
		u.logger.Error("Error reading log data", zap.Error(err))
		return plog.Logs{}, fmt.Errorf("error reading log data: %w", err)
	}

	cwLog, control, err := parseLog(data)
	if err != nil {
		u.logger.Error("Error unmarshalling log message", zap.Error(err))
		return plog.Logs{}, fmt.Errorf("%w: %w", errInvalidRecords, err)
	}

	if control {
		for _, event := range cwLog.LogEvents {
			u.logger.Debug(
				"Skipping CloudWatch control message event",
				zap.Time("timestamp", time.UnixMilli(event.Timestamp)),
				zap.String("message", event.Message),
			)
		}
		return plog.NewLogs(), nil
	}

	logs := plog.NewLogs()

	// Group events by their extracted fields (account ID + region).
	// Events with different extracted fields should be in separate ResourceLogs.
	// Events without extracted fields use the Owner value.

	// Pre-allocate slice with exact length of all events and count unique keys in one pass.
	groupedLogEvents := make([]groupedLogEvent, len(cwLog.LogEvents))
	keyCounts := make(map[resourceGroupKey]int)
	for i, event := range cwLog.LogEvents {
		var key resourceGroupKey
		if event.ExtractedFields != nil {
			if event.ExtractedFields.AccountID != "" {
				key.accountID = event.ExtractedFields.AccountID
			} else {
				key.accountID = cwLog.Owner
			}
			key.region = event.ExtractedFields.Region
		} else {
			key.accountID = cwLog.Owner
		}

		groupedLogEvents[i] = groupedLogEvent{
			key: key,
			data: logRecordData{
				id:        event.ID,
				timestamp: event.Timestamp,
				message:   event.Message,
			},
		}
		// Count occurrences while creating grouped events.
		keyCounts[key]++
	}

	// Create ResourceLogs and populate log records in a single pass.
	resourceLogsMap := make(map[resourceGroupKey]plog.ResourceLogs)

	for _, groupedEvent := range groupedLogEvents {
		// Create ResourceLogs structure on first encounter of each key.
		rl, exists := resourceLogsMap[groupedEvent.key]
		if !exists {
			rl = logs.ResourceLogs().AppendEmpty()
			resourceAttrs := rl.Resource().Attributes()
			resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
			resourceAttrs.PutStr(string(conventions.CloudAccountIDKey), groupedEvent.key.accountID)
			if groupedEvent.key.region != "" {
				resourceAttrs.PutStr(string(conventions.CloudRegionKey), groupedEvent.key.region)
			}
			resourceAttrs.PutEmptySlice(string(conventions.AWSLogGroupNamesKey)).AppendEmpty().SetStr(cwLog.LogGroup)
			resourceAttrs.PutEmptySlice(string(conventions.AWSLogStreamNamesKey)).AppendEmpty().SetStr(cwLog.LogStream)
			// Deprecated: [v0.121.0] Use `string(conventions.AWSLogGroupNamesKey)` instead
			resourceAttrs.PutStr(attributeAWSCloudWatchLogGroupName, cwLog.LogGroup)
			// Deprecated: [v0.121.0] Use `string(conventions.AWSLogStreamNamesKey)` instead
			resourceAttrs.PutStr(attributeAWSCloudWatchLogStreamName, cwLog.LogStream)

			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName(metadata.ScopeName)
			sl.Scope().SetVersion(u.buildInfo.Version)

			// Pre-allocate LogRecords capacity to avoid reallocations.
			logRecords := sl.LogRecords()
			logRecords.EnsureCapacity(keyCounts[groupedEvent.key])

			resourceLogsMap[groupedEvent.key] = rl
		}

		// Create and populate log record immediately.
		// We always have exactly one ScopeLogs per ResourceLogs, so access it directly.
		sl := resourceLogsMap[groupedEvent.key].ScopeLogs().At(0)
		logRecord := sl.LogRecords().AppendEmpty()

		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logRecord.SetTimestamp(pcommon.Timestamp(groupedEvent.data.timestamp * int64(time.Millisecond)))
		logRecord.Body().SetStr(groupedEvent.data.message)
	}

	return logs, nil
}

func parseLog(data []byte) (log cWLog, control bool, _ error) {
	if err := jsoniter.ConfigFastest.Unmarshal(data, &log); err != nil {
		return cWLog{}, false, err
	}
	switch log.MessageType {
	case "DATA_MESSAGE":
		if log.Owner == "" {
			return cWLog{}, false, errMissingOwner
		}
		if log.LogGroup == "" {
			return cWLog{}, false, errMissingLogGroup
		}
		if log.LogStream == "" {
			return cWLog{}, false, errMissingLogStream
		}
		return log, false, nil
	case "CONTROL_MESSAGE":
		return log, true, nil
	default:
		return cWLog{}, false, fmt.Errorf("invalid message type %q", log.MessageType)
	}
}

// Type of the serialized messages.
func (*Unmarshaler) Type() string {
	return TypeStr
}
