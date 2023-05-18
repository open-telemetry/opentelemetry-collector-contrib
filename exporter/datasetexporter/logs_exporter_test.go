// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"github.com/scalyr/dataset-go/pkg/api/request"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

type SuiteLogsExporter struct {
	suite.Suite
}

func (s *SuiteLogsExporter) SetupTest() {
	os.Clearenv()
}

func TestSuiteLogsExporter(t *testing.T) {
	suite.Run(t, new(SuiteLogsExporter))
}

func (s *SuiteLogsExporter) TestCreateLogsExporter() {
	ctx := context.Background()
	createSettings := exportertest.NewNopCreateSettings()
	tests := createExporterTests()

	for _, tt := range tests {
		s.T().Run(tt.name, func(*testing.T) {
			logs, err := createLogsExporter(ctx, createSettings, tt.config)

			if err == nil {
				s.Nil(tt.expectedError, tt.name)
				s.NotNil(logs, tt.name)
			} else {
				if tt.expectedError == nil {
					s.Nil(err, tt.name)
				} else {
					s.Equal(tt.expectedError.Error(), err.Error(), tt.name)
					s.Nil(logs, tt.name)
				}
			}
		})
	}
}

func (s *SuiteLogsExporter) TestBuildBody() {
	slice := pcommon.NewValueSlice()
	err := slice.FromRaw([]any{1, 2, 3})
	s.NoError(err)

	bytes := pcommon.NewValueBytes()
	err = bytes.FromRaw([]byte{byte(65), byte(66), byte(67)})
	s.NoError(err)
	tests := []struct {
		body    pcommon.Value
		key     string
		value   interface{}
		message string
	}{
		{
			body:    pcommon.NewValueEmpty(),
			key:     "body.empty",
			value:   "",
			message: "",
		},
		{
			body:    pcommon.NewValueStr("foo"),
			key:     "body.str",
			value:   "foo",
			message: "foo",
		},
		{
			body:    pcommon.NewValueBool(true),
			key:     "body.bool",
			value:   true,
			message: "true",
		},
		{
			body:    pcommon.NewValueDouble(42.5),
			key:     "body.double",
			value:   float64(42.5),
			message: "42.5",
		},
		{
			body:    pcommon.NewValueInt(42),
			key:     "body.int",
			value:   int64(42),
			message: "42",
		},
		{
			body:    bytes,
			key:     "body.bytes",
			value:   "QUJD",
			message: "QUJD",
		},
		{
			body:    slice,
			key:     "body.slice",
			value:   []interface{}{int64(1), int64(2), int64(3)},
			message: "[1,2,3]",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.key, func(*testing.T) {
			attrs := make(map[string]interface{})
			msg := buildBody(attrs, tt.body)
			expectedAttrs := make(map[string]interface{})
			expectedAttrs["body.type"] = tt.body.Type().String()
			expectedAttrs[tt.key] = tt.value

			s.Equal(tt.message, msg, tt.key)
			s.Equal(expectedAttrs, attrs, tt.key)
		})
	}
}

func (s *SuiteLogsExporter) TestBuildBodyMap() {
	m := pcommon.NewValueMap()
	err := m.FromRaw(map[string]any{
		"scalar": "scalar-value",
		"map": map[string]any{
			"m1": "v1",
			"m2": "v2",
		},
		"array": []any{1, 2, 3},
	})
	if s.NoError(err) {
		attrs := make(map[string]interface{})
		msg := buildBody(attrs, m)
		expectedAttrs := make(map[string]interface{})
		expectedAttrs["body.type"] = pcommon.ValueTypeMap.String()
		expectedAttrs["body.map.scalar"] = "scalar-value"
		expectedAttrs["body.map.map.m1"] = "v1"
		expectedAttrs["body.map.map.m2"] = "v2"
		expectedAttrs["body.map.array.0"] = int64(1)
		expectedAttrs["body.map.array.1"] = int64(2)
		expectedAttrs["body.map.array.2"] = int64(3)

		expectedMsg := "{\"array\":[1,2,3],\"map\":{\"m1\":\"v1\",\"m2\":\"v2\"},\"scalar\":\"scalar-value\"}"

		s.Equal(expectedMsg, msg)
		s.Equal(expectedAttrs, attrs)
	}
}

var testLEventRaw = &add_events.Event{
	Thread: "TL",
	Log:    "LL",
	Sev:    9,
	Ts:     "1581452773000000789",
	Attrs: map[string]interface{}{
		"attributes.app":                    "server",
		"attributes.instance_num":           int64(1),
		"body.str":                          "This is a log message",
		"body.type":                         "Str",
		"dropped_attributes_count":          uint32(1),
		"flag.is_sampled":                   false,
		"flags":                             plog.LogRecordFlags(0),
		"message":                           "OtelExporter - Log - This is a log message",
		"resource.attributes.resource-attr": "resource-attr-val-1",
		"scope.name":                        "",
		"severity.number":                   plog.SeverityNumberInfo,
		"severity.text":                     "Info",
		"span_id":                           "0102040800000000",
		"timestamp":                         "2020-02-11 20:26:13.000000789 +0000 UTC",
		"trace_id":                          "08040201000000000000000000000000",
	},
}

var testLEventReq = &add_events.Event{
	Thread: testLEventRaw.Thread,
	Log:    testLEventRaw.Log,
	Sev:    testLEventRaw.Sev,
	Ts:     testLEventRaw.Ts,
	Attrs: map[string]interface{}{
		"attributes.app":                    "server",
		"attributes.instance_num":           float64(1),
		"body.str":                          "This is a log message",
		"body.type":                         "Str",
		"dropped_attributes_count":          float64(1),
		"flag.is_sampled":                   false,
		"flags":                             float64(plog.LogRecordFlags(0)),
		"message":                           "OtelExporter - Log - This is a log message",
		"resource.attributes.resource-attr": "resource-attr-val-1",
		"scope.name":                        "",
		"severity.number":                   float64(plog.SeverityNumberInfo),
		"severity.text":                     "Info",
		"span_id":                           "0102040800000000",
		"timestamp":                         "2020-02-11 20:26:13.000000789 +0000 UTC",
		"trace_id":                          "08040201000000000000000000000000",
		"bundle_key":                        "d41d8cd98f00b204e9800998ecf8427e",
	},
}

var testLThread = &add_events.Thread{
	Id:   "TL",
	Name: "logs",
}

var testLLog = &add_events.Log{
	Id:    "LL",
	Attrs: map[string]interface{}{},
}

func (s *SuiteLogsExporter) TestBuildEventFromLog() {
	lr := testdata.GenerateLogsOneLogRecord()
	ld := lr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	expected := &add_events.EventBundle{
		Event:  testLEventRaw,
		Thread: testLThread,
		Log:    testLLog,
	}
	was := buildEventFromLog(
		ld,
		lr.ResourceLogs().At(0).Resource(),
		lr.ResourceLogs().At(0).ScopeLogs().At(0).Scope(),
	)

	s.Equal(expected, was)
}

func extract(req *http.Request) (add_events.AddEventsRequest, error) {
	data, _ := io.ReadAll(req.Body)
	b := bytes.NewBuffer(data)
	reader, _ := gzip.NewReader(b)

	var resB bytes.Buffer
	_, _ = resB.ReadFrom(reader)

	cer := &add_events.AddEventsRequest{}
	err := json.Unmarshal(resB.Bytes(), cer)
	return *cer, err
}

func (s *SuiteLogsExporter) TestConsumeLogsShouldSucceed() {
	createSettings := exportertest.NewNopCreateSettings()

	attempt := atomic.Uint64{}
	wasSuccessful := atomic.Bool{}
	addRequest := add_events.AddEventsRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempt.Add(1)
		cer, err := extract(req)
		addRequest = cer

		s.NoError(err, "Error reading request: %v", err)

		wasSuccessful.Store(true)
		payload, err := json.Marshal(map[string]interface{}{
			"status":       "success",
			"bytesCharged": 42,
		})
		s.NoError(err)
		l, err := w.Write(payload)
		s.Greater(l, 1)
		s.NoError(err)
	}))
	defer server.Close()

	config := &Config{
		DatasetURL: server.URL,
		APIKey:     "key-lib",
		BufferSettings: BufferSettings{
			MaxLifetime: time.Millisecond,
			GroupBy:     []string{"attributes.container_id"},
		},
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}

	lr := testdata.GenerateLogsOneLogRecord()

	logs, err := createLogsExporter(context.Background(), createSettings, config)
	if s.NoError(err) {
		err = logs.Start(context.Background(), componenttest.NewNopHost())
		s.NoError(err)

		s.NotNil(logs)
		err = logs.ConsumeLogs(context.Background(), lr)
		s.Nil(err)
		time.Sleep(time.Second)
		err = logs.Shutdown(context.Background())
		s.Nil(err)
	}

	s.True(wasSuccessful.Load())
	s.Equal(
		add_events.AddEventsRequest{
			AuthParams: request.AuthParams{
				Token: "key-lib",
			},
			AddEventsRequestParams: add_events.AddEventsRequestParams{
				Session: addRequest.Session,
				Events:  []*add_events.Event{testLEventReq},
				Threads: []*add_events.Thread{testLThread},
				Logs:    []*add_events.Log{testLLog},
			},
		},
		addRequest,
	)
}
