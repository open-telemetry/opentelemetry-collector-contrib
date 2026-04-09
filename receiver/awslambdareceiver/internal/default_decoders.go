// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"

import (
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal/metadata"
)

// NewDefaultS3LogsDecoder returns a defaultS3Unmarshaler wrapped as an encoding.LogsDecoderFactory.
func NewDefaultS3LogsDecoder() encoding.LogsDecoderFactory {
	return xstreamencoding.NewLogsUnmarshalerDecoderFactory(&defaultS3Unmarshaler{})
}

// defaultS3Unmarshaler defines the default S3 logs decoder for AWS Lambda receiver.
type defaultS3Unmarshaler struct{}

// UnmarshalLogs defines the built-in behavior for S3 events when no encoding extension is provided.
func (*defaultS3Unmarshaler) UnmarshalLogs(data []byte) (plog.Logs, error) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)

	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(string(data))

	return logs, nil
}

// NewDefaultCWLogsDecoder returns a defaultCWLogsDecoder wrapped as an encoding.LogsDecoderFactory.
func NewDefaultCWLogsDecoder() encoding.LogsDecoderFactory {
	return xstreamencoding.NewLogsUnmarshalerDecoderFactory(&defaultCWLogsDecoder{})
}

// defaultCWLogsDecoder defines the default CloudWatch logs decoder for AWS Lambda receiver.
type defaultCWLogsDecoder struct{}

// UnmarshalLogs defines the built-in behavior for CloudWatch logs events when no encoding extension is provided.
func (*defaultCWLogsDecoder) UnmarshalLogs(data []byte) (plog.Logs, error) {
	var cwLog events.CloudwatchLogsData
	err := gojson.Unmarshal(data, &cwLog)
	if err != nil {
		return plog.NewLogs(), fmt.Errorf("failed to unmarshal data from cloudwatch logs event: %w", err)
	}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	resourceAttrs.PutStr(string(conventions.CloudAccountIDKey), cwLog.Owner)
	resourceAttrs.PutStr(string(conventions.AWSLogGroupNamesKey), cwLog.LogGroup)
	resourceAttrs.PutStr(string(conventions.AWSLogStreamNamesKey), cwLog.LogStream)

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)

	for _, event := range cwLog.LogEvents {
		logRecord := sl.LogRecords().AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logRecord.Body().SetStr(event.Message)
	}

	return logs, nil
}
