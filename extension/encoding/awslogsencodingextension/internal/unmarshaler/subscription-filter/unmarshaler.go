// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

import (
	"errors"
	"fmt"
	"io"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
)

const ctrlMessageType = "CONTROL_MESSAGE"

var (
	errEmptyOwner     = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty owner field")
	errEmptyLogGroup  = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log group field")
	errEmptyLogStream = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log stream field")
)

var _ unmarshaler.StreamingLogsUnmarshaler = (*SubscriptionFilterUnmarshaler)(nil)

type SubscriptionFilterUnmarshaler struct {
	buildInfo component.BuildInfo
}

func NewSubscriptionFilterUnmarshaler(buildInfo component.BuildInfo) *SubscriptionFilterUnmarshaler {
	return &SubscriptionFilterUnmarshaler{
		buildInfo: buildInfo,
	}
}

// UnmarshalAWSLogs deserializes the given reader as CloudWatch Logs events
// into a plog.Logs, grouping logs by owner (account ID), log group, and
// log stream. When extracted fields are present (from centralized logging),
// logs are further grouped by their extracted account ID and region.
// Logs are assumed to be gzip-compressed as specified at
// https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html.
func (f *SubscriptionFilterUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	// Decode as a stream but flush all at once using flush options
	streamUnmarshaler, err := f.NewLogsDecoder(reader, encoding.WithFlushItems(0), encoding.WithFlushBytes(0))
	if err != nil {
		return plog.Logs{}, err
	}
	logs, err := streamUnmarshaler.DecodeLogs()
	if err != nil {
		// we must check for EOF with direct comparison and avoid wrapped EOF that can come from stream itself
		//nolint:errorlint
		if err == io.EOF {
			// EOF indicates no logs were found, return any logs that's available
			return logs, nil
		}

		return plog.Logs{}, err
	}

	return logs, nil
}

// NewLogsDecoder returns a LogsDecoder that processes CloudWatch Logs subscription filter events.
// Supported sub formats:
//   - DATA_MESSAGE: Returns logs grouped by owner, log group, and stream; offset is the number of records processed
//   - CONTROL_MESSAGE: Returns empty log; offset is the number of records processed
func (f *SubscriptionFilterUnmarshaler) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	batchHelper := xstreamencoding.NewBatchHelper(options...)
	decoder := gojson.NewDecoder(reader)

	var offset int64

	if batchHelper.Options().Offset > 0 {
		for offset < batchHelper.Options().Offset {
			if !decoder.More() {
				return nil, fmt.Errorf("EOF reached before offset %d records were discarded", batchHelper.Options().Offset)
			}

			var raw gojson.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return nil, err
			}
			offset++
		}
	}

	return xstreamencoding.NewLogsDecoderAdapter(
		func() (plog.Logs, error) {
			logs := plog.NewLogs()
			resourceLogsByKey := make(map[resourceGroupKey]plog.LogRecordSlice)

			for decoder.More() {
				var cwLog cloudwatchLogsData
				if err := decoder.Decode(&cwLog); err != nil {
					return plog.Logs{}, fmt.Errorf("failed to decode decompressed reader: %w", err)
				}

				offset++
				batchHelper.IncrementItems(1)

				if cwLog.MessageType == ctrlMessageType {
					continue
				}

				if err := validateLog(cwLog); err != nil {
					return plog.Logs{}, fmt.Errorf("invalid cloudwatch log: %w", err)
				}

				f.appendLogs(logs, resourceLogsByKey, cwLog)

				if batchHelper.ShouldFlush() {
					batchHelper.Reset()
					return logs, nil
				}
			}

			if logs.ResourceLogs().Len() == 0 {
				return plog.NewLogs(), io.EOF
			}

			return logs, nil
		}, func() int64 {
			return offset
		},
	), nil
}

// appendLogs appends log records from cwLog into the given plog.Logs, reusing
// existing ResourceLogs entries tracked by resourceLogsByKey when possible.
// Events are grouped by their extracted fields (account ID + region) and
// by log group/stream combination.
func (f *SubscriptionFilterUnmarshaler) appendLogs(logs plog.Logs, resourceLogsByKey map[resourceGroupKey]plog.LogRecordSlice, cwLog cloudwatchLogsData) {
	for _, event := range cwLog.LogEvents {
		key := extractResourceKey(event, cwLog.Owner, cwLog.LogGroup, cwLog.LogStream)

		logRecords, exists := resourceLogsByKey[key]
		if !exists {
			rl := logs.ResourceLogs().AppendEmpty()
			resourceAttrs := rl.Resource().Attributes()
			resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
			resourceAttrs.PutStr(string(conventions.CloudAccountIDKey), key.accountID)
			if key.region != "" {
				resourceAttrs.PutStr(string(conventions.CloudRegionKey), key.region)
			}
			resourceAttrs.PutEmptySlice(string(conventions.AWSLogGroupNamesKey)).AppendEmpty().SetStr(cwLog.LogGroup)
			resourceAttrs.PutEmptySlice(string(conventions.AWSLogStreamNamesKey)).AppendEmpty().SetStr(cwLog.LogStream)

			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName(metadata.ScopeName)
			sl.Scope().SetVersion(f.buildInfo.Version)
			sl.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatCloudWatchLogsSubscriptionFilter)

			logRecords = sl.LogRecords()
			resourceLogsByKey[key] = logRecords
		}

		logRecord := logRecords.AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logRecord.Body().SetStr(event.Message)
	}
}

// extractResourceKey extracts the resource group key from a log event.
// When extracted fields are present, uses those; otherwise falls back to owner.
func extractResourceKey(event cloudwatchLogsLogEvent, owner, logGroup, logStream string) resourceGroupKey {
	var key resourceGroupKey
	key.logGroup = logGroup
	key.logStream = logStream
	if event.ExtractedFields != nil {
		if event.ExtractedFields.AccountID != "" {
			key.accountID = event.ExtractedFields.AccountID
		} else {
			key.accountID = owner
		}
		key.region = event.ExtractedFields.Region
	} else {
		key.accountID = owner
	}
	return key
}

func validateLog(log cloudwatchLogsData) error {
	switch log.MessageType {
	case "DATA_MESSAGE":
		if log.Owner == "" {
			return errEmptyOwner
		}
		if log.LogGroup == "" {
			return errEmptyLogGroup
		}
		if log.LogStream == "" {
			return errEmptyLogStream
		}
	case ctrlMessageType:
	default:
		return fmt.Errorf("cloudwatch log has invalid message type %q", log.MessageType)
	}
	return nil
}
