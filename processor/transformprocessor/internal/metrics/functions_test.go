// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_DataPointFunctions(t *testing.T) {
	expected := common.Functions[ottldatapoint.TransformContext]()
	expected["convert_sum_to_gauge"] = newConvertSumToGaugeFactory()
	expected["convert_gauge_to_sum"] = newConvertGaugeToSumFactory()
	expected["convert_summary_sum_val_to_sum"] = newConvertSummarySumValToSumFactory()
	expected["convert_summary_count_val_to_sum"] = newConvertSummaryCountValToSumFactory()

	actual := DataPointFunctions()

	require.Equal(t, len(expected), len(actual))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}

func Test_MetricFunctions(t *testing.T) {
	expected := common.Functions[ottlmetric.TransformContext]()
	actual := MetricFunctions()
	require.Equal(t, len(expected), len(actual))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
