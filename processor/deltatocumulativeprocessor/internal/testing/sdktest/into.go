// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdktest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/sdktest"

import (
	sdk "go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Metrics returns the [sdk.Metrics] defined by this [Spec]
func Metrics(spec Spec) []sdk.Metrics {
	md := make([]sdk.Metrics, 0, len(spec))
	for _, spec := range spec {
		md = append(md, spec.Into())
	}
	return md
}

func (spec Metric) Into() sdk.Metrics {
	m := sdk.Metrics{Name: spec.Name}
	if len(spec.Numbers) == 0 {
		return m
	}

	var (
		ints   []sdk.DataPoint[int64]
		floats []sdk.DataPoint[float64]
	)
	for _, n := range spec.Numbers {
		attr := n.Attr.Into()
		switch {
		case n.Int != nil:
			ints = append(ints, sdk.DataPoint[int64]{Attributes: attr, Value: *n.Int})
		case n.Float != nil:
			floats = append(floats, sdk.DataPoint[float64]{Attributes: attr, Value: *n.Float})
		}
	}

	switch {
	case spec.Type == TypeGauge && ints != nil:
		m.Data = sdk.Gauge[int64]{DataPoints: ints}
	case spec.Type == TypeGauge && floats != nil:
		m.Data = sdk.Gauge[float64]{DataPoints: floats}
	case spec.Type == TypeSum && ints != nil:
		m.Data = sdk.Sum[int64]{DataPoints: ints, Temporality: spec.Temporality, IsMonotonic: spec.Monotonic}
	case spec.Type == TypeSum && floats != nil:
		m.Data = sdk.Sum[float64]{DataPoints: floats, Temporality: spec.Temporality, IsMonotonic: spec.Monotonic}
	}

	return m
}

// Flatten turns the nested [sdk.ResourceMetrics] structure into a flat
// [sdk.Metrics] slice. If a metric is present multiple time in different scopes
// / resources, the last occurrence is used.
func Flatten(rm sdk.ResourceMetrics) []sdk.Metrics {
	set := make(map[string]sdk.Metrics)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			set[m.Name] = m
		}
	}
	md := make([]sdk.Metrics, 0, len(set))
	for _, m := range set {
		md = append(md, m)
	}
	return md
}
