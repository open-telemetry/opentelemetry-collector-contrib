// Copyright  The OpenTelemetry Authors
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

package podmanreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func Test_EventsToLogs(t *testing.T) {
	nanoseconds := 123000000

	// map for adding the test cases in the future.
	tests := []struct {
		name    string
		event   event
		output  pdata.ResourceLogsSlice
		wantErr error
	}{
		{
			name: "test_event",
			event: event{
				TimeNano: int64(nanoseconds),
				Type:     "container",
				Action:   "start",
				ID:       "4e051e79c60b87930f70ca06a8b621ac82dfe54a06b7f5da61fa4d9d34175685",
				Actor: actor{
					ID: "4e051e79c60b87930f70ca06a8b621ac82dfe54a06b7f5da61fa4d9d34175685",
					Attributes: map[string]string{
						"image": "test_image",
						"name":  "test",
					},
				},
			},
			output: func() pdata.ResourceLogsSlice {
				lrs := pdata.NewResourceLogsSlice()
				lr := lrs.AppendEmpty()
				ill := lr.InstrumentationLibraryLogs().AppendEmpty()
				logRecord := ill.Logs().AppendEmpty()
				logRecord.SetName("start")
				logRecord.SetTimestamp(pdata.Timestamp(nanoseconds))
				logRecord.Attributes().InsertString("container.id", "4e051e79c60b87930f70ca06a8b621ac82dfe54a06b7f5da61fa4d9d34175685")
				body := pdata.NewAttributeValueString("podman container ( test ) start")
				body.CopyTo(logRecord.Body())
				logRecord.Attributes().Insert("container.image.name", pdata.NewAttributeValueString("test_image"))
				logRecord.Attributes().Insert("container.name", pdata.NewAttributeValueString("test"))
				return lrs
			}(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := translateEventsToLogs(zap.NewNop(), tt.event)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.output.Len(), result.ResourceLogs().Len())
			assert.Equal(t, tt.output.At(0), result.ResourceLogs().At(0))
		})
	}
}

func Test_ConvertAttributeValueEmpty(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), nil)
	assert.NoError(t, err)
	assert.Equal(t, pdata.NewAttributeValueEmpty(), value)
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
	arr := arrValue.SliceVal()
	arr.AppendEmpty().SetStringVal("foo")
	assert.Equal(t, arrValue, value)
}

func Test_ConvertAttributeValueInvalid(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), event{})
	assert.Error(t, err)
	assert.Equal(t, pdata.NewAttributeValueEmpty(), value)
}

func Test_ConvertAttributeValueInvalidInMap(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), map[string]interface{}{"foo": event{}})
	assert.Error(t, err)
	assert.Equal(t, pdata.NewAttributeValueEmpty(), value)
}

func Test_ConvertAttributeValueInvalidInArray(t *testing.T) {
	value, err := convertInterfaceToAttributeValue(zap.NewNop(), []interface{}{event{}})
	assert.Error(t, err)
	assert.Equal(t, pdata.NewAttributeValueEmpty(), value)
}
