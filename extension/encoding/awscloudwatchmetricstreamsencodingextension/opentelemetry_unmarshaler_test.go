// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"errors"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

// getMetricsAndRecordFromFile reads the pmetric.Metrics
// from filename and returns them along with their record
// in the format the encoding extension expects them to be
func getMetricsAndRecordFromFile(t *testing.T, filename string) (pmetric.Metrics, []byte) {
	metrics, err := golden.ReadMetrics(filename)
	require.NoError(t, err)

	m := pmetric.ProtoMarshaler{}
	data, err := m.MarshalMetrics(metrics)
	require.NoError(t, err)

	record := proto.EncodeVarint(uint64(len(data)))
	record = append(record, data...)
	return metrics, record
}

func TestUnmarshalOpenTelemetryMetrics(t *testing.T) {
	t.Parallel()

	validMetrics, validRecord := getMetricsAndRecordFromFile(t, "testdata/opentelemetry1/valid_metric.yaml")

	tests := map[string]struct {
		record          []byte
		expectedMetrics pmetric.Metrics
		expectedErr     error
	}{
		"valid_record": {
			record:          validRecord,
			expectedMetrics: validMetrics,
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
			err = pmetrictest.CompareMetrics(test.expectedMetrics, result)
			require.NoError(t, err)
		})
	}
}
