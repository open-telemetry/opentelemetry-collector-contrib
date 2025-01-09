// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

			got, err := unmarshaler.UnmarshalMetrics(records)
			require.Equal(t, testCase.wantErr, err)
			require.NotNil(t, got)
			require.Equal(t, testCase.wantResourceCount, got.ResourceMetrics().Len())
			require.Equal(t, testCase.wantMetricCount, got.MetricCount())
			require.Equal(t, testCase.wantDatapointCount, got.DataPointCount())
		})
	}
}
