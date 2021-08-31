// Copyright  OpenTelemetry Authors
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

package observiqexporter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

func resourceAndLogRecordsToLogs(r pdata.Resource, lrs []pdata.LogRecord) pdata.Logs {
	logs := pdata.NewLogs()
	resLogs := logs.ResourceLogs()

	resLog := resLogs.AppendEmpty()
	resLogRes := resLog.Resource()

	r.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		resLogRes.Attributes().Insert(k, v)
		return true
	})

	resLog.InstrumentationLibraryLogs().EnsureCapacity(len(lrs))
	for _, l := range lrs {
		ills := resLog.InstrumentationLibraryLogs().AppendEmpty()
		l.CopyTo(ills.Logs().AppendEmpty())
	}

	return logs
}

func TestLogdataToObservIQFormat(t *testing.T) {
	ts := time.Date(2021, 12, 11, 10, 9, 8, 1, time.UTC)
	stringTs := "2021-12-11T10:09:08.000Z"
	nanoTs := pdata.Timestamp(ts.UnixNano())
	timeNow = func() time.Time {
		return ts
	}

	testCases := []struct {
		name          string
		logRecordFn   func() pdata.LogRecord
		logResourceFn func() pdata.Resource
		agentName     string
		agentID       string
		output        observIQLogEntry
		expectErr     bool
	}{
		{
			"Happy path with string attributes",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("Message")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(nanoTs)
				return logRecord
			},
			pdata.NewResource,
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Message:   "Message",
				Severity:  "default",
				Resource:  map[string]interface{}{},
				Data: map[string]interface{}{
					strings.ReplaceAll(conventions.AttributeServiceName, ".", "_"): "myapp",
					strings.ReplaceAll(conventions.AttributeHostName, ".", "_"):    "myhost",
					"custom": "custom",
				},
				Agent: &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
			},
			false,
		},
		{
			"works with attributes of all types",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("Message")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertBool("bool", true)
				logRecord.Attributes().InsertDouble("double", 1.0)
				logRecord.Attributes().InsertInt("int", 3)
				logRecord.Attributes().InsertNull("null")

				mapVal := pdata.NewAttributeValueMap()
				mapVal.MapVal().Insert("mapKey", pdata.NewAttributeValueString("value"))
				logRecord.Attributes().Insert("map", mapVal)

				arrVal := pdata.NewAttributeValueArray()
				arrVal.ArrayVal().EnsureCapacity(2)
				arrVal.ArrayVal().AppendEmpty().SetIntVal(1)
				arrVal.ArrayVal().AppendEmpty().SetIntVal(2)
				logRecord.Attributes().Insert("array", arrVal)

				logRecord.SetTimestamp(nanoTs)
				return logRecord
			},
			func() pdata.Resource {
				res := pdata.NewResource()
				res.Attributes().InsertBool("bool", true)
				res.Attributes().InsertString("string", "string")
				res.Attributes().InsertInt("int", 1)
				res.Attributes().InsertNull("null")

				mapVal := pdata.NewAttributeValueMap()
				mapVal.MapVal().InsertDouble("double", 1.1)
				mapVal.MapVal().InsertBool("bool", false)
				mapVal.MapVal().InsertNull("null")
				res.Attributes().Insert("map", mapVal)

				arrVal := pdata.NewAttributeValueArray()
				arrVal.ArrayVal().EnsureCapacity(2)
				arrVal.ArrayVal().AppendEmpty().SetIntVal(1)
				arrVal.ArrayVal().AppendEmpty().SetDoubleVal(2.0)
				res.Attributes().Insert("array", arrVal)

				return res
			},
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Message:   "Message",
				Severity:  "default",
				Resource: map[string]interface{}{
					"bool":   true,
					"string": "string",
					// Note here about int values -- while this IS an int value,
					// we marshal then unmarshal json to get this value --
					// which turns it into a float
					"int": float64(1),
					"map": map[string]interface{}{
						"double": float64(1.1),
						"bool":   false,
					},
					"array": []interface{}{float64(1), float64(2.0)},
				},
				Data: map[string]interface{}{
					strings.ReplaceAll(conventions.AttributeServiceName, ".", "_"): "myapp",
					"bool":   true,
					"double": float64(1.0),
					"int":    float64(3),
					"map": map[string]interface{}{
						"mapKey": "value",
					},
					"array": []interface{}{float64(1), float64(2)},
				},
				Agent: &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
			},
			false,
		},
		{
			"Body is nil",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.SetTimestamp(nanoTs)
				return logRecord
			},
			pdata.NewResource,
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Message:   "",
				Severity:  "default",
				Resource:  map[string]interface{}{},
				Data:      nil,
				Agent:     &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
			},
			false,
		},
		{
			"Body is map",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()

				mapVal := pdata.NewAttributeValueMap()
				mapVal.MapVal().Insert("mapKey", pdata.NewAttributeValueString("value"))
				mapVal.CopyTo(logRecord.Body())

				logRecord.SetTimestamp(nanoTs)
				return logRecord
			},
			pdata.NewResource,
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Severity:  "default",
				Resource:  map[string]interface{}{},
				Data:      nil,
				Agent:     &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
				Body: map[string]interface{}{
					"mapKey": "value",
				},
			},
			false,
		},
		{
			"Body is array",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()

				pdata.NewAttributeValueArray().CopyTo(logRecord.Body())
				logRecord.Body().ArrayVal().EnsureCapacity(2)
				logRecord.Body().ArrayVal().AppendEmpty().SetStringVal("string")
				logRecord.Body().ArrayVal().AppendEmpty().SetDoubleVal(1.0)

				logRecord.SetTimestamp(nanoTs)
				return logRecord
			},
			pdata.NewResource,
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Severity:  "default",
				Resource:  map[string]interface{}{},
				Data:      nil,
				Agent:     &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
				Body: []interface{}{
					"string",
					float64(1.0),
				},
			},
			false,
		},
		{
			"Body is an int",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()

				logRecord.Body().SetIntVal(1)

				logRecord.SetTimestamp(nanoTs)
				return logRecord
			},
			pdata.NewResource,
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Severity:  "default",
				Resource:  map[string]interface{}{},
				Data:      nil,
				Agent:     &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
				Body:      float64(1.0),
			},
			false,
		},
		{
			"Body and attributes are maps",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()

				bodyMapVal := pdata.NewAttributeValueMap()
				bodyMapVal.MapVal().Insert("mapKey", pdata.NewAttributeValueString("body"))
				bodyMapVal.CopyTo(logRecord.Body())

				logRecord.Attributes().InsertString("attrib", "logAttrib")

				logRecord.SetTimestamp(nanoTs)
				return logRecord
			},
			pdata.NewResource,
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Severity:  "default",
				Resource:  map[string]interface{}{},
				Data: map[string]interface{}{
					"attrib": "logAttrib",
				},
				Agent: &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
				Body: map[string]interface{}{
					"mapKey": "body",
				},
			},
			false,
		},
		{
			"No timestamp on record",
			func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("Message")
				return logRecord
			},
			pdata.NewResource,
			"agent",
			"agentID",
			observIQLogEntry{
				Timestamp: stringTs,
				Message:   "Message",
				Severity:  "default",
				Resource:  map[string]interface{}{},
				Data:      nil,
				Agent:     &observIQAgentInfo{Name: "agent", ID: "agentID", Version: "latest"},
			},
			false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logs := resourceAndLogRecordsToLogs(testCase.logResourceFn(), []pdata.LogRecord{testCase.logRecordFn()})
			res, errs := logdataToObservIQFormat(
				logs,
				testCase.agentID,
				testCase.agentName,
				component.DefaultBuildInfo().Version)

			if testCase.expectErr {
				require.NotEmpty(t, errs)
			} else {
				require.Empty(t, errs)
			}

			require.Len(t, res.Logs, 1)

			// ID must be of length 32
			require.Len(t, res.Logs[0].ID, 32)

			oiqLogEntry := observIQLogEntry{}
			err := json.Unmarshal(res.Logs[0].Entry, &oiqLogEntry)

			if testCase.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, testCase.output, oiqLogEntry)
		})
	}
}
