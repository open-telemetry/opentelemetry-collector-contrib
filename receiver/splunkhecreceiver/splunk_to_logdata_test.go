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

package splunkhecreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func Test_SplunkHecToLogData(t *testing.T) {

	tests := []struct {
		name    string
		event   splunk.Event
		output  pdata.ResourceLogsSlice
		wantErr error
	}{
		{
			name: "happy_path",
			event: splunk.Event{
				Time:       0.123,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      "value",
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			output: func() pdata.ResourceLogsSlice {
				return createLogsSlice("value")
			}(),
			wantErr: nil,
		},
		{
			name: "double",
			event: splunk.Event{
				Time:       0.123,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      12.3,
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			output: func() pdata.ResourceLogsSlice {
				logsSlice := createLogsSlice("value")
				logsSlice.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body().SetDoubleVal(12.3)
				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "array",
			event: splunk.Event{
				Time:       0.123,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      []interface{}{"foo", "bar"},
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			output: func() pdata.ResourceLogsSlice {
				logsSlice := createLogsSlice("value")
				arr := pdata.NewAnyValueArray()
				arr.Append(pdata.NewAttributeValueString("foo"))
				arr.Append(pdata.NewAttributeValueString("bar"))
				logsSlice.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body().SetArrayVal(arr)
				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "complex_structure",
			event: splunk.Event{
				Time:       0.123,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      map[string]interface{}{"foos": []interface{}{"foo", "bar", "foobar"}, "bool": false, "someInt": int64(12)},
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			output: func() pdata.ResourceLogsSlice {
				logsSlice := createLogsSlice("value")
				attMap := pdata.NewAttributeMap()
				foos := pdata.NewAnyValueArray()
				foos.Append(pdata.NewAttributeValueString("foo"))
				foos.Append(pdata.NewAttributeValueString("bar"))
				foos.Append(pdata.NewAttributeValueString("foobar"))
				foosArr := pdata.NewAttributeValueArray()
				foosArr.SetArrayVal(foos)
				attMap.InsertBool("bool", false)
				attMap.Insert("foos", foosArr)
				attMap.InsertInt("someInt", 12)
				logsSlice.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body().SetMapVal(attMap)
				return logsSlice
			}(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SplunkHecToLogData(zap.NewNop(), []*splunk.Event{&tt.event}, func(resource pdata.Resource) {})
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.output.Len(), result.ResourceLogs().Len())
			assert.Equal(t, tt.output.At(0), result.ResourceLogs().At(0))
		})
	}
}

func createLogsSlice(body string) pdata.ResourceLogsSlice {
	lrs := pdata.NewResourceLogsSlice()
	lrs.Resize(1)
	lr := lrs.At(0)
	lr.InitEmpty()
	lr.Resource().InitEmpty()
	logRecord := pdata.NewLogRecord()
	logRecord.InitEmpty()

	logRecord.SetName("mysourcetype")
	logRecord.Body().SetStringVal(body)
	logRecord.SetTimestamp(pdata.TimestampUnixNano(123000000))
	lr.Resource().Attributes().InsertString("host.hostname", "localhost")
	lr.Resource().Attributes().InsertString("service.name", "mysource")
	lr.Resource().Attributes().InsertString("com.splunk.sourcetype", "mysourcetype")
	logRecord.Attributes().InsertString("foo", "bar")
	ill := pdata.NewInstrumentationLibraryLogs()
	ill.InitEmpty()
	ill.Logs().Append(logRecord)
	lr.InstrumentationLibraryLogs().Append(ill)

	return lrs
}

func Test_ConvertAttributeValueNull(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), nil)
	assert.NoError(t, err)
	assert.Equal(t, pdata.NewAttributeValueNull(), value)
}

func Test_ConvertAttributeValueString(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), "foo")
	assert.NoError(t, err)
	assert.Equal(t, pdata.NewAttributeValueString("foo"), value)
}

func Test_ConvertAttributeValueBool(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), false)
	assert.NoError(t, err)
	assert.Equal(t, pdata.NewAttributeValueBool(false), value)
}

func Test_ConvertAttributeValueFloat(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), 12.3)
	assert.NoError(t, err)
	assert.Equal(t, pdata.NewAttributeValueDouble(12.3), value)
}

func Test_ConvertAttributeValueMap(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), map[string]interface{}{"foo": "bar"})
	assert.NoError(t, err)
	atts := pdata.NewAttributeValueMap()
	attMap := pdata.NewAttributeMap()
	attMap.InsertString("foo", "bar")
	atts.SetMapVal(attMap)
	assert.Equal(t, atts, value)
}

func Test_ConvertAttributeValueArray(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), []interface{}{"foo"})
	assert.NoError(t, err)
	arr := pdata.NewAttributeValueArray()
	arrValue := pdata.NewAnyValueArray()
	arrValue.Append(pdata.NewAttributeValueString("foo"))
	arr.SetArrayVal(arrValue)
	assert.Equal(t, arr, value)
}

func Test_ConvertAttributeValueInvalid(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), splunk.Event{})
	assert.Error(t, err)
	assert.Equal(t, pdata.NewAttributeValueNull(), value)
}

func Test_ConvertAttributeValueInvalidInMap(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), map[string]interface{}{"foo": splunk.Event{}})
	assert.Error(t, err)
	assert.Equal(t, pdata.NewAttributeValueNull(), value)
}

func Test_ConvertAttributeValueInvalidInArray(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), []interface{}{splunk.Event{}})
	assert.Error(t, err)
	assert.Equal(t, pdata.NewAttributeValueNull(), value)
}
