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
	"os"
	"testing"

	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

type SuiteTracesExporter struct{}

func (s *SuiteTracesExporter) PreTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteTracesExporter) PostTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteTracesExporter) Destroy(t *td.T) error {
	os.Clearenv()
	return nil
}

func TestSuiteTracesExporter(t *testing.T) {
	td.NewT(t)
	tdsuite.Run(t, &SuiteTracesExporter{})
}

func (s *SuiteTracesExporter) TestCreateTracesExporter(assert, require *td.T) {
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

var testTEventRaw = &add_events.Event{
	Thread: "TT",
	Log:    "LT",
	Sev:    9,
	Ts:     "1581452772000000321",
	Attrs: map[string]interface{}{
		"OTEL_TYPE":    "trace",
		"sca:schemVer": 1,
		"sca:schema":   "tracing",
		"sca:type":     "span",

		"name": "operationA",
		"kind": "Unspecified",

		"start_time_unix_nano": int64(1581452772000000321),
		"end_time_unix_nano":   int64(1581452773000000789),
		"duration_nano":        int64(1000000468),

		"message":                "OtelExporter - Span - operationA: ",
		"span_id":                "",
		"trace_id":               "",
		"parent_span_id":         "",
		"resource_resource-attr": "resource-attr-val-1",
		"status_code":            "Error",
		"status_message":         "status-cancelled",
	},
}

var testTEventReq = &add_events.Event{
	Thread: testTEventRaw.Thread,
	Log:    testTEventRaw.Log,
	Sev:    testTEventRaw.Sev,
	Ts:     testTEventRaw.Ts,
	Attrs: map[string]interface{}{
		"OTEL_TYPE":    "trace",
		"sca:schemVer": 1,
		"sca:schema":   "tracing",
		"sca:type":     "span",

		"name": "operationA",
		"kind": "Unspecified",

		"start_time_unix_nano": int64(1581452772000000321),
		"end_time_unix_nano":   int64(1581452773000000789),
		"duration_nano":        int64(1000000468),

		"message":                "OtelExporter - Span - operationA: ",
		"span_id":                "",
		"trace_id":               "",
		"parent_span_id":         "",
		"resource_resource-attr": "resource-attr-val-1",
		"status_code":            "Error",
		"status_message":         "status-cancelled",
	},
}

var testTThread = &add_events.Thread{
	Id:   "TT",
	Name: "traces",
}

var testTLog = &add_events.Log{
	Id:    "LT",
	Attrs: map[string]interface{}{},
}

func (s *SuiteTracesExporter) TestBuildEventFromSpanOne(assert, require *td.T) {
	traces := testdata.GenerateTracesOneSpan()
	span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	expected := &add_events.EventBundle{
		Event:  testTEventRaw,
		Thread: testTThread,
		Log:    testTLog,
	}
	was := buildEventFromSpan(
		span,
		traces.ResourceSpans().At(0).Resource(),
		traces.ResourceSpans().At(0).ScopeSpans().At(0).Scope(),
	)

	assert.Cmp(was, expected)
}
