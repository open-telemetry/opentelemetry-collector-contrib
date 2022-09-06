// Copyright 2020, OpenTelemetry Authors
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

package alibabacloudlogserviceexporter

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func fillComplexAttributeValueMap(m pcommon.Map) {
	m.UpsertBool("result", true)
	m.UpsertString("status", "ok")
	m.UpsertDouble("value", 1.3)
	m.UpsertInt("code", 200)
	m.UpsertEmpty("null")
	m.UpsertEmptySlice("array").AppendEmpty().SetStringVal("array")
	m.UpsertEmptyMap("map").UpsertString("data", "hello world")
	m.UpsertString("status", "ok")
}

func createLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().UpsertString("resouceKey", "resourceValue")
	rl.Resource().Attributes().UpsertString(conventions.AttributeServiceName, "test-log-service-exporter")
	rl.Resource().Attributes().UpsertString(conventions.AttributeHostName, "test-host")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("collector")
	sl.Scope().SetVersion("v0.1.0")

	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		switch i {
		case 0:
			// do nothing, left body null
		case 1:
			logRecord.Body().SetBoolVal(true)
		case 2:
			logRecord.Body().SetIntVal(2.0)
		case 3:
			logRecord.Body().SetDoubleVal(3.0)
		case 4:
			logRecord.Body().SetStringVal("4")
		case 5:
			fillComplexAttributeValueMap(logRecord.Attributes().UpsertEmptyMap("map-value"))
			logRecord.Body().SetStringVal("log contents")
		case 6:
			logRecord.Attributes().UpsertEmptySlice("array-value").AppendEmpty().SetStringVal("array")
			logRecord.Body().SetStringVal("log contents")
		default:
			logRecord.Body().SetStringVal("log contents")
		}
		logRecord.Attributes().UpsertString(conventions.AttributeServiceName, "myapp")
		logRecord.Attributes().UpsertString("my-label", "myapp-type")
		logRecord.Attributes().UpsertString(conventions.AttributeHostName, "myhost")
		logRecord.Attributes().UpsertString("custom", "custom")
		logRecord.Attributes().UpsertEmpty("null-value")

		logRecord.SetTimestamp(ts)
	}

	return logs
}

func TestLogsDataToLogService(t *testing.T) {
	totalLogCount := 10
	validLogCount := totalLogCount - 1
	gotLogs := logDataToLogService(createLogData(10))
	assert.Equal(t, len(gotLogs), 9)

	gotLogPairs := make([][]logKeyValuePair, 0, len(gotLogs))

	for _, log := range gotLogs {
		pairs := make([]logKeyValuePair, 0, len(log.Contents))
		for _, content := range log.Contents {
			pairs = append(pairs, logKeyValuePair{
				Key:   content.GetKey(),
				Value: content.GetValue(),
			})
		}
		gotLogPairs = append(gotLogPairs, pairs)

	}

	wantLogs := make([][]logKeyValuePair, 0, validLogCount)
	resultLogFile := "./testdata/logservice_log_data.json"
	if err := loadFromJSON(resultLogFile, &wantLogs); err != nil {
		t.Errorf("Failed load log key value pairs from %q: %v", resultLogFile, err)
		return
	}
	for j := 0; j < validLogCount; j++ {

		sort.Sort(logKeyValuePairs(gotLogPairs[j]))
		sort.Sort(logKeyValuePairs(wantLogs[j]))
		if !reflect.DeepEqual(gotLogPairs[j], wantLogs[j]) {
			t.Errorf("Unsuccessful conversion \nGot:\n\t%v\nWant:\n\t%v", gotLogPairs, wantLogs)
		}
	}
}
