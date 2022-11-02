// Copyright The OpenTelemetry Authors
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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
)

type Processor struct {
	statements []*ottl.Statement[ottldatapoints.TransformContext]
}

func NewProcessor(statements []string, settings component.TelemetrySettings) (*Processor, error) {
	ottlp := ottldatapoints.NewParser(Functions(), settings)
	parsedStatements, err := ottlp.ParseStatements(statements)
	if err != nil {
		return nil, err
	}
	return &Processor{
		statements: parsedStatements,
	}, nil
}

func (p *Processor) ProcessMetrics(ctx context.Context, td pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				var err error
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					err = p.handleNumberDataPoints(ctx, metric.Sum().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeGauge:
					err = p.handleNumberDataPoints(ctx, metric.Gauge().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeHistogram:
					err = p.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeExponentialHistogram:
					err = p.handleExponetialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeSummary:
					err = p.handleSummaryDataPoints(ctx, metric.Summary().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				}
				if err != nil {
					return td, err
				}
			}
		}
	}
	return td, nil
}

func (p *Processor) handleNumberDataPoints(ctx context.Context, dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := p.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) handleHistogramDataPoints(ctx context.Context, dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := p.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) handleExponetialHistogramDataPoints(ctx context.Context, dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := p.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) handleSummaryDataPoints(ctx context.Context, dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := p.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) callFunctions(ctx context.Context, tCtx ottldatapoints.TransformContext) error {
	for _, statement := range p.statements {
		_, _, err := statement.Execute(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}
