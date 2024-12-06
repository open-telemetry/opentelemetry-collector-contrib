// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

const (
	attributeAWSCloudWatchLogGroupName  = "aws.cloudwatch.log_group_name"
	attributeAWSCloudWatchLogStreamName = "aws.cloudwatch.log_stream_name"
)

// ResourceAttributes are the CloudWatch log attributes that define a unique resource.
type ResourceAttributes struct {
	Owner     string
	LogGroup  string
	LogStream string
}

// ResourceLogsBuilder provides convenient access to a Resource's LogRecordSlice.
type ResourceLogsBuilder struct {
	rls plog.LogRecordSlice
}

// setAttributes applies the ResourceAttributes to the provided Resource.
func (ra *ResourceAttributes) setAttributes(resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudAccountID, ra.Owner)
	attrs.PutStr(attributeAWSCloudWatchLogGroupName, ra.LogGroup)
	attrs.PutStr(attributeAWSCloudWatchLogStreamName, ra.LogStream)
}

// NewResourceLogsBuilder to capture logs for the Resource defined by the provided attributes.
func NewResourceLogsBuilder(logs plog.Logs, attrs ResourceAttributes) *ResourceLogsBuilder {
	rls := logs.ResourceLogs().AppendEmpty()
	attrs.setAttributes(rls.Resource())
	return &ResourceLogsBuilder{rls.ScopeLogs().AppendEmpty().LogRecords()}
}

// AddLog events to the LogRecordSlice. Resource attributes are captured when creating
// the ResourceLogsBuilder, so we only need to consider the LogEvents themselves.
func (rlb *ResourceLogsBuilder) AddLog(log CWLog) {
	for _, event := range log.LogEvents {
		logLine := rlb.rls.AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logLine.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logLine.Body().SetStr(event.Message)
	}
}
