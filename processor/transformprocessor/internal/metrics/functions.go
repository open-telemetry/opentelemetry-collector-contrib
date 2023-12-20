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

var useConvertBetweenSumAndGaugeMetricContext = featuregate.GlobalRegistry().MustRegister(
	"processor.transform.ConvertBetweenSumAndGaugeMetricContext",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled will use metric context for conversion between sum and gauge"),
)

func DataPointFunctions() map[string]ottl.Factory[ottldatapoint.TransformContext] {
	functions := ottlfuncs.StandardFuncs[ottldatapoint.TransformContext]()

	datapointFunctions := ottl.CreateFactoryMap[ottldatapoint.TransformContext](
		newConvertSummarySumValToSumFactory(),
		newConvertSummaryCountValToSumFactory(),
	)

	if !useConvertBetweenSumAndGaugeMetricContext.IsEnabled() {
		for _, f := range []ottl.Factory[ottldatapoint.TransformContext]{
			newConvertDatapointSumToGaugeFactory(),
			newConvertDatapointGaugeToSumFactory(),
		} {
			datapointFunctions[f.Name()] = f
		}
	}

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
	)

	if useConvertBetweenSumAndGaugeMetricContext.IsEnabled() {
		for _, f := range []ottl.Factory[ottlmetric.TransformContext]{
			newConvertSumToGaugeFactory(),
			newConvertGaugeToSumFactory(),
		} {
			metricFunctions[f.Name()] = f
		}
	}

	for k, v := range metricFunctions {
		functions[k] = v
	}

	return functions
}
