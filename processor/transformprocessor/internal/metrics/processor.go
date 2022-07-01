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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type Processor struct {
	queries []common.Query
	logger  *zap.Logger
}

func NewProcessor(statements []string, functions map[string]interface{}, settings component.ProcessorCreateSettings) (*Processor, error) {
	queries, err := common.ParseQueries(statements, functions, ParsePath)
	if err != nil {
		return nil, err
	}
	return &Processor{
		queries: queries,
		logger:  settings.Logger,
	}, nil
}

func (p *Processor) ProcessMetrics(_ context.Context, td pmetric.Metrics) (pmetric.Metrics, error) {
	ctx := metricTransformContext{}
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		ctx.resource = rmetrics.Resource()
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			ctx.il = smetrics.Scope()
			metrics := smetrics.Metrics()
			ctx.metrics = metrics
			for k := 0; k < metrics.Len(); k++ {
				ctx.metric = metrics.At(k)
				switch ctx.metric.DataType() {
				case pmetric.MetricDataTypeSum:
					p.handleNumberDataPoints(ctx, ctx.metric.Sum().DataPoints())
				case pmetric.MetricDataTypeGauge:
					p.handleNumberDataPoints(ctx, ctx.metric.Gauge().DataPoints())
				case pmetric.MetricDataTypeHistogram:
					p.handleHistogramDataPoints(ctx, ctx.metric.Histogram().DataPoints())
				case pmetric.MetricDataTypeExponentialHistogram:
					p.handleExponetialHistogramDataPoints(ctx, ctx.metric.ExponentialHistogram().DataPoints())
				case pmetric.MetricDataTypeSummary:
					p.handleSummaryDataPoints(ctx, ctx.metric.Summary().DataPoints())
				}
			}
		}
	}
	return td, nil
}

func (p *Processor) handleNumberDataPoints(ctx metricTransformContext, dps pmetric.NumberDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.dataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleHistogramDataPoints(ctx metricTransformContext, dps pmetric.HistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.dataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleExponetialHistogramDataPoints(ctx metricTransformContext, dps pmetric.ExponentialHistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.dataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleSummaryDataPoints(ctx metricTransformContext, dps pmetric.SummaryDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		ctx.dataPoint = dps.At(i)
		p.callFunctions(ctx)
	}
}

func (p *Processor) callFunctions(ctx metricTransformContext) {
	for _, statement := range p.queries {
		if statement.Condition(ctx) {
			statement.Function(ctx)
		}
	}
}
