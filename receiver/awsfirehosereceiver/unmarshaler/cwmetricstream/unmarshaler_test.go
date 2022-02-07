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

package cwmetricstream

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestUnmarshal(t *testing.T) {
	unmarshaler := &Unmarshaler{}

	tests := []struct {
		filename            string
		expectedMetricCount int
		expectedError       error
	}{
		{
			filename:            "multiple_records",
			expectedMetricCount: 33,
		},
		{
			filename:            "single_record",
			expectedMetricCount: 1,
		},
		{
			filename:      "invalid_records",
			expectedError: errInvalidRecords,
		},
		{
			filename:            "some_invalid_records",
			expectedMetricCount: 35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			record, err := os.ReadFile(path.Join(".", "testdata", tt.filename))
			require.NoError(t, err)

			md, err := unmarshaler.Unmarshal(record, zap.NewNop())
			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, md)
				require.Equal(t, 1, md.ResourceMetrics().Len())
				ilm := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics()
				require.Equal(t, tt.expectedMetricCount, ilm.Len())
			}
		})
	}
}
