// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
)

func fillComplexAttributeValueMap(m pcommon.Map) {
	m.PutBool("result", true)
	m.PutStr("status", "ok")
	m.PutDouble("value", 1.3)
	m.PutInt("code", 200)
	m.PutEmpty("null")
	m.PutEmptySlice("array").AppendEmpty().SetStr("array")
	m.PutEmptyMap("map").PutStr("data", "hello world")
	m.PutStr("status", "ok")
}

func createLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("resourceKey", "resourceValue")
	rl.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "test-log-service-exporter")
	rl.Resource().Attributes().PutStr(string(conventions.HostNameKey), "test-host")
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
			logRecord.Body().SetBool(true)
		case 2:
			logRecord.Body().SetInt(2.0)
		case 3:
			logRecord.Body().SetDouble(3.0)
		case 4:
			logRecord.Body().SetStr("4")
		case 5:
			fillComplexAttributeValueMap(logRecord.Attributes().PutEmptyMap("map-value"))
			logRecord.Body().SetStr("log contents")
		case 6:
			logRecord.Attributes().PutEmptySlice("array-value").AppendEmpty().SetStr("array")
			logRecord.Body().SetStr("log contents")
		default:
			logRecord.Body().SetStr("log contents")
		}
		logRecord.Attributes().PutStr(string(conventions.ServiceNameKey), "myapp")
		logRecord.Attributes().PutStr("my-label", "myapp-type")
		logRecord.Attributes().PutStr(string(conventions.HostNameKey), "myhost")
		logRecord.Attributes().PutStr("custom", "custom")
		logRecord.Attributes().PutEmpty("null-value")

		logRecord.SetTimestamp(ts)
	}

	return logs
}

func TestLogsDataToLogService(t *testing.T) {
	totalLogCount := 10
	validLogCount := totalLogCount - 1
	gotLogs := logDataToLogService(createLogData(10))
	assert.Len(t, gotLogs, 9)

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
		assert.Equal(t, wantLogs[j], gotLogPairs[j])
	}
}
