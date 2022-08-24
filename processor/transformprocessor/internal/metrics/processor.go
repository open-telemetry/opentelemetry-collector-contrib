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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqlmetrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

type Processor struct {
	queries []tql.Query
	logger  *zap.Logger
}

func NewProcessor(statements []string, functions map[string]interface{}, settings component.ProcessorCreateSettings) (*Processor, error) {
	queries, err := tql.ParseQueries(statements, functions, tqlmetrics.ParsePath, tqlmetrics.ParseEnum)
	if err != nil {
		return nil, err
	}
	return &Processor{
		queries: queries,
		logger:  settings.Logger,
	}, nil
}

func (p *Processor) ProcessMetrics(_ context.Context, td pmetric.Metrics) (pmetric.Metrics, error) {
	ctx := tqlmetrics.MetricTransformContext{}
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		ctx.Resource = rmetrics.Resource()
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			ctx.InstrumentationScope = smetrics.Scope()
			metrics := smetrics.Metrics()
			ctx.Metrics = metrics
			for k := 0; k < metrics.Len(); k++ {
				ctx.Metric = metrics.At(k)
				switch ctx.Metric.DataType() {
				case pmetric.MetricDataTypeSum:
					p.handleNumberDataPoints(ctx, ctx.Metric.Sum().DataPoints())
				case pmetric.MetricDataTypeGauge:
					p.handleNumberDataPoints(ctx, ctx.Metric.Gauge().DataPoints())
				case pmetric.MetricDataTypeHistogram:
					p.handleHistogramDataPoints(ctx, ctx.Metric.Histogram().DataPoints())
				case pmetric.MetricDataTypeExponentialHistogram:
					p.handleExponetialHistogramDataPoints(ctx, ctx.Metric.ExponentialHistogram().DataPoints())
				case pmetric.MetricDataTypeSummary:
					p.handleSummaryDataPoints(ctx, ctx.Metric.Summary().DataPoints())
				}
			}
		}
	}
	return td, nil
}

func (p *Processor) handleNumberDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.NumberDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.DataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleHistogramDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.HistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.DataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleExponetialHistogramDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.ExponentialHistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.DataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleSummaryDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.SummaryDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.DataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) callFunctions(ctx tqlmetrics.MetricTransformContext) {
	for _, statement := range p.queries {
		if statement.Condition(ctx) {
			statement.Function(ctx)
		}
	}
}
