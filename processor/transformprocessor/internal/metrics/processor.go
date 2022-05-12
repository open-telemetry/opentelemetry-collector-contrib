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
			il := rmetrics.ScopeMetrics().At(j).Scope()
			ctx.il = il
			metrics := rmetrics.ScopeMetrics().At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				ctx.metric = metric
				switch ctx.metric.DataType() {
				case pmetric.MetricDataTypeSum:
					p.handleNumberDataPoints(ctx, metric.Sum().DataPoints())
				case pmetric.MetricDataTypeGauge:
					p.handleNumberDataPoints(ctx, metric.Gauge().DataPoints())
				case pmetric.MetricDataTypeHistogram:
					p.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints())
				case pmetric.MetricDataTypeExponentialHistogram:
					p.handleExponetialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints())
				case pmetric.MetricDataTypeSummary:
					p.handleSummaryDataPoints(ctx, metric.Summary().DataPoints())
				}
			}
		}
	}
	return td, nil
}

func (p *Processor) handleNumberDataPoints(ctx metricTransformContext, dps pmetric.NumberDataPointSlice) {
	for l := 0; l < dps.Len(); l++ {
		ctx.dataPoint = dps.At(l)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleHistogramDataPoints(ctx metricTransformContext, dps pmetric.HistogramDataPointSlice) {
	for l := 0; l < dps.Len(); l++ {
		ctx.dataPoint = dps.At(l)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleExponetialHistogramDataPoints(ctx metricTransformContext, dps pmetric.ExponentialHistogramDataPointSlice) {
	for l := 0; l < dps.Len(); l++ {
		ctx.dataPoint = dps.At(l)
		p.callFunctions(ctx)
	}
}

func (p *Processor) handleSummaryDataPoints(ctx metricTransformContext, dps pmetric.SummaryDataPointSlice) {
	for l := 0; l < dps.Len(); l++ {
		ctx.dataPoint = dps.At(l)
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
