// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
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

	// Pool the gzip readers, which are expensive to create.
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
	gzipReader, ok := f.gzipPool.Get().(*gzip.Reader)
	if !ok {
		gzipReader, errDecompress = gzip.NewReader(bytes.NewReader(compressedRecord))
	} else {
		errDecompress = gzipReader.Reset(bytes.NewReader(compressedRecord))
	}
	if errDecompress != nil {
		if gzipReader != nil {
			f.gzipPool.Put(gzipReader)
		}
		return plog.Logs{}, fmt.Errorf("failed to decompress record: %w", errDecompress)
	}
	defer func() {
		_ = gzipReader.Close()
		f.gzipPool.Put(gzipReader)
	}()

	var cwLog events.CloudwatchLogsData
	decoder := gojson.NewDecoder(gzipReader)
	if err := decoder.Decode(&cwLog); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to decode decompressed record: %w", err)
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
