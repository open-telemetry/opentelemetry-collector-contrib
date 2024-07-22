// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/remotetap"
)

type metricAttributesProcessor struct {
	logger   *zap.Logger
	attrProc *attraction.AttrProc
	skipExpr expr.BoolExpr[ottlmetric.TransformContext]

	componentID remotetap.ComponentID
	remoteTap   remotetap.Publisher
}

// newMetricAttributesProcessor returns a processor that modifies attributes of a
// metric record. To construct the attributes processors, the use of the factory
// methods are required in order to validate the inputs.
func newMetricAttributesProcessor(logger *zap.Logger, componentID component.ID, attrProc *attraction.AttrProc, skipExpr expr.BoolExpr[ottlmetric.TransformContext]) *metricAttributesProcessor {
	return &metricAttributesProcessor{
		logger:      logger,
		componentID: remotetap.ComponentID(componentID.String()),
		attrProc:    attrProc,
		skipExpr:    skipExpr,
	}
}

// Start is invoked during service startup.
func (a *metricAttributesProcessor) start(_ context.Context, host component.Host) error {
	remoteTapType, _ := component.NewType("remotetap")
	remoteTapExt := host.GetExtensions()[component.NewID(remoteTapType)]
	if remoteTapExt != nil {
		a.remoteTap = remoteTapExt.(remotetap.Publisher)
		a.remoteTap.Register(a.componentID)
	}
	return nil
}

func (a *metricAttributesProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		resource := rs.Resource()
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			scope := ils.Scope()
			metrics := ils.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				if a.skipExpr != nil {
					skip, err := a.skipExpr.Eval(ctx, ottlmetric.NewTransformContext(m, metrics, scope, resource, ils, rs))
					if err != nil {
						return md, err
					}
					if skip {
						continue
					}
				}
				a.processMetricAttributes(ctx, m)
			}
		}
	}
	if a.remoteTap != nil && a.remoteTap.IsActive(a.componentID) {
		a.remoteTap.PublishMetrics(a.componentID, md)
	}
	return md, nil
}

// Attributes are provided for each log and trace, but not at the metric level
// Need to process attributes for every data point within a metric.
func (a *metricAttributesProcessor) processMetricAttributes(ctx context.Context, m pmetric.Metric) {

	// This is a lot of repeated code, but since there is no single parent superclass
	// between metric data types, we can't use polymorphism.
	//exhaustive:enforce
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
	case pmetric.MetricTypeEmpty:
	}
}
