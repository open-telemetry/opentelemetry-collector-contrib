// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpmetricstream

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestNewUnmarshaler(t *testing.T) {
	unmarshaler, err := NewUnmarshaler(context.Background())
	require.NoError(t, err)
	require.NotNil(t, unmarshaler)
}

// getMetricRecord reads the resource metrics inside file
// and adds a uvarint prefix to it, before returning the
// byte array.
func getMetricRecord(t *testing.T, file string) []byte {
	metrics, err := golden.ReadMetrics(file)
	require.NoError(t, err)

	m := pmetric.ProtoMarshaler{}
	data, err := m.MarshalMetrics(metrics)
	require.NoError(t, err)

	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, uint64(len(data)))
	record := buf[:n]
	record = append(record, data...)
	return record
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	unmarshaler, errU := NewUnmarshaler(context.Background())
	require.NoError(t, errU)

	testCases := map[string]struct {
		record                  []byte
		expectedMetricsFilename string
		wantErr                 string
	}{
		"WithSingleRecord": {
			record:                  getMetricRecord(t, "testdata/single_record.yaml"),
			expectedMetricsFilename: "testdata/single_record_expected.yaml",
		},
		"WithMultipleRecords": {
			record:                  getMetricRecord(t, "testdata/multiple_records.yaml"),
			expectedMetricsFilename: "testdata/multiple_records_expected.yaml",
		},
		"WithEmptyRecord": {
			record:                  []byte{},
			expectedMetricsFilename: "testdata/empty_record_expected.yaml",
		},
		"WithInvalidRecord": {
			record:  []byte{1, 2},
			wantErr: "unable to unmarshal input",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := unmarshaler.UnmarshalMetrics(testCase.record)
			if testCase.wantErr != "" {
				require.ErrorContains(t, err, testCase.wantErr)
				return
			}

			require.NoError(t, err)

			expected, err := golden.ReadMetrics(testCase.expectedMetricsFilename)
			require.NoError(t, err)

			err = pmetrictest.CompareMetrics(expected, got)
			require.NoError(t, err)
		})
	}
}
