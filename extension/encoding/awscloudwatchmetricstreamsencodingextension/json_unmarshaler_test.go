// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestUnmarshalJSONMetrics(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata/json"
	tests := map[string]struct {
		metricFilename         string
		metricExpectedFilename string
		expectedErr            error
	}{
		"ValidRecordSingleMetric": {
			// test a record with a single metric
			metricFilename:         "valid_record.json",
			metricExpectedFilename: "valid_record_expected.json",
		},
		"InvalidRecord": {
			// test a record with one invalid metric
			metricFilename: "invalid_record.json",
			expectedErr:    errors.New("0 metrics were extracted from the record"),
		},
		"ValidRecordSomeInvalidMetrics": {
			// test a record with multiple
			// metrics: some invalid, some
			// valid
			metricFilename:         "multiple_metrics.json",
			metricExpectedFilename: "multiple_metrics_expected.json",
		},
	}

	unmarshalerCW := &formatJSONUnmarshaler{component.BuildInfo{}, zap.NewNop()}
	unmarshallerJSONMetric := pmetric.JSONUnmarshaler{}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filesDirectory + "/" + test.metricFilename)
			require.NoError(t, err)

			// a new line represents a new record, so we parse the file
			// content to remove all new lines within each record, and
			// leave only the ones that separate each of them
			// Example input:
			// [
			//     {
			//         "name": "a"
			//     },
			//     {
			//         "name": "b"
			//     }
			// ]
			// Example output:
			// {name: "a"}
			// {name: "b"}
			record := bytes.ReplaceAll(data, []byte("]"), []byte(""))
			record = bytes.ReplaceAll(record, []byte("["), []byte(""))
			record = bytes.ReplaceAll(record, []byte("\n    },"), []byte("}#\n"))
			record = bytes.ReplaceAll(record, []byte("\n"), []byte(""))
			record = bytes.ReplaceAll(record, []byte("#"), []byte("\n"))

			metrics, err := unmarshalerCW.UnmarshalMetrics(record)
			if test.expectedErr != nil {
				require.Equal(t, test.expectedErr, err)
				return
			}

			require.NoError(t, err)

			content, err := os.ReadFile(filesDirectory + "/" + test.metricExpectedFilename)
			require.NoError(t, err)

			expected, err := unmarshallerJSONMetric.UnmarshalMetrics(content)
			require.NoError(t, err)

			require.Equal(t, expected, metrics)
		})
	}
}
