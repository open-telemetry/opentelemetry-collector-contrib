// Copyright  The OpenTelemetry Authors
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

package metadataparser

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

func TestParseMetadataConfig(t *testing.T) {
	testCases := map[string]struct {
		filePath    string
		expectError bool
	}{
		"Valid metadata":     {"../../testdata/metadata_valid.yaml", false},
		"YAML parsing error": {"../../testdata/metadata_not_yaml.yaml", true},
		"Invalid metadata":   {"../../testdata/metadata_invalid.yaml", true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(testCase.filePath)

			require.NoError(t, err)

			metadataSlice, err := ParseMetadataConfig(content)

			if testCase.expectError {
				require.Error(t, err)
				require.Nil(t, metadataSlice)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 2, len(metadataSlice))

				mData := metadataSlice[0]

				assert.NotNil(t, mData)
				assertMetricsMetadata(t, "current stats", mData)

				mData = metadataSlice[1]

				assert.NotNil(t, mData)
				assertMetricsMetadata(t, "interval stats", mData)
			}
		})
	}
}

func assertMetricsMetadata(t *testing.T, expectedName string, metricsMetadata *metadata.MetricsMetadata) {
	assert.Equal(t, expectedName, metricsMetadata.Name)
	assert.Equal(t, "query", metricsMetadata.Query)
	assert.Equal(t, "metric_name_prefix", metricsMetadata.MetricNamePrefix)

	assert.Equal(t, 1, len(metricsMetadata.QueryLabelValuesMetadata))
	assert.Equal(t, "label_name", metricsMetadata.QueryLabelValuesMetadata[0].Name())
	assert.Equal(t, "LABEL_NAME", metricsMetadata.QueryLabelValuesMetadata[0].ColumnName())
	assert.IsType(t, metadata.StringLabelValueMetadata{}, metricsMetadata.QueryLabelValuesMetadata[0])

	assert.Equal(t, 1, len(metricsMetadata.QueryMetricValuesMetadata))
	assert.Equal(t, "metric_name", metricsMetadata.QueryMetricValuesMetadata[0].Name())
	assert.Equal(t, "METRIC_NAME", metricsMetadata.QueryMetricValuesMetadata[0].ColumnName())
	assert.Equal(t, "metric_unit", metricsMetadata.QueryMetricValuesMetadata[0].Unit())
	assert.Equal(t, pdata.MetricDataTypeGauge, metricsMetadata.QueryMetricValuesMetadata[0].DataType().MetricDataType())
	assert.IsType(t, metadata.Int64MetricValueMetadata{}, metricsMetadata.QueryMetricValuesMetadata[0])
}
