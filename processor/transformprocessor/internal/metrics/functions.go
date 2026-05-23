// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"maps"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func DataPointFunctions() map[string]ottl.Factory[*ottldatapoint.TransformContext] {
	functions := ottlfuncs.StandardFuncs[*ottldatapoint.TransformContext]()

	datapointFunctions := ottl.CreateFactoryMap(
		newConvertSummarySumValToSumFactory(),
		newConvertSummaryCountValToSumFactory(),
		newMergeHistogramBucketsFactory(),
	)

	maps.Copy(functions, datapointFunctions)

	return functions
}

func MetricFunctions() map[string]ottl.Factory[*ottlmetric.TransformContext] {
	functions := ottlfuncs.StandardFuncs[*ottlmetric.TransformContext]()

	metricFunctions := ottl.CreateFactoryMap(
		newExtractSumMetricFactory(),
		newExtractCountMetricFactory(),
		newConvertGaugeToSumFactory(),
		newConvertSumToGaugeFactory(),
		newCopyMetricFactory(),
		newScaleMetricFactory(),
		newAggregateOnAttributesFactory(),
		newconvertExponentialHistToExplicitHistFactory(),
		newAggregateOnAttributeValueFactory(),
		newConvertSummaryQuantileValToGaugeFactory(),
	)

	maps.Copy(functions, metricFunctions)

	return functions
}
