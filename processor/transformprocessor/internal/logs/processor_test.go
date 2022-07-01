// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)
)

func TestProcess(t *testing.T) {
	tests := []struct {
		query string
		want  func(td plog.Logs)
	}{
		{
			query: `set(attributes["test"], "pass") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("test", "pass")
			},
		},
		{
			query: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().InsertString("test", "pass")
			},
		},
		{
			query: `keep_keys(attributes, "http.method") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("http.method", "get")
			},
		},
		{
			query: `set(severity_text, "ok") where attributes["http.path"] == "/health"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetSeverityText("ok")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).SetSeverityText("ok")
			},
		},
		{
			query: `replace_pattern(attributes["http.method"], "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().UpdateString("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().UpdateString("http.method", "post")
			},
		},
		{
			query: `replace_all_patterns(attributes, "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().UpdateString("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().UpdateString("http.method", "post")
			},
		},
		{
			query: `set(attributes["test"], "pass") where dropped_attributes_count == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("test", "pass")
			},
		},
		{
			query: `set(attributes["test"], "pass") where flags == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("test", "pass")
			},
		},
		{
			query: `set(attributes["test"], "pass") where trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("test", "pass")
			},
		},
		{
			query: `set(attributes["test"], "pass") where span_id == SpanID(0x0102030405060708)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("test", "pass")
			},
		},
		{
			query: `set(attributes["test"], "pass") where IsMatch(body, "operation[AC]") == true`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().InsertString("test", "pass")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]string{tt.query}, DefaultFunctions(), component.ProcessorCreateSettings{})
			assert.NoError(t, err)

			_, err = processor.ProcessLogs(context.Background(), td)
			assert.NoError(t, err)

			exTd := constructLogs()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func constructLogs() plog.Logs {
	td := plog.NewLogs()
	rs0 := td.ResourceLogs().AppendEmpty()
	rs0.Resource().Attributes().InsertString("host.name", "localhost")
	rs0ils0 := rs0.ScopeLogs().AppendEmpty()
	fillLogOne(rs0ils0.LogRecords().AppendEmpty())
	fillLogTwo(rs0ils0.LogRecords().AppendEmpty())
	return td
}

func fillLogOne(log plog.LogRecord) {
	log.Body().SetStringVal("operationA")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetFlags(1)
	log.SetTraceID(pcommon.NewTraceID(traceID))
	log.SetSpanID(pcommon.NewSpanID(spanID))
	log.Attributes().InsertString("http.method", "get")
	log.Attributes().InsertString("http.path", "/health")
	log.Attributes().InsertString("http.url", "http://localhost/health")
}

func fillLogTwo(log plog.LogRecord) {
	log.Body().SetStringVal("operationB")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.Attributes().InsertString("http.method", "get")
	log.Attributes().InsertString("http.path", "/health")
	log.Attributes().InsertString("http.url", "http://localhost/health")
}
