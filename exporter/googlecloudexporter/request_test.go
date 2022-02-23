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

package googlecloudexporter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/logging/v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/testhelpers"
)

func BenchmarkBuildRequest(b *testing.B) {
	entryBuilder := &GoogleEntryBuilder{
		MaxEntrySize: defaultMaxEntrySize,
		ProjectID:    "project",
	}

	logs := pdata.NewLogs()
	resourceLogsSlice := pdata.NewResourceLogsSlice()
	resourceLogs := resourceLogsSlice.AppendEmpty()
	instrumentationLibraryLogsSlice := pdata.NewInstrumentationLibraryLogsSlice()
	instrumentationLibraryLogs := instrumentationLibraryLogsSlice.AppendEmpty()
	logRecordSlice := pdata.NewLogRecordSlice()

	for i := 0; i < 1000; i++ {
		logRecord := logRecordSlice.AppendEmpty()
		populateLogRecord(logRecord, fmt.Sprint(i))
	}

	logRecordSlice.CopyTo(instrumentationLibraryLogs.LogRecords())
	instrumentationLibraryLogsSlice.CopyTo(resourceLogs.InstrumentationLibraryLogs())
	resourceLogsSlice.CopyTo(logs.ResourceLogs())

	requestBuilder := GoogleRequestBuilder{
		MaxRequestSize: 10000,
		ProjectID:      "test_project",
		EntryBuilder:   entryBuilder,
		SugaredLogger:  zap.NewNop().Sugar(),
	}

	requests := requestBuilder.Build(&logs)
	require.Len(b, requests, 7)
}

func TestBuildRequest(t *testing.T) {
	logs := pdata.NewLogs()
	resourceLogsSlice := pdata.NewResourceLogsSlice()
	resourceLogs := resourceLogsSlice.AppendEmpty()
	instrumentationLibraryLogsSlice := pdata.NewInstrumentationLibraryLogsSlice()
	instrumentationLibraryLogs := instrumentationLibraryLogsSlice.AppendEmpty()
	logRecordSlice := pdata.NewLogRecordSlice()

	logRecordOne := logRecordSlice.AppendEmpty()
	populateLogRecord(logRecordOne, "request 1")
	logRecordTwo := logRecordSlice.AppendEmpty()
	populateLogRecord(logRecordTwo, "request 2")
	logRecordThree := logRecordSlice.AppendEmpty()
	populateLogRecord(logRecordThree, "request 3")
	logRecordFour := logRecordSlice.AppendEmpty()
	populateLogRecord(logRecordFour, "request 4")

	logRecordSlice.CopyTo(instrumentationLibraryLogs.LogRecords())
	instrumentationLibraryLogsSlice.CopyTo(resourceLogs.InstrumentationLibraryLogs())
	resourceLogsSlice.CopyTo(logs.ResourceLogs())

	resultOne := &logging.LogEntry{Payload: &logging.LogEntry_TextPayload{TextPayload: "request 1"}}
	resultTwo := &logging.LogEntry{Payload: &logging.LogEntry_TextPayload{TextPayload: "request 2"}}
	resultFour := &logging.LogEntry{Payload: &logging.LogEntry_TextPayload{TextPayload: "request 4"}}

	entryBuilder := &testhelpers.MockEntryBuilder{}
	entryBuilder.On("Build", &logRecordOne).Return(resultOne, nil)
	entryBuilder.On("Build", &logRecordTwo).Return(resultTwo, nil)
	entryBuilder.On("Build", &logRecordThree).Return(nil, errors.New("error"))
	entryBuilder.On("Build", &logRecordFour).Return(resultFour, nil)

	requestBuilder := GoogleRequestBuilder{
		MaxRequestSize: 100,
		ProjectID:      "test_project",
		EntryBuilder:   entryBuilder,
		SugaredLogger:  zap.NewNop().Sugar(),
	}

	requests := requestBuilder.Build(&logs)
	require.Len(t, requests, 2)

	require.Len(t, requests[0].Entries, 2)
	require.Len(t, requests[1].Entries, 1)
	require.Equal(t, requests[0].Entries, []*logging.LogEntry{resultOne, resultTwo})
	require.Equal(t, requests[1].Entries, []*logging.LogEntry{resultFour})
}

func TestImpossibleEntry(t *testing.T) {
	logs := pdata.NewLogs()
	resourceLogsSlice := pdata.NewResourceLogsSlice()
	resourceLogs := resourceLogsSlice.AppendEmpty()
	instrumentationLibraryLogsSlice := pdata.NewInstrumentationLibraryLogsSlice()
	instrumentationLibraryLogs := instrumentationLibraryLogsSlice.AppendEmpty()
	logRecordSlice := pdata.NewLogRecordSlice()

	logRecordOne := logRecordSlice.AppendEmpty()
	populateLogRecord(logRecordOne, "Test Request")

	logRecordSlice.CopyTo(instrumentationLibraryLogs.LogRecords())
	instrumentationLibraryLogsSlice.CopyTo(resourceLogs.InstrumentationLibraryLogs())
	resourceLogsSlice.CopyTo(logs.ResourceLogs())

	resultOne := &logging.LogEntry{Payload: &logging.LogEntry_TextPayload{TextPayload: "Test Request"}}

	entryBuilder := &testhelpers.MockEntryBuilder{}
	entryBuilder.On("Build", &logRecordOne).Return(resultOne, nil)

	requestBuilder := GoogleRequestBuilder{
		MaxRequestSize: 1,
		ProjectID:      "test_project",
		EntryBuilder:   entryBuilder,
		SugaredLogger:  zap.NewNop().Sugar(),
	}

	requests := requestBuilder.Build(&logs)
	require.Len(t, requests, 0)
}

func populateLogRecord(logRecord pdata.LogRecord, data string) {
	testhelpers.NewLogRecordBuilderWithLogRecord(logRecord).
		WithBodyString(data).
		WithTimeStamp().
		WithSpanID([...]byte{0, 1, 0, 0, 0, 0, 0, 0}).
		WithTraceID([...]byte{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}).
		WithEmptyAttributes().
		Build()
}
