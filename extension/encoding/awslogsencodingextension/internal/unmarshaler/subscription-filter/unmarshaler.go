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
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
)

var (
	errEmptyOwner     = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty owner field")
	errEmptyLogGroup  = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log group field")
	errEmptyLogStream = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log stream field")
)

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
	case "CONTROL_MESSAGE":
	default:
		return fmt.Errorf("cloudwatch log has invalid message type %q", log.MessageType)
	}
	return nil
}

type subscriptionFilterUnmarshaler struct {
	buildInfo component.BuildInfo
}

func NewSubscriptionFilterUnmarshaler(buildInfo component.BuildInfo) unmarshaler.AWSUnmarshaler {
	return &subscriptionFilterUnmarshaler{
		buildInfo: buildInfo,
	}
}

// UnmarshalAWSLogs deserializes the given reader as CloudWatch Logs events
// into a plog.Logs, grouping logs by owner (account ID), log group, and
// log stream. When extracted fields are present (from centralized logging),
// logs are further grouped by their extracted account ID and region.
// Logs are assumed to be gzip-compressed as specified at
// https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html.
func (f *subscriptionFilterUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	logs := plog.NewLogs()
	resourceLogsByKey := make(map[resourceGroupKey]plog.LogRecordSlice)

	decoder := gojson.NewDecoder(reader)
	for decoder.More() {
		var cwLog cloudwatchLogsData
		if err := decoder.Decode(&cwLog); err != nil {
			return plog.Logs{}, fmt.Errorf("failed to decode decompressed reader: %w", err)
		}

		if cwLog.MessageType == "CONTROL_MESSAGE" {
			continue
		}

		if err := validateLog(cwLog); err != nil {
			return plog.Logs{}, fmt.Errorf("invalid cloudwatch log: %w", err)
		}

		f.appendLogs(logs, resourceLogsByKey, cwLog)
	}

	return logs, nil
}

// appendLogs appends log records from cwLog into the given plog.Logs, reusing
// existing ResourceLogs entries tracked by resourceLogsByKey when possible.
// Events are grouped by their extracted fields (account ID + region) and
// by log group/stream combination.
func (f *subscriptionFilterUnmarshaler) appendLogs(logs plog.Logs, resourceLogsByKey map[resourceGroupKey]plog.LogRecordSlice, cwLog cloudwatchLogsData) {
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
