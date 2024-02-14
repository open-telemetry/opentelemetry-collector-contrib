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
