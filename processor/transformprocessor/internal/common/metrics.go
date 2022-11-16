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
	"go.opentelemetry.io/collector/pdata/pcommon"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

var _ consumer.Metrics = &metricStatements{}

type metricStatements []*ottl.Statement[ottlmetric.TransformContext]

func (m metricStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (m metricStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errors error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		rmetrics.ScopeMetrics().RemoveIf(func(smetrics pmetric.ScopeMetrics) bool {
			smetrics.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
				tCtx := ottlmetric.NewTransformContext(metric, smetrics.Scope(), rmetrics.Resource())
				remove, err := executeStatements(ctx, tCtx, m)
				if err != nil {
					errors = multierr.Append(errors, err)
					return false
				}
				return bool(remove)
			})
			return smetrics.Metrics().Len() == 0
		})
		return rmetrics.ScopeMetrics().Len() == 0
	})
	return errors
}

var _ consumer.Metrics = &dataPointStatements{}

type dataPointStatements []*ottl.Statement[ottldatapoint.TransformContext]

func (d dataPointStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (d dataPointStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errors error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		rmetrics.ScopeMetrics().RemoveIf(func(smetrics pmetric.ScopeMetrics) bool {
			smetrics.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					err := d.handleNumberDataPoints(ctx, metric.Sum().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
					if err != nil {
						errors = multierr.Append(errors, err)
					}
					return metric.Sum().DataPoints().Len() == 0
				case pmetric.MetricTypeGauge:
					err := d.handleNumberDataPoints(ctx, metric.Gauge().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
					if err != nil {
						errors = multierr.Append(errors, err)
					}
					return metric.Gauge().DataPoints().Len() == 0
				case pmetric.MetricTypeHistogram:
					err := d.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
					if err != nil {
						errors = multierr.Append(errors, err)
					}
					return metric.Histogram().DataPoints().Len() == 0
				case pmetric.MetricTypeExponentialHistogram:
					err := d.handleExponetialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
					if err != nil {
						errors = multierr.Append(errors, err)
					}
					return metric.ExponentialHistogram().DataPoints().Len() == 0
				case pmetric.MetricTypeSummary:
					err := d.handleSummaryDataPoints(ctx, metric.Summary().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
					if err != nil {
						errors = multierr.Append(errors, err)
					}
					return metric.Summary().DataPoints().Len() == 0
				default:
					return false
				}
			})
			return smetrics.Metrics().Len() == 0
		})
		return rmetrics.ScopeMetrics().Len() == 0
	})
	return errors
}

func (d dataPointStatements) handleNumberDataPoints(ctx context.Context, dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		remove, err := executeStatements(ctx, tCtx, d)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return bool(remove)
	})
	return errors
}

func (d dataPointStatements) handleHistogramDataPoints(ctx context.Context, dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.HistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		remove, err := executeStatements(ctx, tCtx, d)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return bool(remove)
	})
	return errors
}

func (d dataPointStatements) handleExponetialHistogramDataPoints(ctx context.Context, dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.ExponentialHistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		remove, err := executeStatements(ctx, tCtx, d)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return bool(remove)
	})
	return errors
}

func (d dataPointStatements) handleSummaryDataPoints(ctx context.Context, dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.SummaryDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		remove, err := executeStatements(ctx, tCtx, d)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return bool(remove)
	})
	return errors
}

type MetricParserCollection struct {
	parserCollection
	metricParser    ottl.Parser[ottlmetric.TransformContext]
	dataPointParser ottl.Parser[ottldatapoint.TransformContext]
}

type MetricParserCollectionOption func(*MetricParserCollection) error

func WithMetricParser(functions map[string]interface{}) MetricParserCollectionOption {
	return func(mp *MetricParserCollection) error {
		mp.metricParser = ottlmetric.NewParser(functions, mp.settings)
		return nil
	}
}

func WithDataPointParser(functions map[string]interface{}) MetricParserCollectionOption {
	return func(mp *MetricParserCollection) error {
		mp.dataPointParser = ottldatapoint.NewParser(functions, mp.settings)
		return nil
	}
}

func NewMetricParserCollection(settings component.TelemetrySettings, options ...MetricParserCollectionOption) (*MetricParserCollection, error) {
	mpc := &MetricParserCollection{
		parserCollection: parserCollection{
			settings:       settings,
			resourceParser: ottlresource.NewParser(ResourceFunctions(), settings),
			scopeParser:    ottlscope.NewParser(ScopeFunctions(), settings),
		},
	}

	for _, op := range options {
		err := op(mpc)
		if err != nil {
			return nil, err
		}
	}

	return mpc, nil
}

func (pc MetricParserCollection) ParseContextStatements(contextStatements ContextStatements) (consumer.Metrics, error) {
	switch contextStatements.Context {
	case Metric:
		mStatements, err := pc.metricParser.ParseStatements(contextStatements.Statements)
		if err != nil {
			return nil, err
		}
		return metricStatements(mStatements), nil
	case DataPoint:
		dpStatements, err := pc.dataPointParser.ParseStatements(contextStatements.Statements)
		if err != nil {
			return nil, err
		}
		return dataPointStatements(dpStatements), nil
	default:
		statements, err := pc.parseCommonContextStatements(contextStatements)
		if err != nil {
			return nil, err
		}
		return statements, nil
	}
}
