// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestUnmarshalOpenTelemetryMetrics(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata/opentelemetry1"
	unmarshaler := formatOpenTelemetry10Unmarshaler{}
	tests := map[string]struct {
		record                  []byte
		expectedMetricsFilename string
		expectedErrRegexp       string
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
			record:            []byte{1, 2, 3},
			expectedErrRegexp: "unable to unmarshal input: proto: .*",
		},
		"record_out_of_bounds": {
			record:            []byte("a"),
			expectedErrRegexp: "unable to read OTLP metric message",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := unmarshaler.UnmarshalMetrics(test.record)
			if test.expectedErrRegexp != "" {
				require.Error(t, err)
				require.Regexp(t, test.expectedErrRegexp, err.Error())
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

func TestNewMetricsDecoder_otel(t *testing.T) {
	directory := "testdata/opentelemetry1/stream"
	expectPattern := "valid_metric_expect_%d.yaml"

	tests := []struct {
		name   string
		offset int64
		index  int
	}{
		{
			name:   "Normal streaming",
			offset: 0,
			index:  0,
		},
		{
			name:   "Stream with offset",
			offset: 325, // skip first record
			index:  1,   // start from first index
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// read and combine 3 inputs
			input := getRecordFromFiles(t, []string{
				filepath.Join(directory, "valid_metric_1.yaml"),
				filepath.Join(directory, "valid_metric_2.yaml"),
				filepath.Join(directory, "valid_metric_3.yaml"),
			})

			otelUnmarshaler := &formatOpenTelemetry10Unmarshaler{buildInfo: component.BuildInfo{}}

			// flush every item & start from offset defined in test case.
			streamer, err := otelUnmarshaler.NewMetricsDecoder(bytes.NewReader(input), encoding.WithFlushItems(1), encoding.WithOffset(tt.offset))
			require.NoError(t, err)

			index := tt.index
			for {
				index++

				var metrics pmetric.Metrics
				metrics, err = streamer.DecodeMetrics()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					t.Errorf("failed to unmarshal log for index %d: %v", index, err)
				}

				// To check or update offset, uncomment offset below
				// fmt.Println(streamer.Offset())
				var expectedMetrics pmetric.Metrics
				expectedMetrics, err = golden.ReadMetrics(filepath.Join(directory, fmt.Sprintf(expectPattern, index)))
				require.NoError(t, err)
				require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, metrics, pmetrictest.IgnoreMetricsOrder()))
			}

			// expect EOF after all logs are read
			_, err = streamer.DecodeMetrics()
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

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
