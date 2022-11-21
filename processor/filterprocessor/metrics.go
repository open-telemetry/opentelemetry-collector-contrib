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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

type filterMetricProcessor struct {
	cfg                 *Config
	include             filtermetric.Matcher
	includeAttribute    filtermatcher.AttributesMatcher
	exclude             filtermetric.Matcher
	excludeAttribute    filtermatcher.AttributesMatcher
	logger              *zap.Logger
	checksMetrics       bool
	checksResouces      bool
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

	inc, includeAttr, err := createMatcher(cfg.Metrics.Include)
	if err != nil {
		return nil, err
	}

	exc, excludeAttr, err := createMatcher(cfg.Metrics.Exclude)
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

	checksMetrics := cfg.Metrics.Exclude.ChecksMetrics() || cfg.Metrics.Include.ChecksMetrics()
	checksResouces := cfg.Metrics.Exclude.ChecksResourceAtributes() || cfg.Metrics.Include.ChecksResourceAtributes()

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
		zap.Bool("checksMetrics", checksMetrics),
		zap.Bool("checkResouces", checksResouces),
	)

	return &filterMetricProcessor{
		cfg:              cfg,
		include:          inc,
		includeAttribute: includeAttr,
		exclude:          exc,
		excludeAttribute: excludeAttr,
		logger:           logger,
		checksMetrics:    checksMetrics,
		checksResouces:   checksResouces,
	}, nil
}

func createMatcher(mp *filtermetric.MatchProperties) (filtermetric.Matcher, filtermatcher.AttributesMatcher, error) {
	// Nothing specified in configuration
	if mp == nil {
		return nil, nil, nil
	}
	var attributeMatcher filtermatcher.AttributesMatcher
	attributeMatcher, err := filtermatcher.NewAttributesMatcher(
		filterset.Config{
			MatchType:    filterset.MatchType(mp.MatchType),
			RegexpConfig: mp.RegexpConfig,
		},
		mp.ResourceAttributes,
	)
	if err != nil {
		return nil, attributeMatcher, err
	}

	nameMatcher, err := filtermetric.NewMatcher(mp)
	return nameMatcher, attributeMatcher, err
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

	pdm.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		keepMetricsForResource := fmp.shouldKeepMetricsForResource(rm.Resource())
		if !keepMetricsForResource {
			return true
		}

		if fmp.checksResouces && !fmp.checksMetrics {
			return false
		}

		rm.ScopeMetrics().RemoveIf(func(ilm pmetric.ScopeMetrics) bool {
			ilm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				keep, err := fmp.shouldKeepMetric(m)
				if err != nil {
					fmp.logger.Error("shouldKeepMetric failed", zap.Error(err))
					// don't `return`, keep the metric if there's an error
				}
				return !keep
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
	return pdm, nil
}

func (fmp *filterMetricProcessor) shouldKeepMetric(metric pmetric.Metric) (bool, error) {
	if fmp.include != nil {
		matches, err := fmp.include.MatchMetric(metric)
		if err != nil {
			// default to keep if there's an error
			return true, err
		}
		if !matches {
			return false, nil
		}
	}

	if fmp.exclude != nil {
		matches, err := fmp.exclude.MatchMetric(metric)
		if err != nil {
			return true, err
		}
		if matches {
			return false, nil
		}
	}

	return true, nil
}

func (fmp *filterMetricProcessor) shouldKeepMetricsForResource(resource pcommon.Resource) bool {
	resourceAttributes := resource.Attributes()

	if fmp.include != nil && fmp.includeAttribute != nil {
		matches := fmp.includeAttribute.Match(resourceAttributes)
		if !matches {
			return false
		}
	}

	if fmp.exclude != nil && fmp.excludeAttribute != nil {
		matches := fmp.excludeAttribute.Match(resourceAttributes)
		if matches {
			return false
		}
	}

	return true
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
