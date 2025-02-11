// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog/compression"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	require.Equal(t, TypeStr, unmarshaler.Type())
}

func TestUnmarshal(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	testCases := map[string]struct {
		filename          string
		wantResourceCount int
		wantLogCount      int
		wantErr           error
	}{
		"WithMultipleRecords": {
			filename:          "multiple_records",
			wantResourceCount: 1,
			wantLogCount:      2,
		},
		"WithSingleRecord": {
			filename:          "single_record",
			wantResourceCount: 1,
			wantLogCount:      1,
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			wantErr:  errInvalidRecords,
		},
		"WithSomeInvalidRecords": {
			filename:          "some_invalid_records",
			wantResourceCount: 1,
			wantLogCount:      2,
		},
		"WithMultipleResources": {
			filename:          "multiple_resources",
			wantResourceCount: 3,
			wantLogCount:      6,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join(".", "testdata", testCase.filename))
			require.NoError(t, err)

			compressedRecord, err := compression.Zip(record)
			require.NoError(t, err)
			records := [][]byte{compressedRecord}

			got, err := unmarshaler.Unmarshal(records)
			if testCase.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, testCase.wantErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, testCase.wantResourceCount, got.ResourceLogs().Len())
				gotLogCount := 0
				for i := 0; i < got.ResourceLogs().Len(); i++ {
					rm := got.ResourceLogs().At(i)
					require.Equal(t, 1, rm.ScopeLogs().Len())
					ilm := rm.ScopeLogs().At(0)
					gotLogCount += ilm.LogRecords().Len()
				}
				require.Equal(t, testCase.wantLogCount, gotLogCount)
			}
		})
	}
}

func TestLogTimestamp(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	record, err := os.ReadFile(filepath.Join(".", "testdata", "single_record"))
	require.NoError(t, err)

	compressedRecord, err := compression.Zip(record)
	require.NoError(t, err)
	records := [][]byte{compressedRecord}

	got, err := unmarshaler.Unmarshal(records)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 1, got.ResourceLogs().Len())

	rm := got.ResourceLogs().At(0)
	require.Equal(t, 1, rm.ScopeLogs().Len())
	ilm := rm.ScopeLogs().At(0)
	ilm.LogRecords().At(0).Timestamp()
	expectedTimestamp := "2024-09-05 13:47:15.523 +0000 UTC"
	require.Equal(t, expectedTimestamp, ilm.LogRecords().At(0).Timestamp().String())
}
