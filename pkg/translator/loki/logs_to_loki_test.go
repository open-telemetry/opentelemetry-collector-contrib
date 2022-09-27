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

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestLogsToLoki(t *testing.T) {
	testCases := []struct {
		desc          string
		hints         map[string]interface{}
		attrs         map[string]interface{}
		res           map[string]interface{}
		expectedLabel string
		expectedLines []string
	}{
		{
			desc: "with attribute to label and regular attribute",
			attrs: map[string]interface{}{
				"host.name":   "guarana",
				"http.status": 200,
			},
			hints: map[string]interface{}{
				hintAttributes: "host.name",
			},
			expectedLabel: `{exporter="OTLP", host.name="guarana"}`,
			expectedLines: []string{
				`{"traceid":"01020304000000000000000000000000","attributes":{"http.status":200}}`,
				`{"traceid":"01020304050000000000000000000000","attributes":{"http.status":200}}`,
				`{"traceid":"01020304050600000000000000000000","attributes":{"http.status":200}}`,
			},
		},
		{
			desc: "with resource to label and regular resource",
			res: map[string]interface{}{
				"host.name": "guarana",
				"region.az": "eu-west-1a",
			},
			hints: map[string]interface{}{
				hintResources: "host.name",
			},
			expectedLabel: `{exporter="OTLP", host.name="guarana"}`,
			expectedLines: []string{
				`{"traceid":"01020304000000000000000000000000","resources":{"region.az":"eu-west-1a"}}`,
				`{"traceid":"01020304050000000000000000000000","resources":{"region.az":"eu-west-1a"}}`,
				`{"traceid":"01020304050600000000000000000000","resources":{"region.az":"eu-west-1a"}}`,
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			// prepare
			ld := plog.NewLogs()
			ld.ResourceLogs().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5}))
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(2).SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6}))

			// copy the attributes from the test case to the log entry
			if len(tC.attrs) > 0 {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().FromRaw(tC.attrs)
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().FromRaw(tC.attrs)
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(2).Attributes().FromRaw(tC.attrs)
			}
			if len(tC.res) > 0 {
				ld.ResourceLogs().At(0).Resource().Attributes().FromRaw(tC.res)
			}

			// we can't use copy here, as the value (Value) will be used as string lookup later, so, we need to convert it to string now
			for k, v := range tC.hints {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutString(k, fmt.Sprintf("%v", v))
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().PutString(k, fmt.Sprintf("%v", v))
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(2).Attributes().PutString(k, fmt.Sprintf("%v", v))
			}

			// test
			pushRequest, report := LogsToLoki(ld)
			entries := pushRequest.Streams[0].Entries

			var entriesLines []string
			for i := 0; i < len(entries); i++ {
				entriesLines = append(entriesLines, entries[i].Line)
			}

			// actualPushRequest is populated within the test http server, we check it here as assertions are better done at the
			// end of the test function
			assert.Empty(t, report.Errors)
			assert.Equal(t, 0, report.NumDropped)
			assert.Equal(t, ld.LogRecordCount(), report.NumSubmitted)
			assert.Len(t, pushRequest.Streams, 1)
			assert.Equal(t, tC.expectedLabel, pushRequest.Streams[0].Labels)
			assert.Len(t, entries, ld.LogRecordCount())
			assert.ElementsMatch(t, tC.expectedLines, entriesLines)
		})
	}
}
