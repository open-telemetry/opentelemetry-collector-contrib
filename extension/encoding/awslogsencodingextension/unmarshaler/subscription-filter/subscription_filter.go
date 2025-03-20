// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/unmarshaler/subscription-filter"

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

var (
	errEmptyOwner     = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty owner field")
	errEmptyLogGroup  = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log group field")
	errEmptyLogStream = errors.New("cloudwatch log with message type 'DATA_MESSAGE' has empty log stream field")
)

func validateLog(log events.CloudwatchLogsData) error {
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

	// We need to decompress the records.
	// Creating a new gzip.Reader every time is expensive.
	// We can keep the gzip in cache, and take advantage
	// of the continuously AWS environment or the AWS
	// warm starts, in case of lambda.
	// See https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html.
	gzipPool sync.Pool
}

func NewSubscriptionFilterUnmarshaler(buildInfo component.BuildInfo) plog.Unmarshaler {
	return &subscriptionFilterUnmarshaler{
		buildInfo: buildInfo,
		gzipPool:  sync.Pool{},
	}
}

var _ plog.Unmarshaler = (*subscriptionFilterUnmarshaler)(nil)

// UnmarshalLogs deserializes the given record as CloudWatch Logs events
// into a plog.Logs, grouping logs by owner (account ID), log group, and
// log stream. Logs are assumed to be gzip-compressed as specified at
// https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html.
func (f *subscriptionFilterUnmarshaler) UnmarshalLogs(compressedRecord []byte) (plog.Logs, error) {
	var errDecompress error
	record, ok := f.gzipPool.Get().(*gzip.Reader)
	if !ok {
		record, errDecompress = gzip.NewReader(bytes.NewReader(compressedRecord))
	} else {
		errDecompress = record.Reset(bytes.NewReader(compressedRecord))
	}
	if errDecompress != nil {
		if record != nil {
			f.gzipPool.Put(record)
		}
		return plog.Logs{}, fmt.Errorf("failed to decompress record: %w", errDecompress)
	}
	defer func() {
		_ = record.Close()
		f.gzipPool.Put(record)
	}()

	decompressedRecord, errRead := io.ReadAll(record)
	if errRead != nil {
		return plog.Logs{}, fmt.Errorf("failed to read decompressed record: %w", errRead)
	}

	var cwLog events.CloudwatchLogsData
	if err := jsoniter.ConfigFastest.Unmarshal(decompressedRecord, &cwLog); err != nil {
		return plog.Logs{}, fmt.Errorf("error unmarshaling data: %w", err)
	}

	if cwLog.MessageType == "CONTROL_MESSAGE" {
		return plog.Logs{}, nil
	}

	if err := validateLog(cwLog); err != nil {
		return plog.Logs{}, fmt.Errorf("invalid cloudwatch log: %w", err)
	}

	return f.createLogs(cwLog), nil
}

// createLogs create plog.Logs from the cloudwatchLog
func (f *subscriptionFilterUnmarshaler) createLogs(
	cwLog events.CloudwatchLogsData,
) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	resourceAttrs.PutStr(conventions.AttributeCloudAccountID, cwLog.Owner)
	resourceAttrs.PutEmptySlice(conventions.AttributeAWSLogGroupNames).AppendEmpty().SetStr(cwLog.LogGroup)
	resourceAttrs.PutEmptySlice(conventions.AttributeAWSLogStreamNames).AppendEmpty().SetStr(cwLog.LogStream)
	resourceAttrs.PutStr(conventions.AttributeAWSLogGroupNames, cwLog.LogGroup)
	resourceAttrs.PutStr(conventions.AttributeAWSLogStreamNames, cwLog.LogStream)

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)
	sl.Scope().SetVersion(f.buildInfo.Version)
	for _, event := range cwLog.LogEvents {
		logRecord := sl.LogRecords().AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logRecord.Body().SetStr(event.Message)
	}
	return logs
}
