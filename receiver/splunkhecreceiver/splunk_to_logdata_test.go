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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type testingHecConfiguration struct {
	sourceKey     string
	sourceTypeKey string
	indexKey      string
	hostKey       string
}

func (t *testingHecConfiguration) GetSourceKey() string {
	return t.sourceKey
}

func (t *testingHecConfiguration) GetSourceTypeKey() string {
	return t.sourceTypeKey
}

func (t *testingHecConfiguration) GetIndexKey() string {
	return t.indexKey
}

func (t *testingHecConfiguration) GetHostKey() string {
	return t.hostKey
}

var defaultTestingHecConfig = &testingHecConfiguration{
	sourceKey:     splunk.DefaultSourceLabel,
	sourceTypeKey: splunk.DefaultSourceTypeLabel,
	indexKey:      splunk.DefaultIndexLabel,
	hostKey:       conventions.AttributeHostName,
}

func Test_SplunkHecToLogData(t *testing.T) {

	time := 0.123
	nanoseconds := 123000000

	tests := []struct {
		name      string
		event     splunk.Event
		output    pdata.ResourceLogsSlice
		hecConfig splunk.HECConfiguration
		wantErr   error
	}{
		{
			name: "happy_path",
			event: splunk.Event{
				Time:       &time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      "value",
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() pdata.ResourceLogsSlice {
				return createLogsSlice(nanoseconds)
			}(),
			wantErr: nil,
		},
		{
			name: "double",
			event: splunk.Event{
				Time:       &time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      12.3,
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() pdata.ResourceLogsSlice {
				logsSlice := createLogsSlice(nanoseconds)
				logsSlice.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body().SetDoubleVal(12.3)
				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "array",
			event: splunk.Event{
				Time:       &time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      []interface{}{"foo", "bar"},
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() pdata.ResourceLogsSlice {
				logsSlice := createLogsSlice(nanoseconds)
				arrVal := pdata.NewAttributeValueArray()
				arr := arrVal.ArrayVal()
				arr.AppendEmpty().SetStringVal("foo")
				arr.AppendEmpty().SetStringVal("bar")
				arrVal.CopyTo(logsSlice.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body())
				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "complex_structure",
			event: splunk.Event{
				Time:       &time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      map[string]interface{}{"foos": []interface{}{"foo", "bar", "foobar"}, "bool": false, "someInt": int64(12)},
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() pdata.ResourceLogsSlice {
				logsSlice := createLogsSlice(nanoseconds)
				foosArr := pdata.NewAttributeValueArray()
				foos := foosArr.ArrayVal()
				foos.EnsureCapacity(3)
				foos.AppendEmpty().SetStringVal("foo")
				foos.AppendEmpty().SetStringVal("bar")
				foos.AppendEmpty().SetStringVal("foobar")

				attVal := pdata.NewAttributeValueMap()
				attMap := attVal.MapVal()
				attMap.InsertBool("bool", false)
				attMap.Insert("foos", foosArr)
				attMap.InsertInt("someInt", 12)
				attVal.CopyTo(logsSlice.At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body())
				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "nil_timestamp",
			event: splunk.Event{
				Time:       new(float64),
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      "value",
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() pdata.ResourceLogsSlice {
				return createLogsSlice(0)
			}(),
			wantErr: nil,
		},
		{
			name: "custom_config_mapping",
			event: splunk.Event{
				Time:       new(float64),
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      "value",
				Fields: map[string]interface{}{
					"foo": "bar",
				},
			},
			hecConfig: &testingHecConfiguration{
				sourceKey:     "mysource",
				sourceTypeKey: "mysourcetype",
				indexKey:      "myindex",
				hostKey:       "myhost",
			},
			output: func() pdata.ResourceLogsSlice {
				lrs := pdata.NewResourceLogsSlice()
				lr := lrs.AppendEmpty()
				ill := lr.InstrumentationLibraryLogs().AppendEmpty()
				logRecord := ill.Logs().AppendEmpty()
				logRecord.SetName("mysourcetype")
				logRecord.Body().SetStringVal("value")
				logRecord.SetTimestamp(pdata.Timestamp(0))
				logRecord.Attributes().InsertString("myhost", "localhost")
				logRecord.Attributes().InsertString("mysource", "mysource")
				logRecord.Attributes().InsertString("mysourcetype", "mysourcetype")
				logRecord.Attributes().InsertString("myindex", "myindex")
				logRecord.Attributes().InsertString("foo", "bar")
				return lrs
			}(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := splunkHecToLogData(zap.NewNop(), []*splunk.Event{&tt.event}, func(resource pdata.Resource) {}, tt.hecConfig)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.output.Len(), result.ResourceLogs().Len())
			assert.Equal(t, tt.output.At(0), result.ResourceLogs().At(0))
		})
	}
}

func createLogsSlice(nanoseconds int) pdata.ResourceLogsSlice {
	lrs := pdata.NewResourceLogsSlice()
	lr := lrs.AppendEmpty()
	ill := lr.InstrumentationLibraryLogs().AppendEmpty()
	logRecord := ill.Logs().AppendEmpty()
	logRecord.SetName("mysourcetype")
	logRecord.Body().SetStringVal("value")
	logRecord.SetTimestamp(pdata.Timestamp(nanoseconds))
	logRecord.Attributes().InsertString("host.name", "localhost")
	logRecord.Attributes().InsertString("com.splunk.source", "mysource")
	logRecord.Attributes().InsertString("com.splunk.sourcetype", "mysourcetype")
	logRecord.Attributes().InsertString("com.splunk.index", "myindex")
	logRecord.Attributes().InsertString("foo", "bar")

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
	attMap := atts.MapVal()
	attMap.InsertString("foo", "bar")
	assert.Equal(t, atts, value)
}

func Test_ConvertAttributeValueArray(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), []interface{}{"foo"})
	assert.NoError(t, err)
	arrValue := pdata.NewAttributeValueArray()
	arr := arrValue.ArrayVal()
	arr.AppendEmpty().SetStringVal("foo")
	assert.Equal(t, arrValue, value)
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
