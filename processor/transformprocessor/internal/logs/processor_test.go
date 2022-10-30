// Copyright The OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	TestObservedTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestObservedTimestamp = pcommon.NewTimestampFromTime(TestObservedTime)

	traceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func TestProcess(t *testing.T) {
	tests := []struct {
		statement string
		want      func(td plog.Logs)
	}{
		{
			statement: `set(attributes["test"], "pass") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `keep_keys(attributes, ["http.method"]) where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method",
					"get")
			},
		},
		{
			statement: `set(severity_text, "ok") where attributes["http.path"] == "/health"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetSeverityText("ok")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).SetSeverityText("ok")
			},
		},
		{
			statement: `replace_pattern(attributes["http.method"], "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "value", "get", "post")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "post")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "post")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "key", "http.url", "url")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method", "get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.path", "/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags", "A|B|C")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.method", "get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("http.path", "/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("url", "http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutStr("flags", "C|D")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where dropped_attributes_count == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where flags == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where severity_number == SEVERITY_NUMBER_TRACE`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(severity_number, SEVERITY_NUMBER_TRACE2) where severity_number == 1`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetSeverityNumber(2)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where trace_id == TraceID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where span_id == SpanID(0x0102030405060708)`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMatch(body, "operation[AC]") == true`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "pass")
			},
		},
		{
			statement: `delete_key(attributes, "http.url") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.method",
					"get")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.path",
					"/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags",
					"A|B|C")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "http.*t.*") where body == "operationA"`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Clear()
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("http.url",
					"http://localhost/health")
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("flags",
					"A|B|C")
			},
		},
		{
			statement: `set(attributes["test"], Concat([attributes["http.method"], attributes["http.url"]], ": ")) where body == Concat(["operation", "A"], "")`,
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("test", "get: http://localhost/health")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|"))`,
			want: func(td plog.Logs) {
				v1 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				v1.AppendEmpty().SetStr("A")
				v1.AppendEmpty().SetStr("B")
				v1.AppendEmpty().SetStr("C")
				v2 := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutEmptySlice("test")
				v2.AppendEmpty().SetStr("C")
				v2.AppendEmpty().SetStr("D")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|")) where body == "operationA"`,
			want: func(td plog.Logs) {
				newValue := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutEmptySlice("test")
				newValue.AppendEmpty().SetStr("A")
				newValue.AppendEmpty().SetStr("B")
				newValue.AppendEmpty().SetStr("C")
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["not_exist"], "|"))`,
			want:      func(td plog.Logs) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			td := constructLogs()
			processor, err := NewProcessor([]string{tt.statement}, Functions(), componenttest.NewNopTelemetrySettings())
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
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeLogs().AppendEmpty()
	fillLogOne(rs0ils0.LogRecords().AppendEmpty())
	fillLogTwo(rs0ils0.LogRecords().AppendEmpty())
	return td
}

func fillLogOne(log plog.LogRecord) {
	log.Body().SetStr("operationA")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.SetDroppedAttributesCount(1)
	log.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	log.SetSeverityNumber(1)
	log.SetTraceID(traceID)
	log.SetSpanID(spanID)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "A|B|C")

}

func fillLogTwo(log plog.LogRecord) {
	log.Body().SetStr("operationB")
	log.SetTimestamp(TestLogTimestamp)
	log.SetObservedTimestamp(TestObservedTimestamp)
	log.Attributes().PutStr("http.method", "get")
	log.Attributes().PutStr("http.path", "/health")
	log.Attributes().PutStr("http.url", "http://localhost/health")
	log.Attributes().PutStr("flags", "C|D")

}
