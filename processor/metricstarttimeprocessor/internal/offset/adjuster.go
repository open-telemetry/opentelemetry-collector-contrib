// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package offset // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/offset"

import (
	"context"
	"iter"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const Type = "offset"

type Adjuster struct {
	set    component.TelemetrySettings
	offset time.Duration
}

// NewAdjuster returns a new Adjuster which adjust metrics' start times based on the initial received points.
func NewAdjuster(set component.TelemetrySettings, offset time.Duration) *Adjuster {
	return &Adjuster{
		set:    set,
		offset: offset,
	}
}

func (a *Adjuster) AdjustMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for m := range all(md) {
		switch m.Type() {
		case pmetric.MetricTypeSum:
			for _, dp := range m.Sum().DataPoints().All() {
				if dp.StartTimestamp() == 0 {
					dp.SetStartTimestamp(dp.Timestamp() - pcommon.Timestamp(a.offset))
				}
			}
		case pmetric.MetricTypeGauge:
			for _, dp := range m.Gauge().DataPoints().All() {
				if dp.StartTimestamp() == 0 {
					dp.SetStartTimestamp(dp.Timestamp() - pcommon.Timestamp(a.offset))
				}
			}
		case pmetric.MetricTypeHistogram:
			for _, dp := range m.Histogram().DataPoints().All() {
				if dp.StartTimestamp() == 0 {
					dp.SetStartTimestamp(dp.Timestamp() - pcommon.Timestamp(a.offset))
				}
			}
		case pmetric.MetricTypeExponentialHistogram:
			for _, dp := range m.ExponentialHistogram().DataPoints().All() {
				if dp.StartTimestamp() == 0 {
					dp.SetStartTimestamp(dp.Timestamp() - pcommon.Timestamp(a.offset))
				}
			}
		case pmetric.MetricTypeSummary:
			for _, dp := range m.Summary().DataPoints().All() {
				if dp.StartTimestamp() == 0 {
					dp.SetStartTimestamp(dp.Timestamp() - pcommon.Timestamp(a.offset))
				}
			}
		}
	}
	return md, nil
}

func all(md pmetric.Metrics) iter.Seq[pmetric.Metric] {
	return func(yield func(pmetric.Metric) bool) {
		for _, rm := range md.ResourceMetrics().All() {
			for _, sm := range rm.ScopeMetrics().All() {
				for _, m := range sm.Metrics().All() {
					if !yield(m) {
						return
					}
				}
			}
		}
	}
}
