// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// sdktest performs partial comparison of [sdk.ResourceMetrics] to a [Spec].
package sdktest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/sdktest"

import (
	stdcmp "cmp"
	"context"
	"fmt"
	"slices"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/compare"
)

// Test the metrics returned by [metric.ManualReader.Collect] against the [Spec]
func Test(spec Spec, mr *metric.ManualReader, opts ...cmp.Option) error {
	var rm sdk.ResourceMetrics
	if err := mr.Collect(context.Background(), &rm); err != nil {
		return err
	}
	return Compare(spec, rm, opts...)
}

// Compare the [sdk.ResourceMetrics] against the [Spec]
func Compare(spec Spec, rm sdk.ResourceMetrics, opts ...cmp.Option) error {
	got := make(map[string]sdk.Metrics)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if _, ok := spec[m.Name]; ok {
				got[m.Name] = sortData(m)
			}
		}
	}

	want := make(map[string]sdk.Metrics)
	for name, spec := range spec {
		m := into(spec, got[name])
		want[name] = sortData(m)
	}

	cmpfn := func(a, b sdk.Metrics) int { return stdcmp.Compare(a.Name, b.Name) }
	mdgot := values(got, cmpfn)
	mdwant := values(want, cmpfn)

	opts = append(opts,
		cmp.Transformer("sdktest.Transform.int64", Transform[int64]),
		cmp.Transformer("sdktest.Transform.float64", Transform[float64]),
		// ignore attribute.Set while diffing, as we already compare the map[string]any returned by Transform
		cmp.FilterValues(func(_, _ attribute.Set) bool { return true }, cmp.Ignore()),
	)

	if diff := compare.Diff(mdwant, mdgot, opts...); diff != "" {
		return fmt.Errorf("\n%s", diff)
	}
	return nil
}

func into(spec Metric, base sdk.Metrics) sdk.Metrics {
	md := sdk.Metrics{Name: spec.Name, Description: base.Description, Unit: base.Unit}

	intSum := sdk.Sum[int64]{Temporality: spec.Temporality, IsMonotonic: spec.Monotonic}
	floatSum := sdk.Sum[float64]{Temporality: spec.Temporality, IsMonotonic: spec.Monotonic}
	intGauge := sdk.Gauge[int64]{}
	floatGauge := sdk.Gauge[float64]{}

	var idps *[]sdk.DataPoint[int64]
	var fdps *[]sdk.DataPoint[float64]

	switch spec.Type {
	case TypeSum:
		idps = &intSum.DataPoints
		fdps = &floatSum.DataPoints
	case TypeGauge:
		idps = &intGauge.DataPoints
		fdps = &floatGauge.DataPoints
	default:
		panic("todo")
	}

	for _, num := range spec.Numbers {
		attr := num.Attr.Into()

		switch {
		case num.Int != nil:
			dp := find[int64](base, attr)
			dp.Value = *num.Int
			*idps = append(*idps, dp)
		case num.Float != nil:
			dp := find[float64](base, attr)
			dp.Value = *num.Float
			*fdps = append(*fdps, dp)
		}
	}

	switch {
	case len(intSum.DataPoints) > 0:
		md.Data = intSum
	case len(floatSum.DataPoints) > 0:
		md.Data = floatSum
	case len(intGauge.DataPoints) > 0:
		md.Data = intGauge
	case len(floatGauge.DataPoints) > 0:
		md.Data = floatGauge
	}

	return md
}

func find[N num](base sdk.Metrics, set attribute.Set) sdk.DataPoint[N] {
	var dps []sdk.DataPoint[N]
	switch ty := base.Data.(type) {
	case sdk.Sum[N]:
		dps = ty.DataPoints
	case sdk.Gauge[N]:
		dps = ty.DataPoints
	}

	for _, dp := range dps {
		if dp.Attributes.Equals(&set) {
			return dp
		}
	}
	return sdk.DataPoint[N]{Attributes: set}
}

type num interface {
	int64 | float64
}

// DataPoint is like [sdk.DataPoint], but with the attributes as a plain
// map[string]any for better comparison.
type DataPoint[N num] struct {
	sdk.DataPoint[N]
	Attributes map[string]any
}

// Transform is used with [cmp.Transformer] to transform [sdk.DataPoint] into [DataPoint] during comparison.
//
// This is done because the [attribute.Set] inside the datapoint does not diff
// properly, as it is too deeply nested and as such truncated by [cmp].
func Transform[N num](dps []sdk.DataPoint[N]) []DataPoint[N] {
	out := make([]DataPoint[N], len(dps))
	for i, dp := range dps {
		attr := make(map[string]any)
		for _, kv := range dp.Attributes.ToSlice() {
			attr[string(kv.Key)] = kv.Value.AsInterface()
		}
		out[i] = DataPoint[N]{DataPoint: dp, Attributes: attr}
	}
	return out
}

func keys[K stdcmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortStableFunc(keys, stdcmp.Compare)
	return keys
}

func values[K comparable, V any](m map[K]V, cmp func(V, V) int) []V {
	vals := make([]V, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}

	slices.SortStableFunc(vals, cmp)
	return vals
}

func compareDp[N num](a, b sdk.DataPoint[N]) int {
	return stdcmp.Compare(
		a.Attributes.Encoded(attribute.DefaultEncoder()),
		b.Attributes.Encoded(attribute.DefaultEncoder()),
	)
}

func sortData(m sdk.Metrics) sdk.Metrics {
	switch ty := m.Data.(type) {
	case sdk.Sum[int64]:
		slices.SortStableFunc(ty.DataPoints, compareDp[int64])
		m.Data = ty
	case sdk.Sum[float64]:
		slices.SortStableFunc(ty.DataPoints, compareDp[float64])
		m.Data = ty
	case sdk.Gauge[int64]:
		slices.SortStableFunc(ty.DataPoints, compareDp[int64])
		m.Data = ty
	case sdk.Gauge[float64]:
		slices.SortStableFunc(ty.DataPoints, compareDp[float64])
		m.Data = ty
	}
	return m
}
