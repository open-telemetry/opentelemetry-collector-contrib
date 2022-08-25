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
	queries    []tql.Query
	dropMetric bool
	logger     *zap.Logger
}

type DropType int32

const (
	DropNone DropType = iota
	DropDataPoint
	DropMetric
)

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

func (p *Processor) ProcessMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	ctx := tqlmetrics.MetricTransformContext{}
	rms := md.ResourceMetrics()
	rms.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		ctx.Resource = rm.Resource()
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			ctx.InstrumentationScope = sm.Scope()
			ctx.Metrics = sm.Metrics()
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				ctx.Metric = m
				p.dropMetric = false
				return p.handleDataPoints(ctx)
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
	return md, nil
}

func (p *Processor) handleDataPoints(ctx tqlmetrics.MetricTransformContext) bool {
	switch ctx.Metric.DataType() {
	case pmetric.MetricDataTypeSum:
		return p.handleNumberDataPoints(ctx, ctx.Metric.Sum().DataPoints())
	case pmetric.MetricDataTypeGauge:
		return p.handleNumberDataPoints(ctx, ctx.Metric.Gauge().DataPoints())
	case pmetric.MetricDataTypeHistogram:
		return p.handleHistogramDataPoints(ctx, ctx.Metric.Histogram().DataPoints())
	case pmetric.MetricDataTypeExponentialHistogram:
		return p.handleExponetialHistogramDataPoints(ctx, ctx.Metric.ExponentialHistogram().DataPoints())
	case pmetric.MetricDataTypeSummary:
		return p.handleSummaryDataPoints(ctx, ctx.Metric.Summary().DataPoints())
	}
	return false
}

func (p *Processor) handleNumberDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.NumberDataPointSlice) bool {
	dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		if p.dropMetric {
			return true
		}
		ctx.DataPoint = dp
		return p.handleDropType(p.callFunctions(ctx))

	})
	return p.dropMetric || dps.Len() == 0
}

func (p *Processor) handleHistogramDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.HistogramDataPointSlice) bool {
	dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
		if p.dropMetric {
			return true
		}
		ctx.DataPoint = dp
		return p.handleDropType(p.callFunctions(ctx))
	})
	return p.dropMetric || dps.Len() == 0
}

func (p *Processor) handleExponetialHistogramDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.ExponentialHistogramDataPointSlice) bool {
	dps.RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
		if p.dropMetric {
			return true
		}
		ctx.DataPoint = dp
		return p.handleDropType(p.callFunctions(ctx))
	})
	return p.dropMetric || dps.Len() == 0
}

func (p *Processor) handleSummaryDataPoints(ctx tqlmetrics.MetricTransformContext, dps pmetric.SummaryDataPointSlice) bool {
	dps.RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
		if p.dropMetric {
			return true
		}
		ctx.DataPoint = dp
		return p.handleDropType(p.callFunctions(ctx))
	})
	return p.dropMetric || dps.Len() == 0
}

func (p *Processor) callFunctions(ctx tqlmetrics.MetricTransformContext) DropType {
	for _, statement := range p.queries {
		if statement.Condition(ctx) {
			output := statement.Function(ctx)
			if val, ok := output.(DropType); ok {
				return val
			}
		}
	}
	return DropNone
}

func (p *Processor) handleDropType(dropType DropType) bool {
	p.dropMetric = dropType == DropMetric
	if p.dropMetric || dropType == DropDataPoint {
		return true
	}
	return false
}
