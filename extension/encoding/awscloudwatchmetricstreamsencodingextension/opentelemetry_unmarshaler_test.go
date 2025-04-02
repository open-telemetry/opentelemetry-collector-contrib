// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

// getRecordFromFiles reads the pmetric.Metrics
// from each given file in metricFiles and returns
// the record in the format the encoding extension
// expects the metrics to be
func getRecordFromFiles(t *testing.T, metricFiles []string) []byte {
	var record []byte
	for _, file := range metricFiles {
		metrics, err := golden.ReadMetrics(file)
		require.NoError(t, err)

		m := pmetric.ProtoMarshaler{}
		data, err := m.MarshalMetrics(metrics)
		require.NoError(t, err)

		buf := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(buf, uint64(len(data)))
		datum := buf[:n]
		datum = append(datum, data...)

		record = append(record, datum...)
	}

	return record
}

func TestUnmarshalOpenTelemetryMetrics(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata/opentelemetry1"
	unmarshaler := formatOpenTelemetry10Unmarshaler{}
	tests := map[string]struct {
		record                  []byte
		expectedMetricsFilename string
		expectedErrStr          string
	}{
		"valid_record_single_metric": {
			record:                  getRecordFromFiles(t, []string{filepath.Join(filesDirectory, "valid_metric.yaml")}),
			expectedMetricsFilename: filepath.Join(filesDirectory, "valid_metric_single_expected.yaml"),
		},
		"valid_record_multiple_metrics": {
			record: getRecordFromFiles(t, []string{
				filepath.Join(filesDirectory, "valid_metric.yaml"),
				filepath.Join(filesDirectory, "valid_metric.yaml"),
			}),
			expectedMetricsFilename: filepath.Join(filesDirectory, "valid_metric_multiple_expected.yaml"),
		},
		"empty_record": {
			record:                  []byte{},
			expectedMetricsFilename: filepath.Join(filesDirectory, "empty_expected.yaml"),
		},
		"invalid_record_no_metrics": {
			record:         []byte{1, 2, 3},
			expectedErrStr: "unable to unmarshal input: proto: ExportMetricsServiceRequest: illegal tag 0 (wire type 2)",
		},
		"record_out_of_bounds": {
			record:         []byte("a"),
			expectedErrStr: "index out of bounds: length prefix exceeds available bytes in record",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := unmarshaler.UnmarshalMetrics(test.record)
			if test.expectedErrStr != "" {
				require.EqualError(t, err, test.expectedErrStr)
				return
			}

			require.NoError(t, err)
			expected, err := golden.ReadMetrics(test.expectedMetricsFilename)
			require.NoError(t, err)
			err = pmetrictest.CompareMetrics(expected, result)
			require.NoError(t, err)
		})
	}
}
