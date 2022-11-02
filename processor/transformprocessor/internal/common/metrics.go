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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type MetricsContext interface {
	ProcessMetrics(td pmetric.Metrics) error
}

var _ Context = &metricStatements{}
var _ MetricsContext = &metricStatements{}

type metricStatements []*ottl.Statement[ottlmetric.TransformContext]

func (m metricStatements) isContext() {}

func (m metricStatements) ProcessMetrics(td pmetric.Metrics) error {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				ctx := ottlmetric.NewTransformContext(metrics.At(k), smetrics.Scope(), rmetrics.Resource())
				for _, statement := range m {
					_, _, err := statement.Execute(ctx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

var _ Context = &dataPointStatements{}
var _ MetricsContext = &dataPointStatements{}

type dataPointStatements []*ottl.Statement[ottldatapoints.TransformContext]

func (d dataPointStatements) isContext() {}

func (d dataPointStatements) ProcessMetrics(td pmetric.Metrics) error {
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
					err = d.handleNumberDataPoints(metric.Sum().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeGauge:
					err = d.handleNumberDataPoints(metric.Gauge().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeHistogram:
					err = d.handleHistogramDataPoints(metric.Histogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeExponentialHistogram:
					err = d.handleExponetialHistogramDataPoints(metric.ExponentialHistogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeSummary:
					err = d.handleSummaryDataPoints(metric.Summary().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d dataPointStatements) handleNumberDataPoints(dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) handleHistogramDataPoints(dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) handleExponetialHistogramDataPoints(dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) handleSummaryDataPoints(dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d dataPointStatements) callFunctions(ctx ottldatapoints.TransformContext) error {
	for _, statement := range d {
		_, _, err := statement.Execute(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
