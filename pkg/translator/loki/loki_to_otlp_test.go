// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestPushRequestToLogs(t *testing.T) {
	testCases := []struct {
		name          string
		pushRequest   *push.PushRequest
		keepTimestamp bool
		expected      plog.Logs
	}{
		{
			name: "Should return empty log list if number of streams is 0",
			pushRequest: &push.PushRequest{
				Streams: []push.Stream{},
			},
			expected: plog.NewLogs(),
		},
		{
			name: "Should create log records correctly",
			pushRequest: &push.PushRequest{
				Streams: []push.Stream{
					{
						Labels: "{foo=\"bar\", label1=\"value1\"}",
						Entries: []push.Entry{
							{Timestamp: time.Unix(0, 1676888496000000000), Line: "logline 1"},
						},
					},
				},
			},
			keepTimestamp: true,
			expected: generateLogs([]Log{
				{
					Timestamp: 1676888496000000000,
					Body:      pcommon.NewValueStr("logline 1"),
					Attributes: map[string]interface{}{
						"foo":    "bar",
						"label1": "value1",
					},
				},
			}),
		},
		{
			name: "Should ignore label with name starting from __",
			pushRequest: &push.PushRequest{
				Streams: []push.Stream{
					{
						Labels: "{__foo=\"bar\", label1=\"value1\"}",
						Entries: []push.Entry{
							{Timestamp: time.Unix(0, 1676888496000000000), Line: "logline 1"},
						},
					},
				},
			},
			keepTimestamp: true,
			expected: generateLogs([]Log{
				{
					Timestamp: 1676888496000000000,
					Body:      pcommon.NewValueStr("logline 1"),
					Attributes: map[string]interface{}{
						"label1": "value1",
					},
				},
			}),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			logs, err := PushRequestToLogs(tt.pushRequest, tt.keepTimestamp)
			assert.NoError(t, err)
			assert.Equal(t, len(tt.pushRequest.Streams), logs.LogRecordCount())
			require.NoError(t, plogtest.CompareLogs(tt.expected, logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

type Log struct {
	Timestamp  int64
	Body       pcommon.Value
	Attributes map[string]interface{}
}

func generateLogs(logs []Log) plog.Logs {
	ld := plog.NewLogs()
	logSlice := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()

	for _, log := range logs {
		lr := logSlice.AppendEmpty()
		_ = lr.Attributes().FromRaw(log.Attributes)
		lr.SetTimestamp(pcommon.Timestamp(log.Timestamp))
		lr.Body().SetStr(log.Body.AsString())
	}
	return ld
}
