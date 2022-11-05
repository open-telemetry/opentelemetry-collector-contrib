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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

type MetricsContext interface {
	ProcessMetrics(ctx context.Context, td pmetric.Metrics) error
}

var _ MetricsContext = &metricStatements{}

type metricStatements []*ottl.Statement[ottlmetric.TransformContext]

func (m metricStatements) ProcessMetrics(ctx context.Context, td pmetric.Metrics) error {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				tCtx := ottlmetric.NewTransformContext(metrics.At(k), smetrics.Scope(), rmetrics.Resource())
				for _, statement := range m {
					_, _, err := statement.Execute(ctx, tCtx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

var _ MetricsContext = &dataPointStatements{}

type dataPointStatements []*ottl.Statement[ottldatapoints.TransformContext]

func (d dataPointStatements) ProcessMetrics(ctx context.Context, td pmetric.Metrics) error {
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
					err = d.handleNumberDataPoints(ctx, metric.Sum().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeGauge:
					err = d.handleNumberDataPoints(ctx, metric.Gauge().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeHistogram:
					err = d.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeExponentialHistogram:
					err = d.handleExponetialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeSummary:
					err = d.handleSummaryDataPoints(ctx, metric.Summary().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d dataPointStatements) handleNumberDataPoints(ctx context.Context, dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) handleHistogramDataPoints(ctx context.Context, dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) handleExponetialHistogramDataPoints(ctx context.Context, dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) handleSummaryDataPoints(ctx context.Context, dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) callFunctions(ctx context.Context, tCtx ottldatapoints.TransformContext) error {
	for _, statement := range d {
		_, _, err := statement.Execute(ctx, tCtx)
		if err != nil {
			return err
		}
	}
	return nil
}

type MetricParserCollection struct {
	parserCollection
	metricParser    ottl.Parser[ottlmetric.TransformContext]
	dataPointParser ottl.Parser[ottldatapoints.TransformContext]
}

func NewMetricParserCollection(functions map[string]interface{}, settings component.TelemetrySettings) MetricParserCollection {
	return MetricParserCollection{
		parserCollection: parserCollection{
			settings:       settings,
			resourceParser: ottlresource.NewParser(ResourceFunctions(), settings),
			scopeParser:    ottlscope.NewParser(ScopeFunctions(), settings),
		},
		metricParser:    ottlmetric.NewParser(functions, settings),
		dataPointParser: ottldatapoints.NewParser(functions, settings),
	}
}

func (pc MetricParserCollection) ParseContextStatements(contextStatements []ContextStatements) ([]MetricsContext, error) {
	contexts := make([]MetricsContext, len(contextStatements))
	var errors error

	for i, s := range contextStatements {
		switch s.Context {
		case Metric:
			tStatements, err := pc.metricParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = metricStatements(tStatements)
		case DataPoint:
			seStatements, err := pc.dataPointParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = dataPointStatements(seStatements)
		default:
			statements, err := pc.parseCommonContextStatements(s)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = statements
		}
	}
	if errors != nil {
		return nil, errors
	}
	return contexts, nil
}
