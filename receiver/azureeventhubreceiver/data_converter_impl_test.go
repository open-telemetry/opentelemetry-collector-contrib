// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver

import (
	"encoding/hex"
	"encoding/json"
	"github.com/go-test/deep"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestAsTimestamp(t *testing.T) {
	timestamp := "2022-11-11T04:48:27.6767145Z"
	nanos, err := AsTimestamp(timestamp)
	assert.NoError(t, err)
	assert.Less(t, pcommon.Timestamp(0), nanos)

	timestamp = "invalid-time"
	nanos, err = AsTimestamp(timestamp)
	assert.Error(t, err)
	assert.Equal(t, pcommon.Timestamp(0), nanos)
}

func TestAsSeverity(t *testing.T) {
	tests := map[string]plog.SeverityNumber{
		"Informational": plog.SeverityNumberInfo,
		"Warning":       plog.SeverityNumberWarn,
		"Error":         plog.SeverityNumberError,
		"Critical":      plog.SeverityNumberFatal,
		"unknown":       plog.SeverityNumberUnspecified,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, AsSeverity(input))
		})
	}
}

func TestSetIf(t *testing.T) {
	m := map[string]interface{}{}

	SetIf(m, "key", nil)
	actual, found := m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v := ""
	SetIf(m, "key", &v)
	actual, found = m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v = "ok"
	SetIf(m, "key", &v)
	actual, found = m["key"]
	assert.True(t, found)
	assert.Equal(t, "ok", actual)
}

func TestTraceIDFromGUID(t *testing.T) {

	emptyTrace := pcommon.NewTraceIDEmpty()

	guidBytes, err := hex.DecodeString("607964b641a54e24a5dbdb7aab3b9b34")
	require.NoError(t, err)
	var goodTraceID pcommon.TraceID
	copy(goodTraceID[:], guidBytes[0:16])

	assert.Equal(t, emptyTrace, TraceIDFromGUID(nil))

	tests := map[string]pcommon.TraceID{
		"":                                          emptyTrace,
		"invalid":                                   emptyTrace,
		"607964b641a54e24a5dbdb7aab3b":              emptyTrace, // too short
		"607964b641a54e24a5dbdb7aab3b9b34":          goodTraceID,
		"607964b6-41a5-4e24-a5db-db7aab3b9b34":      goodTraceID,
		"607964b6-41A5-4E24-A5DB-DB7AAB3B9B34":      goodTraceID,
		"OK///607964b6-41a5-4e24-a5db-db7aab3b9b34": goodTraceID,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, TraceIDFromGUID(&input))
		})
	}
}

func TestJsonNumberToRaw(t *testing.T) {
	assert.Equal(t, float64(1.34), JsonNumberToRaw("1.34"))
	assert.Equal(t, int64(1024), JsonNumberToRaw("1024"))
}

func TestReplaceJsonNumber(t *testing.T) {
	input := map[string]interface{}{
		"one": json.Number("3.14"),
		"two": []interface{}{json.Number("1"), json.Number("2"), json.Number("3.1"), json.Number("4.1")},
		"three": map[string]interface{}{
			"s1": "ok",
			"s2": json.Number("1"),
			"s3": true,
		},
		"four": []interface{}{"ok", json.Number("3.14"), false},
	}

	expected := map[string]interface{}{
		"one": float64(3.14),
		"two": []interface{}{int64(1), int64(2), float64(3.1), float64(4.1)},
		"three": map[string]interface{}{
			"s1": "ok",
			"s2": int64(1),
			"s3": true,
		},
		"four": []interface{}{"ok", float64(3.14), false},
	}

	if diff := deep.Equal(expected, ReplaceJsonNumber(input)); diff != nil {
		t.Errorf("FAIL\n%s\n", strings.Join(diff, "\n"))
	}
}

func TestExtractRawAttributes(t *testing.T) {
	badDuration := "invalid"
	goodDuration := "1234"

	tenantID := "tenant.id"
	operationVersion := "operation.version"
	resultType := "result.type"
	resultSignature := "result.signature"
	resultDescription := "result.description"
	callerIPAddress := "127.0.0.1"
	correlationID := "edb70d1a-eec2-4b4c-b2f4-60e3510160ee"
	level := "Informational"
	location := "location"

	var identity interface{}
	identity = "someone"

	var properties interface{}
	properties = map[string]interface{}{
		"a": uint64(1),
		"b": true,
		"c": 1.23,
		"d": "ok",
	}

	tests := []struct {
		name     string
		log      AzureLogRecord
		expected map[string]interface{}
	}{
		{
			name: "minimal",
			log: AzureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]interface{}{
				"azure.resource.id":    "resource.id",
				"azure.operation.name": "operation.name",
				"azure.category":       "category",
				"cloud.provider":       "azure",
			},
		},
		{
			name: "bad-duration",
			log: AzureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]interface{}{
				"azure.resource.id":    "resource.id",
				"azure.operation.name": "operation.name",
				"azure.category":       "category",
				"cloud.provider":       "azure",
			},
		},
		{
			name: "everything",
			log: AzureLogRecord{
				Time:              "",
				ResourceID:        "resource.id",
				TenantID:          &tenantID,
				OperationName:     "operation.name",
				OperationVersion:  &operationVersion,
				Category:          "category",
				ResultType:        &resultType,
				ResultSignature:   &resultSignature,
				ResultDescription: &resultDescription,
				DurationMs:        &goodDuration,
				CallerIPAddress:   &callerIPAddress,
				CorrelationID:     &correlationID,
				Identity:          &identity,
				Level:             &level,
				Location:          &location,
				Properties:        &properties,
			},
			expected: map[string]interface{}{
				"azure.resource.id":        "resource.id",
				"azure.tenant.id":          "tenant.id",
				"azure.operation.name":     "operation.name",
				"azure.operation.version":  "operation.version",
				"azure.category":           "category",
				"azure.result.type":        "result.type",
				"azure.result.signature":   "result.signature",
				"azure.result.description": "result.description",
				"azure.duration":           int64(1234),
				"net.sock.peer.addr":       "127.0.0.1",
				"azure.identity":           "someone",
				"cloud.region":             "location",
				"cloud.provider":           "azure",
				"azure.properties":         properties,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ExtractRawAttributes(tt.log))
		})
	}

}

var minimumLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := AsTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.Attributes().PutStr("azure.resource.id", "/RESOURCE_ID")
	lr.Attributes().PutStr("azure.operation.name", "SecretGet")
	lr.Attributes().PutStr("azure.category", "AuditEvent")
	lr.Attributes().PutStr("cloud.provider", "azure")
	lr.Attributes().Sort()
	return lr
}()

var maximumLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := AsTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("Warning")
	guid := "607964b6-41a5-4e24-a5db-db7aab3b9b34"
	lr.SetTraceID(TraceIDFromGUID(&guid))

	lr.Attributes().PutStr("azure.resource.id", "/RESOURCE_ID")
	lr.Attributes().PutStr("azure.tenant.id", "/TENANT_ID")
	lr.Attributes().PutStr("azure.operation.name", "SecretGet")
	lr.Attributes().PutStr("azure.operation.version", "7.0")
	lr.Attributes().PutStr("azure.category", "AuditEvent")
	lr.Attributes().PutStr("azure.result.type", "Success")
	lr.Attributes().PutStr("azure.result.signature", "Signature")
	lr.Attributes().PutStr("azure.result.description", "Description")
	lr.Attributes().PutInt("azure.duration", 1234)
	lr.Attributes().PutStr("net.sock.peer.addr", "127.0.0.1")
	lr.Attributes().PutStr("cloud.region", "ukso")
	lr.Attributes().PutStr("cloud.provider", "azure")

	lr.Attributes().PutEmptyMap("azure.identity").PutEmptyMap("claim").PutStr("oid", "607964b6-41a5-4e24-a5db-db7aab3b9b34")
	m := lr.Attributes().PutEmptyMap("azure.properties")
	m.PutStr("string", "string")
	m.PutInt("int", 429)
	m.PutDouble("float", 3.14)
	m.PutBool("bool", false)
	m.Sort()

	lr.Attributes().Sort()

	return lr
}()

// sortLogAttributes is a utility function that will sort
// all the resource and logRecord attributes to allow
// reliable comparison in the tests
func sortLogAttributes(logs plog.Logs) plog.Logs {
	for r := 0; r < logs.ResourceLogs().Len(); r++ {
		rl := logs.ResourceLogs().At(r)
		sortAttributes(rl.Resource().Attributes())
		for s := 0; s < rl.ScopeLogs().Len(); s++ {
			sl := rl.ScopeLogs().At(s)
			for l := 0; l < sl.LogRecords().Len(); l++ {
				sortAttributes(sl.LogRecords().At(l).Attributes())
			}
		}
	}
	return logs
}

// sortAttributes will sort the keys in the given map and
// then recursively sort any child maps. The function will
// traverse both maps and slices.
func sortAttributes(attrs pcommon.Map) {
	attrs.Range(func(k string, v pcommon.Value) bool {
		if v.Type() == pcommon.ValueTypeMap {
			sortAttributes(v.Map())
		} else if v.Type() == pcommon.ValueTypeSlice {
			s := v.Slice()
			for i := 0; i < s.Len(); i++ {
				value := s.At(i)
				if value.Type() == pcommon.ValueTypeMap {
					sortAttributes(value.Map())
				}
			}
		}
		return true
	})
	attrs.Sort()
}

func TestDecodeAzureLogRecord(t *testing.T) {

	expectedMinimum := plog.NewLogs()
	lr := expectedMinimum.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	minimumLogRecord.CopyTo(lr)

	expectedMinimum2 := plog.NewLogs()
	logRecords := expectedMinimum2.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)

	expectedMaximum := plog.NewLogs()
	lr = expectedMaximum.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	maximumLogRecord.CopyTo(lr)

	tests := []struct {
		file     string
		expected plog.Logs
	}{
		{
			file:     "log-minimum.json",
			expected: expectedMinimum,
		},
		{
			file:     "log-minimum-2.json",
			expected: expectedMinimum2,
		},
		{
			file:     "log-maximum.json",
			expected: expectedMaximum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			assert.NoError(t, err)
			assert.NotNil(t, data)

			logs, err := Transform(data)
			assert.NoError(t, err)

			deep.CompareUnexportedFields = true
			if diff := deep.Equal(tt.expected, sortLogAttributes(*logs)); diff != nil {
				t.Errorf("FAIL\n%s\n", strings.Join(diff, "\n"))
			}
			deep.CompareUnexportedFields = false
		})
	}
}
