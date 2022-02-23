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

package googlecloudexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

const invalidUTF8 = "\xa0\xa1"

func TestToProto(t *testing.T) {
	testCases := []struct {
		name          string
		value         interface{}
		expectedValue *structpb.Value
		expectedErr   string
	}{
		{
			name:          "nil value",
			value:         nil,
			expectedValue: structpb.NewNullValue(),
		},
		{
			name: "valid numbers",
			value: map[string]interface{}{
				"int":     int(1),
				"int8":    int8(1),
				"int16":   int16(1),
				"int32":   int32(1),
				"int64":   int64(1),
				"uint":    uint(1),
				"uint8":   uint8(1),
				"uint16":  uint16(1),
				"uint32":  uint32(1),
				"uint64":  uint64(1),
				"float32": float32(1),
				"float64": float64(1),
			},
			expectedValue: structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"int":     structpb.NewNumberValue(float64(1)),
					"int8":    structpb.NewNumberValue(float64(1)),
					"int16":   structpb.NewNumberValue(float64(1)),
					"int32":   structpb.NewNumberValue(float64(1)),
					"int64":   structpb.NewNumberValue(float64(1)),
					"uint":    structpb.NewNumberValue(float64(1)),
					"uint8":   structpb.NewNumberValue(float64(1)),
					"uint16":  structpb.NewNumberValue(float64(1)),
					"uint32":  structpb.NewNumberValue(float64(1)),
					"uint64":  structpb.NewNumberValue(float64(1)),
					"float32": structpb.NewNumberValue(float64(1)),
					"float64": structpb.NewNumberValue(float64(1)),
				},
			}),
		},
		{
			name: "valid map[string]interface{}",
			value: map[string]interface{}{
				"test": "value",
			},
			expectedValue: structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"test": structpb.NewStringValue("value"),
				},
			}),
		},
		{
			name: "map[string]interface with invalid key",
			value: map[string]interface{}{
				invalidUTF8: "value",
			},
			expectedErr: "invalid UTF-8 in key",
		},
		{
			name: "map[string]interface with invalid value",
			value: map[string]interface{}{
				"test_key": make(chan int),
			},
			expectedErr: "failed to convert value for key test_key to proto",
		},
		{
			name: "valid map[string]string",
			value: map[string]string{
				"test": "value",
			},
			expectedValue: structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"test": structpb.NewStringValue("value"),
				},
			}),
		},
		{
			name: "map[string]string with invalid key",
			value: map[string]string{
				invalidUTF8: "value",
			},
			expectedErr: "invalid UTF-8 in key",
		},
		{
			name: "map[string]string with invalid value",
			value: map[string]string{
				"test_key": invalidUTF8,
			},
			expectedErr: "failed to convert value for key test_key to proto",
		},
		{
			name: "valid map[string]map[string]string",
			value: map[string]map[string]string{
				"test": {
					"key": "value",
				},
			},
			expectedValue: structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"test": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"key": structpb.NewStringValue("value"),
						},
					}),
				},
			}),
		},
		{
			name: "map[string]map[string]string with invalid key",
			value: map[string]map[string]string{
				invalidUTF8: {
					"key": "value",
				},
			},
			expectedErr: "invalid UTF-8 in key",
		},
		{
			name: "map[string]map[string]string with invalid value",
			value: map[string]map[string]string{
				"test_key": {
					"key": invalidUTF8,
				},
			},
			expectedErr: "failed to convert embedded value for key test_key to proto",
		},
		{
			name:  "valid list",
			value: []interface{}{"value", 1},
			expectedValue: structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{
				structpb.NewStringValue("value"),
				structpb.NewNumberValue(float64(1)),
			}}),
		},
		{
			name:        "list with invalid value",
			value:       []interface{}{make(chan int)},
			expectedErr: "failed to convert index 0 of list to proto",
		},
		{
			name:  "valid string list",
			value: []string{"value", "test"},
			expectedValue: structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{
				structpb.NewStringValue("value"),
				structpb.NewStringValue("test"),
			}}),
		},
		{
			name:        "string list with invalid utf-8",
			value:       []string{invalidUTF8},
			expectedErr: "failed to convert index 0 to proto",
		},
		{
			name:          "valid byte array",
			value:         []byte("test"),
			expectedValue: structpb.NewStringValue("dGVzdA=="),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proto, err := convertToProto(tc.value)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedValue, proto)
		})
	}
}
