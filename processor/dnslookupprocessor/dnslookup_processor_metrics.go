// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func (dp *dnsLookupProcessor) processMetrics(ctx context.Context, ms pmetric.Metrics) (pmetric.Metrics, error) {
	rm := ms.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		for _, pp := range dp.processPairs {
			switch pp.ContextID {
			case resource:
				err := pp.ProcessFn(ctx, rm.At(i).Resource().Attributes())
				if err != nil {
					return ms, err
				}
			case record:
				for j := 0; j < rm.At(i).ScopeMetrics().Len(); j++ {
					for k := 0; k < rm.At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
						err := dp.processMetricAttributes(ctx, rm.At(i).ScopeMetrics().At(j).Metrics().At(k), pp)
						if err != nil {
							return ms, err
						}
					}
				}
			default:
				return ms, errUnknownContextID
			}
		}

	}
	return ms, nil
}

func (dp *dnsLookupProcessor) processMetricAttributes(ctx context.Context, m pmetric.Metric, pp ProcessPair) error {
	// This is a lot of repeated code, but since there is no single parent superclass
	// between metric data types, we can't use polymorphism.
	//exhaustive:enforce

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			err := pp.ProcessFn(ctx, dps.At(i).Attributes())
			if err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			err := pp.ProcessFn(ctx, dps.At(i).Attributes())
			if err != nil {
				return err
			}
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			err := pp.ProcessFn(ctx, dps.At(i).Attributes())
			if err != nil {
				return err
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			err := pp.ProcessFn(ctx, dps.At(i).Attributes())
			if err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			err := pp.ProcessFn(ctx, dps.At(i).Attributes())
			if err != nil {
				return err
			}
		}
	case pmetric.MetricTypeEmpty:
	}

	return nil
}
