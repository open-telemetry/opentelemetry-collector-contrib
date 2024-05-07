// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gmetric // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor/internal/gmetric"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type List[T any] interface {
	At(int) T
	Len() int
}

func All[L List[T], T any](list L) func(func(int, T) bool) {
	return func(yield func(int, T) bool) {
		for i := 0; i < list.Len(); i++ {
			if !yield(i, list.At(i)) {
				break
			}
		}
	}
}

type ResourceMetric struct {
	Resource pcommon.Resource
	Scope    pcommon.InstrumentationScope
	pmetric.Metric
}

func (rm ResourceMetric) Ident() identity.Metric {
	return identity.OfResourceMetric(rm.Resource, rm.Scope, rm.Metric)
}

func (rm ResourceMetric) Typed() any {
	return Typed(rm.Metric)
}

func ResourceMetrics(md pmetric.Metrics) func(func(ResourceMetric) bool) {
	return func(yield func(ResourceMetric) bool) {
		ok := true
		All(md.ResourceMetrics())(func(_ int, rm pmetric.ResourceMetrics) bool {
			All(rm.ScopeMetrics())(func(_ int, sm pmetric.ScopeMetrics) bool {
				All(sm.Metrics())(func(_ int, m pmetric.Metric) bool {
					ok = yield(ResourceMetric{
						Resource: rm.Resource(),
						Scope:    sm.Scope(),
						Metric:   m,
					})
					return ok
				})
				return ok
			})
			return ok
		})
	}
}
