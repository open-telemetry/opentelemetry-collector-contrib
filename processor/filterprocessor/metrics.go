// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

type filterMetricProcessor struct {
	cfg                 *Config
	skipResourceExpr    expr.BoolExpr[ottlresource.TransformContext]
	skipMetricExpr      expr.BoolExpr[ottlmetric.TransformContext]
	logger              *zap.Logger
	metricConditions    []*ottl.Statement[ottlmetric.TransformContext]
	dataPointConditions []*ottl.Statement[ottldatapoint.TransformContext]
}

func newFilterMetricProcessor(logger *zap.Logger, cfg *Config) (*filterMetricProcessor, error) {
	if cfg.Metrics.MetricConditions != nil || cfg.Metrics.DataPointConditions != nil {
		fsp := &filterMetricProcessor{
			cfg:    cfg,
			logger: logger,
		}

		if cfg.Metrics.MetricConditions != nil {
			metricp := ottlmetric.NewParser(common.Functions[ottlmetric.TransformContext](), component.TelemetrySettings{Logger: zap.NewNop()})
			statements, err := metricp.ParseStatements(common.PrepareConditionForParsing(cfg.Metrics.MetricConditions))
			if err != nil {
				return nil, err
			}
			fsp.metricConditions = statements
		}

		if cfg.Metrics.DataPointConditions != nil {
			datapointp := ottldatapoint.NewParser(common.Functions[ottldatapoint.TransformContext](), component.TelemetrySettings{Logger: zap.NewNop()})
			statements, err := datapointp.ParseStatements(common.PrepareConditionForParsing(cfg.Metrics.DataPointConditions))
			if err != nil {
				return nil, err
			}
			fsp.dataPointConditions = statements
		}

		return fsp, nil
	}

	skipResourceExpr, err := newSkipResExpr(cfg.Metrics.Include, cfg.Metrics.Exclude)
	if err != nil {
		return nil, err
	}

	skipMetricExpr, err := filtermetric.NewSkipExpr(cfg.Metrics.Include, cfg.Metrics.Exclude)
	if err != nil {
		return nil, err
	}

	includeMatchType := ""
	var includeExpressions []string
	var includeMetricNames []string
	var includeResourceAttributes []filterconfig.Attribute
	if cfg.Metrics.Include != nil {
		includeMatchType = string(cfg.Metrics.Include.MatchType)
		includeExpressions = cfg.Metrics.Include.Expressions
		includeMetricNames = cfg.Metrics.Include.MetricNames
		includeResourceAttributes = cfg.Metrics.Include.ResourceAttributes
	}

	excludeMatchType := ""
	var excludeExpressions []string
	var excludeMetricNames []string
	var excludeResourceAttributes []filterconfig.Attribute
	if cfg.Metrics.Exclude != nil {
		excludeMatchType = string(cfg.Metrics.Exclude.MatchType)
		excludeExpressions = cfg.Metrics.Exclude.Expressions
		excludeMetricNames = cfg.Metrics.Exclude.MetricNames
		excludeResourceAttributes = cfg.Metrics.Exclude.ResourceAttributes
	}

	logger.Info(
		"Metric filter configured",
		zap.String("include match_type", includeMatchType),
		zap.Strings("include expressions", includeExpressions),
		zap.Strings("include metric names", includeMetricNames),
		zap.Any("include metrics with resource attributes", includeResourceAttributes),
		zap.String("exclude match_type", excludeMatchType),
		zap.Strings("exclude expressions", excludeExpressions),
		zap.Strings("exclude metric names", excludeMetricNames),
		zap.Any("exclude metrics with resource attributes", excludeResourceAttributes),
	)

	return &filterMetricProcessor{
		cfg:              cfg,
		skipResourceExpr: skipResourceExpr,
		skipMetricExpr:   skipMetricExpr,
		logger:           logger,
	}, nil
}

// processMetrics filters the given metrics based off the filterMetricProcessor's filters.
func (fmp *filterMetricProcessor) processMetrics(ctx context.Context, pdm pmetric.Metrics) (pmetric.Metrics, error) {
	filteringMetrics := fmp.metricConditions != nil
	filteringDataPoints := fmp.dataPointConditions != nil

	if filteringMetrics || filteringDataPoints {
		var errors error
		pdm.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
			rmetrics.ScopeMetrics().RemoveIf(func(smetrics pmetric.ScopeMetrics) bool {
				smetrics.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
					if filteringMetrics {
						tCtx := ottlmetric.NewTransformContext(metric, smetrics.Scope(), rmetrics.Resource())
						metCondition, err := common.CheckConditions(ctx, tCtx, fmp.metricConditions)
						if err != nil {
							errors = multierr.Append(errors, err)
						}
						if metCondition {
							return true
						}
					}
					if filteringDataPoints {
						switch metric.Type() {
						case pmetric.MetricTypeSum:
							err := fmp.handleNumberDataPoints(ctx, metric.Sum().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
							if err != nil {
								errors = multierr.Append(errors, err)
							}
							return metric.Sum().DataPoints().Len() == 0
						case pmetric.MetricTypeGauge:
							err := fmp.handleNumberDataPoints(ctx, metric.Gauge().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
							if err != nil {
								errors = multierr.Append(errors, err)
							}
							return metric.Gauge().DataPoints().Len() == 0
						case pmetric.MetricTypeHistogram:
							err := fmp.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
							if err != nil {
								errors = multierr.Append(errors, err)
							}
							return metric.Histogram().DataPoints().Len() == 0
						case pmetric.MetricTypeExponentialHistogram:
							err := fmp.handleExponetialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
							if err != nil {
								errors = multierr.Append(errors, err)
							}
							return metric.ExponentialHistogram().DataPoints().Len() == 0
						case pmetric.MetricTypeSummary:
							err := fmp.handleSummaryDataPoints(ctx, metric.Summary().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource())
							if err != nil {
								errors = multierr.Append(errors, err)
							}
							return metric.Summary().DataPoints().Len() == 0
						default:
							return false
						}
					}
					return false
				})
				return smetrics.Metrics().Len() == 0
			})
			return rmetrics.ScopeMetrics().Len() == 0
		})

		if errors != nil {
			return pdm, errors
		}
		if pdm.ResourceMetrics().Len() == 0 {
			return pdm, processorhelper.ErrSkipProcessingData
		}
		return pdm, nil
	}

	var errors error
	pdm.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		resource := rm.Resource()
		skip, err := fmp.skipResourceExpr.Eval(ctx, ottlresource.NewTransformContext(resource))
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		if skip {
			return true
		}

		rm.ScopeMetrics().RemoveIf(func(ilm pmetric.ScopeMetrics) bool {
			scope := ilm.Scope()
			ilm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				skip, err = fmp.skipMetricExpr.Eval(ctx, ottlmetric.NewTransformContext(m, scope, resource))
				if err != nil {
					errors = multierr.Append(errors, err)
					return false
				}
				return skip
			})
			// Filter out empty ScopeMetrics
			return ilm.Metrics().Len() == 0
		})
		// Filter out empty ResourceMetrics
		return rm.ScopeMetrics().Len() == 0
	})
	if pdm.ResourceMetrics().Len() == 0 {
		return pdm, processorhelper.ErrSkipProcessingData
	}
	if errors != nil {
		return pdm, errors
	}
	return pdm, nil
}

func newSkipResExpr(include *filtermetric.MatchProperties, exclude *filtermetric.MatchProperties) (expr.BoolExpr[ottlresource.TransformContext], error) {
	var matchers []expr.BoolExpr[ottlresource.TransformContext]
	inclExpr, err := newResExpr(include)
	if err != nil {
		return nil, err
	}
	if inclExpr != nil {
		matchers = append(matchers, expr.Not(inclExpr))
	}
	exclExpr, err := newResExpr(exclude)
	if err != nil {
		return nil, err
	}
	if exclExpr != nil {
		matchers = append(matchers, exclExpr)
	}
	return expr.Or(matchers...), nil
}

type resExpr filtermatcher.AttributesMatcher

func (r resExpr) Eval(_ context.Context, tCtx ottlresource.TransformContext) (bool, error) {
	return filtermatcher.AttributesMatcher(r).Match(tCtx.GetResource().Attributes()), nil
}

func newResExpr(mp *filtermetric.MatchProperties) (expr.BoolExpr[ottlresource.TransformContext], error) {
	if mp == nil {
		return nil, nil
	}
	attributeMatcher, err := filtermatcher.NewAttributesMatcher(
		filterset.Config{
			MatchType:    filterset.MatchType(mp.MatchType),
			RegexpConfig: mp.RegexpConfig,
		},
		mp.ResourceAttributes,
	)
	if err != nil {
		return nil, err
	}
	if attributeMatcher == nil {
		return nil, err
	}
	return resExpr(attributeMatcher), nil
}

func (fmp *filterMetricProcessor) handleNumberDataPoints(ctx context.Context, dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		metCondition, err := common.CheckConditions(ctx, tCtx, fmp.dataPointConditions)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return metCondition
	})
	return errors
}

func (fmp *filterMetricProcessor) handleHistogramDataPoints(ctx context.Context, dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.HistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		metCondition, err := common.CheckConditions(ctx, tCtx, fmp.dataPointConditions)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return metCondition
	})
	return errors
}

func (fmp *filterMetricProcessor) handleExponetialHistogramDataPoints(ctx context.Context, dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.ExponentialHistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		metCondition, err := common.CheckConditions(ctx, tCtx, fmp.dataPointConditions)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return metCondition
	})
	return errors
}

func (fmp *filterMetricProcessor) handleSummaryDataPoints(ctx context.Context, dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.SummaryDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource)
		metCondition, err := common.CheckConditions(ctx, tCtx, fmp.dataPointConditions)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return metCondition
	})
	return errors
}
