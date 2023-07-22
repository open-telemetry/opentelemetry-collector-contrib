// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestGenerateMetricDatas(t *testing.T) {
	mds, err := GenerateMetrics("testdata/generated_pict_pairs_metrics.txt")
	require.NoError(t, err)
	require.Equal(t, 25, len(mds))
}

func TestPICTtoCfg(t *testing.T) {
	tests := []struct {
		name   string
		inputs PICTMetricInputs
		cfg    MetricsCfg
	}{
		{
			name: "none",
			inputs: PICTMetricInputs{
				NumResourceAttrs: AttrsNone,
				NumPtsPerMetric:  NumPtsPerMetricOne,
				MetricType:       MetricTypeIntGauge,
				NumPtLabels:      LabelsNone,
			},
			cfg: MetricsCfg{
				NumResourceAttrs:     0,
				NumPtsPerMetric:      1,
				MetricDescriptorType: pmetric.MetricTypeGauge,
				MetricValueType:      pmetric.NumberDataPointValueTypeInt,
				NumPtLabels:          0,
			},
		},
		{
			name: "one",
			inputs: PICTMetricInputs{
				NumResourceAttrs: AttrsOne,
				NumPtsPerMetric:  NumPtsPerMetricOne,
				MetricType:       MetricTypeDoubleGauge,
				NumPtLabels:      LabelsOne,
			},
			cfg: MetricsCfg{
				NumResourceAttrs:     1,
				NumPtsPerMetric:      1,
				MetricDescriptorType: pmetric.MetricTypeGauge,
				MetricValueType:      pmetric.NumberDataPointValueTypeDouble,
				NumPtLabels:          1,
			},
		},
		{
			name: "many",
			inputs: PICTMetricInputs{
				NumResourceAttrs: AttrsTwo,
				NumPtsPerMetric:  NumPtsPerMetricMany,
				MetricType:       MetricTypeDoubleExemplarsHistogram,
				NumPtLabels:      LabelsMany,
			},
			cfg: MetricsCfg{
				NumResourceAttrs:     2,
				NumPtsPerMetric:      16,
				MetricDescriptorType: pmetric.MetricTypeHistogram,
				NumPtLabels:          16,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := pictToCfg(test.inputs)
			expected := test.cfg
			assert.Equal(t, expected.NumResourceAttrs, actual.NumResourceAttrs)
			assert.Equal(t, expected.NumPtsPerMetric, actual.NumPtsPerMetric)
			assert.Equal(t, expected.MetricDescriptorType, actual.MetricDescriptorType)
			assert.Equal(t, expected.MetricValueType, actual.MetricValueType)
			assert.Equal(t, expected.NumPtLabels, actual.NumPtLabels)
		})
	}
}
