// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package firehoselog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/firehoselog"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"time"
)

const (
	attributeAWSFirehoseARN = "aws.firehose.arn"
)

// resourceAttributes are the firehose log attributes that define a unique resource.
type resourceAttributes struct {
	firehoseARN string
}

// resourceLogsBuilder provides convenient access to the resource's LogRecordSlice.
type resourceLogsBuilder struct {
	rls plog.LogRecordSlice
}

// setAttributes applies the resourceAttributes to the provided Resource.
func (ra *resourceAttributes) setAttributes(resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.PutStr(attributeAWSFirehoseARN, ra.firehoseARN)
}

// newResourceLogsBuilder to capture logs for the Resource defined by the provided attributes.
func newResourceLogsBuilder(logs plog.Logs, attrs resourceAttributes) *resourceLogsBuilder {
	rls := logs.ResourceLogs().AppendEmpty()
	attrs.setAttributes(rls.Resource())
	return &resourceLogsBuilder{rls.ScopeLogs().AppendEmpty().LogRecords()}
}

// AddLog events to the LogRecordSlice. Resource attributes are captured when creating
// the resourceLogsBuilder, so we only need to consider the LogEvents themselves.
func (rlb *resourceLogsBuilder) AddLog(log firehoseLog) {
	logLine := rlb.rls.AppendEmpty()
	// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
	// but timestamp in cloudwatch logs are in milliseconds.
	logLine.SetTimestamp(pcommon.Timestamp(log.Timestamp * int64(time.Millisecond)))
	logLine.Body().SetStr(log.Message)
}
