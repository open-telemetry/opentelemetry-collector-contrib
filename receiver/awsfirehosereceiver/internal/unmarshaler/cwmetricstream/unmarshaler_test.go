// Copyright The OpenTelemetry Authors
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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	require.Equal(t, TypeStr, unmarshaler.Type())
}

func TestUnmarshal(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	testCases := map[string]struct {
		filename           string
		wantResourceCount  int
		wantMetricCount    int
		wantDatapointCount int
		wantErr            error
	}{
		"WithMultipleRecords": {
			filename:           "multiple_records",
			wantResourceCount:  6,
			wantMetricCount:    33,
			wantDatapointCount: 127,
		},
		"WithSingleRecord": {
			filename:           "single_record",
			wantResourceCount:  1,
			wantMetricCount:    1,
			wantDatapointCount: 1,
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			wantErr:  errInvalidRecords,
		},
		"WithSomeInvalidRecords": {
			filename:           "some_invalid_records",
			wantResourceCount:  5,
			wantMetricCount:    35,
			wantDatapointCount: 88,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join(".", "testdata", testCase.filename))
			require.NoError(t, err)

			records := [][]byte{record}

			got, err := unmarshaler.Unmarshal(records)
			if testCase.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, testCase.wantErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, testCase.wantResourceCount, got.ResourceMetrics().Len())
				gotMetricCount := 0
				gotDatapointCount := 0
				for i := 0; i < got.ResourceMetrics().Len(); i++ {
					rm := got.ResourceMetrics().At(i)
					require.Equal(t, 1, rm.ScopeMetrics().Len())
					ilm := rm.ScopeMetrics().At(0)
					gotMetricCount += ilm.Metrics().Len()
					for j := 0; j < ilm.Metrics().Len(); j++ {
						metric := ilm.Metrics().At(j)
						gotDatapointCount += metric.Summary().DataPoints().Len()
					}
				}
				require.Equal(t, testCase.wantMetricCount, gotMetricCount)
				require.Equal(t, testCase.wantDatapointCount, gotDatapointCount)
			}
		})
	}
}
