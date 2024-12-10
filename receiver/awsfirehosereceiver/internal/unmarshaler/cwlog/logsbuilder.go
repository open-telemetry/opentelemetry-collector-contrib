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

// resourceAttributes are the CloudWatch log attributes that define a unique resource.
type resourceAttributes struct {
	owner, logGroup, logStream string
}

// resourceLogsBuilder provides convenient access to the a Resource's LogRecordSlice.
type resourceLogsBuilder struct {
	rls plog.LogRecordSlice
}

// setAttributes applies the resourceAttributes to the provided Resource.
func (ra *resourceAttributes) setAttributes(resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeCloudAccountID, ra.owner)
	attrs.PutStr(attributeAWSCloudWatchLogGroupName, ra.logGroup)
	attrs.PutStr(attributeAWSCloudWatchLogStreamName, ra.logStream)
}

// newResourceLogsBuilder to capture logs for the Resource defined by the provided attributes.
func newResourceLogsBuilder(logs plog.Logs, attrs resourceAttributes) *resourceLogsBuilder {
	rls := logs.ResourceLogs().AppendEmpty()
	attrs.setAttributes(rls.Resource())
	return &resourceLogsBuilder{rls.ScopeLogs().AppendEmpty().LogRecords()}
}

// AddLog events to the LogRecordSlice. Resource attributes are captured when creating
// the resourceLogsBuilder, so we only need to consider the LogEvents themselves.
func (rlb *resourceLogsBuilder) AddLog(log cWLog) {
	for _, event := range log.LogEvents {
		logLine := rlb.rls.AppendEmpty()
		// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
		// but timestamp in cloudwatch logs are in milliseconds.
		logLine.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
		logLine.Body().SetStr(event.Message)
	}
}
