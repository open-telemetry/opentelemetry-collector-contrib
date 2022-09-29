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

package attributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermetric"
)

type metricAttributesProcessor struct {
	logger   *zap.Logger
	attrProc *attraction.AttrProc
	include  filtermetric.Matcher
	exclude  filtermetric.Matcher
}

// newMetricAttributesProcessor returns a processor that modifies attributes of a
// metric record. To construct the attributes processors, the use of the factory
// methods are required in order to validate the inputs.
func newMetricAttributesProcessor(logger *zap.Logger, attrProc *attraction.AttrProc, include, exclude filtermetric.Matcher) *metricAttributesProcessor {
	return &metricAttributesProcessor{
		logger:   logger,
		attrProc: attrProc,
		include:  include,
		exclude:  exclude,
	}
}

func (a *metricAttributesProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				mr := metrics.At(k)
				if filtermetric.SkipMetric(a.include, a.exclude, mr, a.logger) {
					continue
				}

				a.processMetricAttributes(ctx, mr)
			}
		}
	}
	return md, nil
}

// Attributes are provided for each log and trace, but not at the metric level
// Need to process attributes for every data point within a metric.
func (a *metricAttributesProcessor) processMetricAttributes(ctx context.Context, m pmetric.Metric) {

	// This is a lot of repeated code, but since there is no single parent superclass
	// between metric data types, we can't use polymorphism.
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			a.attrProc.Process(ctx, a.logger, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			a.attrProc.Process(ctx, a.logger, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			a.attrProc.Process(ctx, a.logger, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			a.attrProc.Process(ctx, a.logger, dps.At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			a.attrProc.Process(ctx, a.logger, dps.At(i).Attributes())
		}
	}
}
