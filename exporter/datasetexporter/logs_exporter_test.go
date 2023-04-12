// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasetexporter

import (
	"context"
	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"go.opentelemetry.io/collector/pdata/plog"
	"os"
	"testing"

	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type SuiteLogsExporter struct{}

func (s *SuiteLogsExporter) PreTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteLogsExporter) PostTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteLogsExporter) Destroy(t *td.T) error {
	os.Clearenv()
	return nil
}

func TestSuiteLogsExporter(t *testing.T) {
	td.NewT(t)
	tdsuite.Run(t, &SuiteLogsExporter{})
}

func (s *SuiteLogsExporter) TestCreateLogsExporter(assert, require *td.T) {
	ctx := context.Background()
	createSettings := exportertest.NewNopCreateSettings()
	tests := createExporterTests()

	for _, tt := range tests {
		require.Run(tt.name, func(*td.T) {
			exporterInstance = nil
			logs, err := createLogsExporter(ctx, createSettings, tt.config)

			if err == nil {
				assert.Nil(tt.expectedError)
				assert.NotNil(logs)
			} else {
				assert.Cmp(err.Error(), tt.expectedError.Error())
				assert.Nil(logs)
			}
		})
	}
}

func (s *SuiteLogsExporter) TestBuildBody(assert, require *td.T) {
	slice := pcommon.NewValueSlice()
	slice.FromRaw([]any{1, 2, 3})

	bytes := pcommon.NewValueBytes()
	bytes.FromRaw([]byte{byte(65), byte(66), byte(67)})
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
		require.Run(tt.key, func(*td.T) {
			attrs := make(map[string]interface{})
			msg := buildBody(attrs, tt.body)
			expectedAttrs := make(map[string]interface{})
			expectedAttrs["body.type"] = tt.body.Type().String()
			expectedAttrs[tt.key] = tt.value

			assert.Cmp(msg, tt.message, tt.key)
			assert.Cmp(attrs, expectedAttrs, tt.key)
		})
	}
}

func (s *SuiteLogsExporter) TestBuildBodyMap(assert, require *td.T) {
	m := pcommon.NewValueMap()
	err := m.FromRaw(map[string]any{
		"scalar": "scalar-value",
		"map": map[string]any{
			"m1": "v1",
			"m2": "v2",
		},
		"array": []any{1, 2, 3},
	})
	if assert.CmpNoError(err) {
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

		assert.Cmp(msg, expectedMsg)
		assert.Cmp(attrs, expectedAttrs)
	}
}

var testEventRaw = &add_events.Event{
	Thread: "TL",
	Log:    "LL",
	Sev:    9,
	Ts:     "1581452773000000789",
	Attrs: map[string]interface{}{
		"OTEL_TYPE":                         "log",
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

var testEventReq = &add_events.Event{
	Thread: testEventRaw.Thread,
	Log:    testEventRaw.Log,
	Sev:    testEventRaw.Sev,
	Ts:     testEventRaw.Ts,
	Attrs: map[string]interface{}{
		"OTEL_TYPE":                         "log",
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

var testThread = &add_events.Thread{
	Id:   "TL",
	Name: "logs",
}

var testLog = &add_events.Log{
	Id:    "LL",
	Attrs: map[string]interface{}{},
}

func (s *SuiteLogsExporter) TestBuildEventFromLog(assert, require *td.T) {
	lr := testdata.GenerateLogsOneLogRecord()
	ld := lr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	expected := &add_events.EventBundle{
		Event:  testEventRaw,
		Thread: testThread,
		Log:    testLog,
	}
	was := buildEventFromLog(
		ld,
		lr.ResourceLogs().At(0).Resource(),
		lr.ResourceLogs().At(0).ScopeLogs().At(0).Scope(),
	)

	assert.Cmp(was, expected)
}
