// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

var _ encoding.StreamDecoder[plog.Logs] = (*subscriptionFilterUnmarshaler)(nil)

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
	reader    io.Reader
	decoder   *gojson.Decoder
	offset    encoding.StreamOffset
}

type subscriptionFilterUnmarshalerFactory struct {
	buildInfo component.BuildInfo
}

func NewSubscriptionFilterUnmarshalerFactory(buildInfo component.BuildInfo) func(reader io.Reader, opts encoding.StreamDecoderOptions) (encoding.StreamDecoder[plog.Logs], error) {
	return func(reader io.Reader, opts encoding.StreamDecoderOptions) (encoding.StreamDecoder[plog.Logs], error) {
		decoder := gojson.NewDecoder(reader)
		// Skip to initial offset
		for i := encoding.StreamOffset(0); i < opts.InitialOffset; i++ {
			var dummy interface{}
			if err := decoder.Decode(&dummy); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				break
			}
		}
		return &subscriptionFilterUnmarshaler{
			buildInfo: buildInfo,
			reader:    reader,
			decoder:   decoder,
			offset:    opts.InitialOffset,
		}, nil
	}
}

// Decode reads the next CloudWatch Logs event from the stream.
// Logs are assumed to be gzip-compressed as specified at
// https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html.
func (f *subscriptionFilterUnmarshaler) Decode(ctx context.Context, to plog.Logs) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var cwLog events.CloudwatchLogsData
	if err := f.decoder.Decode(&cwLog); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		return fmt.Errorf("failed to decode decompressed reader: %w", err)
	}

	if cwLog.MessageType == "CONTROL_MESSAGE" {
		// Return empty logs for control messages
		plog.NewLogs().CopyTo(to)
		return nil
	}

	if err := validateLog(cwLog); err != nil {
		return fmt.Errorf("invalid cloudwatch log: %w", err)
	}

	logs := f.createLogs(cwLog)
	logs.CopyTo(to)

	// Update offset (count log records)
	recordCount := int64(to.LogRecordCount())
	f.offset += encoding.StreamOffset(recordCount)

	return nil
}

func (f *subscriptionFilterUnmarshaler) Offset() encoding.StreamOffset {
	return f.offset
}

// createLogs create plog.Logs from the cloudwatchLog
func (f *subscriptionFilterUnmarshaler) createLogs(
	cwLog events.CloudwatchLogsData,
) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	resourceAttrs.PutStr(string(conventions.CloudAccountIDKey), cwLog.Owner)
	resourceAttrs.PutEmptySlice(string(conventions.AWSLogGroupNamesKey)).AppendEmpty().SetStr(cwLog.LogGroup)
	resourceAttrs.PutEmptySlice(string(conventions.AWSLogStreamNamesKey)).AppendEmpty().SetStr(cwLog.LogStream)
	resourceAttrs.PutStr(string(conventions.AWSLogGroupNamesKey), cwLog.LogGroup)
	resourceAttrs.PutStr(string(conventions.AWSLogStreamNamesKey), cwLog.LogStream)

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)
	sl.Scope().SetVersion(f.buildInfo.Version)
	sl.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatCloudWatchLogsSubscriptionFilter)
	for _, event := range cwLog.LogEvents {
		logRecord := sl.LogRecords().AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logRecord.Body().SetStr(event.Message)
	}
	return logs
}
