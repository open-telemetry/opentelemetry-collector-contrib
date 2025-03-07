// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestValidateMetric(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		metric      cloudwatchMetric
		expectedErr error
	}{
		"valid_metric": {
			metric: cloudwatchMetric{
				Namespace: "test/namespace",
				Unit:      "Seconds",
				Value: cloudwatchMetricValue{
					isSet: true,
				},
				MetricName: "test",
			},
		},
		"no_metric_name": {
			metric: cloudwatchMetric{
				Namespace: "test/namespace",
				Unit:      "Seconds",
				Value: cloudwatchMetricValue{
					isSet: true,
				},
			},
			expectedErr: errNoMetricName,
		},
		"no_metric_namespace": {
			metric: cloudwatchMetric{
				Unit: "Seconds",
				Value: cloudwatchMetricValue{
					isSet: true,
				},
				MetricName: "test",
			},
			expectedErr: errNoMetricNamespace,
		},
		"no_metric_unit": {
			metric: cloudwatchMetric{
				Namespace: "test/namespace",
				Value: cloudwatchMetricValue{
					isSet: true,
				},
				MetricName: "test",
			},
			expectedErr: errNoMetricUnit,
		},
		"no_metric_value": {
			metric: cloudwatchMetric{
				Namespace: "test/namespace",
				Unit:      "Seconds",
				Value: cloudwatchMetricValue{
					isSet: false,
				},
				MetricName: "test",
			},
			expectedErr: errNoMetricValue,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateMetric(test.metric)
			require.Equal(t, test.expectedErr, err)
		})
	}
}

// joinMetricsFromFile reads the metrics inside the files,
// and joins them in the format a record expects it to be:
// each metric is expected to be in 1 line, and every new
// line marks a new metric
func joinMetricsFromFile(t *testing.T, dir string, files []string) []byte {
	if len(files) == 0 {
		t.Fatalf("joinMetricsFromFile requires at least one file")
	}
	var buffer bytes.Buffer
	for _, file := range files {
		// get the metric from the files
		data, err := os.ReadFile(filepath.Join(dir, file))
		require.NoError(t, err)

		// remove all insignificant spaces,
		// including new lines
		var compacted bytes.Buffer
		err = json.Compact(&compacted, data)
		require.NoError(t, err)

		// append the metric and add new line
		// to mark the end of this metric
		buffer.Write(compacted.Bytes())
		buffer.WriteByte('\n')
	}
	return buffer.Bytes()
}

func TestUnmarshalJSONMetrics(t *testing.T) {
	t.Parallel()

	filesDirectory := "testdata/json"
	tests := map[string]struct {
		files                  []string
		metricExpectedFilename string
		expectedErr            error
	}{
		"valid_record_single_metric": {
			// test a record with a single metric
			files:                  []string{"valid_metric.json"},
			metricExpectedFilename: "valid_record_single_metric_expected.yaml",
		},
		"invalid_record": {
			// test a record with one invalid metric
			files:       []string{"invalid_metric.json"},
			expectedErr: errEmptyRecord,
		},
		"valid_record_multiple_metrics": {
			// test a record with multiple
			// metrics: some invalid, some
			// valid
			files: []string{
				"valid_metric.json",
				"invalid_metric.json",
				"valid_metric.json",
				"invalid_metric.json",
			},
			metricExpectedFilename: "valid_record_multiple_metrics_expected.yaml",
		},
	}

	unmarshalerCW := &formatJSONUnmarshaler{component.BuildInfo{}, zap.NewNop()}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := joinMetricsFromFile(t, filesDirectory, test.files)

			metrics, err := unmarshalerCW.UnmarshalMetrics(record)
			if test.expectedErr != nil {
				require.Equal(t, test.expectedErr, err)
				return
			}

			expectedMetrics, err := golden.ReadMetrics(filepath.Join(filesDirectory, test.metricExpectedFilename))
			require.NoError(t, err)

			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, metrics))
		})
	}
}
