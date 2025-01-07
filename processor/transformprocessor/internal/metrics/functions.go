// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

var UseConvertBetweenSumAndGaugeMetricContext = featuregate.GlobalRegistry().MustRegister(
	"processor.transform.ConvertBetweenSumAndGaugeMetricContext",
	featuregate.StageStable,
	featuregate.WithRegisterDescription("When enabled will use metric context for conversion between sum and gauge"),
	featuregate.WithRegisterToVersion("v0.114.0"),
)

func DataPointFunctions() map[string]ottl.Factory[ottldatapoint.TransformContext] {
	functions := ottlfuncs.StandardFuncs[ottldatapoint.TransformContext]()

	datapointFunctions := ottl.CreateFactoryMap[ottldatapoint.TransformContext](
		newConvertSummarySumValToSumFactory(),
		newConvertSummaryCountValToSumFactory(),
	)

	for k, v := range datapointFunctions {
		functions[k] = v
	}

	return functions
}

func MetricFunctions() map[string]ottl.Factory[ottlmetric.TransformContext] {
	functions := ottlfuncs.StandardFuncs[ottlmetric.TransformContext]()

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
	)

	for k, v := range metricFunctions {
		functions[k] = v
	}

	return functions
}
