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

package comparetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/pdatautil"
)

func metricsByName(metricSlice pmetric.MetricSlice) map[string]pmetric.Metric {
	byName := make(map[string]pmetric.Metric, metricSlice.Len())
	for i := 0; i < metricSlice.Len(); i++ {
		a := metricSlice.At(i)
		byName[a.Name()] = a
	}
	return byName
}

func getDataPointSlice(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	var dataPointSlice pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.Type()))
	}
	return dataPointSlice
}

func lessResourceMetrics(subsort bool) func(a, b pmetric.ResourceMetrics) bool {
	return func(a, b pmetric.ResourceMetrics) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		if pdatautil.MapHash(a.Resource().Attributes()) != pdatautil.MapHash(b.Resource().Attributes()) {
			return pdatautil.MapHash(a.Resource().Attributes()) < pdatautil.MapHash(b.Resource().Attributes())
		}

		if a.ScopeMetrics().Len() != b.ScopeMetrics().Len() {
			return a.ScopeMetrics().Len() < b.ScopeMetrics().Len()
		}

		if subsort {
			lessWithSubsort := lessScopeMetrics(true)
			a.ScopeMetrics().Sort(lessWithSubsort)
			b.ScopeMetrics().Sort(lessWithSubsort)
		}

		less := lessScopeMetrics(false)
		for i := 0; i < a.ScopeMetrics().Len(); i++ {
			aScope := a.ScopeMetrics().At(i)
			bScope := b.ScopeMetrics().At(i)
			if less(aScope, bScope) {
				return true
			}
		}
		return false
	}
}

func lessScopeMetrics(subsort bool) func(a, b pmetric.ScopeMetrics) bool {
	return func(a, b pmetric.ScopeMetrics) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		if pdatautil.MapHash(a.Scope().Attributes()) != pdatautil.MapHash(b.Scope().Attributes()) {
			return pdatautil.MapHash(a.Scope().Attributes()) < pdatautil.MapHash(b.Scope().Attributes())
		}

		if a.Scope().Name() != b.Scope().Name() {
			return a.Scope().Name() < b.Scope().Name()
		}
		if a.Scope().Version() != b.Scope().Version() {
			return a.Scope().Version() < b.Scope().Version()
		}
		if a.Metrics().Len() != b.Metrics().Len() {
			return a.Metrics().Len() < b.Metrics().Len()
		}

		if subsort {
			lessWithSubsort := lessMetrics(true)
			a.Metrics().Sort(lessWithSubsort)
			b.Metrics().Sort(lessWithSubsort)
		}

		less := lessMetrics(false)
		for i := 0; i < a.Metrics().Len(); i++ {
			aMetric := a.Metrics().At(i)
			bMetric := b.Metrics().At(i)
			if less(aMetric, bMetric) {
				return true
			}
		}
		return false
	}
}

func lessMetrics(subsort bool) func(a, b pmetric.Metric) bool {
	return func(a, b pmetric.Metric) bool {
		if a.Name() != b.Name() {
			return a.Name() < b.Name()
		}
		if a.Description() != b.Description() {
			return a.Description() < b.Description()
		}
		if a.Unit() != b.Unit() {
			return a.Unit() < b.Unit()
		}
		if a.Type() != b.Type() {
			return a.Type() < b.Type()
		}

		var aDataPoints pmetric.NumberDataPointSlice
		var bDataPoints pmetric.NumberDataPointSlice

		switch a.Type() {
		case pmetric.MetricTypeGauge:
			aDataPoints = a.Gauge().DataPoints()
			bDataPoints = b.Gauge().DataPoints()
		case pmetric.MetricTypeSum:
			if a.Sum().AggregationTemporality() != b.Sum().AggregationTemporality() {
				return a.Sum().AggregationTemporality() < b.Sum().AggregationTemporality()
			}
			if a.Sum().IsMonotonic() != b.Sum().IsMonotonic() {
				return b.Sum().IsMonotonic()
			}
			aDataPoints = a.Sum().DataPoints()
			bDataPoints = b.Sum().DataPoints()
		}

		if subsort {
			aDataPoints.Sort(lessDataPoints)
			bDataPoints.Sort(lessDataPoints)
		}

		for i := 0; i < aDataPoints.Len(); i++ {
			aDataPoint := aDataPoints.At(i)
			bDataPoint := bDataPoints.At(i)
			if lessDataPoints(aDataPoint, bDataPoint) {
				return true
			}
		}
		return false
	}
}

func lessDataPoints(a, b pmetric.NumberDataPoint) bool {
	if pdatautil.MapHash(a.Attributes()) != pdatautil.MapHash(b.Attributes()) {
		return pdatautil.MapHash(a.Attributes()) < pdatautil.MapHash(b.Attributes())
	}

	if a.IntValue() != b.IntValue() {
		return a.IntValue() < b.IntValue()
	}
	if a.DoubleValue() != b.DoubleValue() {
		return a.DoubleValue() < b.DoubleValue()
	}
	return false
}
