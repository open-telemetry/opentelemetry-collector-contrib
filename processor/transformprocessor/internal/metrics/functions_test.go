// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func Test_DataPointFunctions(t *testing.T) {
	type testCase struct {
		name        string
		flagEnabled bool
	}

	tests := []testCase{
		{
			name:        "ConvertBetweenSumAndGaugeMetricContextEnabled enabled",
			flagEnabled: true,
		},
		{
			name:        "ConvertBetweenSumAndGaugeMetricContextEnabled disabled",
			flagEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := ottlfuncs.StandardFuncs[ottldatapoint.TransformContext]()
			expected["convert_summary_sum_val_to_sum"] = newConvertSummarySumValToSumFactory()
			expected["convert_summary_count_val_to_sum"] = newConvertSummaryCountValToSumFactory()

			actual := DataPointFunctions()

			require.Len(t, actual, len(expected))
			for k := range actual {
				assert.Contains(t, expected, k)
			}
		},
		)
	}
}

func Test_MetricFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[ottlmetric.TransformContext]()
	expected["convert_sum_to_gauge"] = newConvertSumToGaugeFactory()
	expected["convert_gauge_to_sum"] = newConvertGaugeToSumFactory()
	expected["aggregate_on_attributes"] = newAggregateOnAttributesFactory()
	expected["aggregate_on_attribute_value"] = newAggregateOnAttributeValueFactory()
	expected["extract_sum_metric"] = newExtractSumMetricFactory()
	expected["extract_count_metric"] = newExtractCountMetricFactory()
	expected["copy_metric"] = newCopyMetricFactory()
	expected["scale_metric"] = newScaleMetricFactory()
	expected["convert_exponential_histogram_to_histogram"] = newconvertExponentialHistToExplicitHistFactory()

	actual := MetricFunctions()
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
