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

package metadataparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	labelName       = "labelName"
	labelColumnName = "labelColumnName"
)

func TestLabel_ToLabelValueMetadata(t *testing.T) {
	testCases := map[string]struct {
		valueType    string
		expectedType interface{}
		expectError  bool
	}{
		"Value type is string":       {labelValueTypeString, metadata.StringLabelValueMetadata{}, false},
		"Value type is int":          {labelValueTypeInt, metadata.Int64LabelValueMetadata{}, false},
		"Value type is bool":         {labelValueTypeBool, metadata.BoolLabelValueMetadata{}, false},
		"Value type is string slice": {labelValueTypeStringSlice, metadata.StringSliceLabelValueMetadata{}, false},
		"Value type is byte slice":   {labelValueTypeByteSlice, metadata.ByteSliceLabelValueMetadata{}, false},
		"Value type is unknown":      {"unknown", nil, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			label := Label{
				Name:       labelName,
				ColumnName: labelColumnName,
				ValueType:  testCase.valueType,
			}

			valueMetadata, err := label.toLabelValueMetadata()

			if testCase.expectError {
				require.Nil(t, valueMetadata)
				require.Error(t, err)
			} else {
				require.NotNil(t, valueMetadata)
				require.NoError(t, err)

				assert.IsType(t, testCase.expectedType, valueMetadata)
				assert.Equal(t, label.Name, valueMetadata.Name())
				assert.Equal(t, label.ColumnName, valueMetadata.ColumnName())
			}
		})
	}
}
