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
	"encoding/json"
	"github.com/go-test/deep"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestAsTimestamp(t *testing.T) {
	timestamp := "2022-11-11T04:48:27.6767145Z"
	nanos, err := asTimestamp(timestamp)
	assert.NoError(t, err)
	assert.Less(t, pcommon.Timestamp(0), nanos)

	timestamp = "invalid-time"
	nanos, err = asTimestamp(timestamp)
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
			assert.Equal(t, expected, asSeverity(input))
		})
	}
}

func TestSetIf(t *testing.T) {
	m := map[string]interface{}{}

	setIf(m, "key", nil)
	actual, found := m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v := ""
	setIf(m, "key", &v)
	actual, found = m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v = "ok"
	setIf(m, "key", &v)
	actual, found = m["key"]
	assert.True(t, found)
	assert.Equal(t, "ok", actual)
}

func TestJsonNumberToRaw(t *testing.T) {
	assert.Equal(t, float64(1.34), jsonNumberToRaw("1.34"))
	assert.Equal(t, int64(1024), jsonNumberToRaw("1024"))
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

	if diff := deep.Equal(expected, replaceJsonNumber(input)); diff != nil {
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
		log      azureLogRecord
		expected map[string]interface{}
	}{
		{
			name: "minimal",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]interface{}{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				cloudProvider:      "azure",
			},
		},
		{
			name: "bad-duration",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]interface{}{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				cloudProvider:      "azure",
			},
		},
		{
			name: "everything",
			log: azureLogRecord{
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
				azureTenantID:          "tenant.id",
				azureOperationName:     "operation.name",
				azureOperationVersion:  "operation.version",
				azureCategory:          "category",
				azureCorrelationID:     correlationID,
				azureResultType:        "result.type",
				azureResultSignature:   "result.signature",
				azureResultDescription: "result.description",
				azureDuration:          int64(1234),
				netSockPeerAddr:        "127.0.0.1",
				azureIdentity:          "someone",
				cloudRegion:            "location",
				cloudProvider:          "azure",
				azureProperties:        properties,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractRawAttributes(tt.log))
		})
	}

}

var minimumLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.Attributes().PutStr(azureOperationName, "SecretGet")
	lr.Attributes().PutStr(azureCategory, "AuditEvent")
	lr.Attributes().PutStr(cloudProvider, "azure")
	lr.Attributes().Sort()
	return lr
}()

var maximumLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("Warning")
	guid := "607964b6-41a5-4e24-a5db-db7aab3b9b34"

	lr.Attributes().PutStr(azureTenantID, "/TENANT_ID")
	lr.Attributes().PutStr(azureOperationName, "SecretGet")
	lr.Attributes().PutStr(azureOperationVersion, "7.0")
	lr.Attributes().PutStr(azureCategory, "AuditEvent")
	lr.Attributes().PutStr(azureCorrelationID, guid)
	lr.Attributes().PutStr(azureResultType, "Success")
	lr.Attributes().PutStr(azureResultSignature, "Signature")
	lr.Attributes().PutStr(azureResultDescription, "Description")
	lr.Attributes().PutInt(azureDuration, 1234)
	lr.Attributes().PutStr(netSockPeerAddr, "127.0.0.1")
	lr.Attributes().PutStr(cloudRegion, "ukso")
	lr.Attributes().PutStr(cloudProvider, "azure")

	lr.Attributes().PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", "607964b6-41a5-4e24-a5db-db7aab3b9b34")
	m := lr.Attributes().PutEmptyMap(azureProperties)
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
	resourceLogs := expectedMinimum.ResourceLogs().AppendEmpty()
	lr := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	minimumLogRecord.CopyTo(lr)

	expectedMinimum2 := plog.NewLogs()
	resourceLogs = expectedMinimum2.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	logRecords := resourceLogs.ScopeLogs().AppendEmpty().LogRecords()
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)

	expectedMaximum := plog.NewLogs()
	resourceLogs = expectedMaximum.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	lr = resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
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

			logs, err := transform(data)
			assert.NoError(t, err)

			deep.CompareUnexportedFields = true
			if diff := deep.Equal(tt.expected, sortLogAttributes(*logs)); diff != nil {
				t.Errorf("FAIL\n%s\n", strings.Join(diff, "\n"))
			}
			deep.CompareUnexportedFields = false
		})
	}
}
