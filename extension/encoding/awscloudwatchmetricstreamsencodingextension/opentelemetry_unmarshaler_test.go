// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

// getRecordFromFile reads the pmetric.Metrics
// from filename and returns the record in the
// format the encoding extension expects the
// metrics to be
func getRecordFromFile(t *testing.T, filename string) []byte {
	metrics, err := golden.ReadMetrics(filename)
	require.NoError(t, err)

	m := pmetric.ProtoMarshaler{}
	data, err := m.MarshalMetrics(metrics)
	require.NoError(t, err)

	record := proto.EncodeVarint(uint64(len(data)))
	record = append(record, data...)
	return record
}

func TestUnmarshalOpenTelemetryMetrics(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		record                  []byte
		expectedMetricsFilename string
		expectedErr             error
	}{
		"valid_record": {
			record:                  getRecordFromFile(t, "testdata/opentelemetry1/valid_metric.yaml"),
			expectedMetricsFilename: "testdata/opentelemetry1/valid_metric_expected.yaml",
		},
		"invalid_record_empty": {
			record:      []byte{},
			expectedErr: errUvarintReadFailure,
		},
		"invalid_record_no_metrics": {
			record:      []byte{1, 2, 3},
			expectedErr: errors.New("proto: MetricsData: illegal tag 0 (wire type 2)"),
		},
	}

	unmarshaler := formatOpenTelemetry10Unmarshaler{protoUnmarshaler: &pmetric.ProtoUnmarshaler{}}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := unmarshaler.UnmarshalMetrics(test.record)
			if test.expectedErr != nil {
				require.Error(t, err)
				require.EqualError(t, test.expectedErr, err.Error())
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
