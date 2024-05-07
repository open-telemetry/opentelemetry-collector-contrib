// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import "go.opentelemetry.io/collector/pdata/pmetric"

func Filter(metrics pmetric.Metrics, fn func(m Metric) bool) {
	metrics.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				return !fn(From(rm.Resource(), sm.Scope(), m))
			})
			return false
		})
		return false
	})
}

func Each(metrics pmetric.Metrics, fn func(m Metric)) {
	Filter(metrics, func(m Metric) bool {
		fn(m)
		return true
	})
}

func All(metrics pmetric.Metrics) func(func(Metric) bool) {
	return func(yield func(Metric) bool) {
		rms := metrics.ResourceMetrics()
		for r := 0; r < rms.Len(); r++ {
			sms := rms.At(r).ScopeMetrics()
			for s := 0; s < sms.Len(); s++ {
				ms := sms.At(s).Metrics()
				for m := 0; m < ms.Len(); m++ {
					yield(From(rms.At(r).Resource(), sms.At(s).Scope(), ms.At(m)))
				}
			}
		}
	}
}
