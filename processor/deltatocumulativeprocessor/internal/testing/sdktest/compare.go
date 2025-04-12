// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// sdktest performs partial comparison of [sdk.ResourceMetrics] to a [Spec].
package sdktest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/sdktest"

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/compare"
)

type Option = cmp.Option

// Test the metrics returned by [metric.ManualReader.Collect] against the [Spec]
func Test(spec Spec, mr *metric.ManualReader, opts ...Option) error {
	var rm sdk.ResourceMetrics
	if err := mr.Collect(context.Background(), &rm); err != nil {
		return err
	}
	return Compare(spec, rm, opts...)
}

// Compare the [sdk.ResourceMetrics] against the [Spec]
func Compare(spec Spec, rm sdk.ResourceMetrics, opts ...Option) error {
	got := Flatten(rm)
	want := Metrics(spec)

	diff := compare.Diff(want, got,
		IgnoreUnspec(spec),
		IgnoreTime(),
		IgnoreMetadata(),
		cmpopts.EquateEmpty(),
		Transform(),
		Sort(),
		cmp.Options(opts),
	)

	if diff != "" {
		return fmt.Errorf("\n%s", diff)
	}
	return nil
}

// IgnoreTime ignores [sdk.DataPoint.Time] and [sdk.DataPoint.StartTime],
// because those are changing per run and typically not of interest.
func IgnoreTime() Option {
	return cmp.Options{
		cmpopts.IgnoreFields(sdk.DataPoint[int64]{}, "StartTime", "Time"),
		cmpopts.IgnoreFields(sdk.DataPoint[float64]{}, "StartTime", "Time"),
	}
}

// IgnoreTime ignores [sdk.Metrics.Unit] and [sdk.Metrics.Description],
// because those are usually static
func IgnoreMetadata() Option {
	return cmpopts.IgnoreFields(sdk.Metrics{}, "Description", "Unit")
}

// IgnoreUnspec ignores any Metrics not present in the [Spec]
func IgnoreUnspec(spec Spec) Option {
	return cmpopts.IgnoreSliceElements(func(m sdk.Metrics) bool {
		_, ok := spec[m.Name]
		return !ok
	})
}

// Sort [sdk.Metrics] by name and [sdk.DataPoint] by their [attribute.Set]
func Sort() Option {
	return cmp.Options{
		cmpopts.SortSlices(func(a, b sdk.Metrics) bool {
			return a.Name < b.Name
		}),
		sort[int64](), sort[float64](),
	}
}

func sort[N int64 | float64]() Option {
	return cmpopts.SortSlices(func(a, b DataPoint[N]) bool {
		as := a.DataPoint.Attributes.Encoded(attribute.DefaultEncoder())
		bs := b.DataPoint.Attributes.Encoded(attribute.DefaultEncoder())
		return as < bs
	})
}

// DataPoint holds a [sdk.DataPoints] and its attributes as a plain map.
// See [Transform]
type DataPoint[N int64 | float64] struct {
	Attributes map[string]any
	sdk.DataPoint[N]
}

// Transform turns []sdk.DataPoint[N] into []DataPoint[N].
//
// Primarily done to have DataPoint.Attributes as a flat, diffable map instead
// of the hard to understand internal structure of [attribute.Set], which is
// being truncated by go-cmp before reaching the depth where attribute values
// appear.
//
// This must happen on the slice level, transforming the values is not
// sufficient because when entire DataPoints are added / removed, go-cmp does
// not apply transformers on the fields.
func Transform() Option {
	return cmp.Options{
		transform[int64](),
		transform[float64](),
		cmpopts.IgnoreTypes(attribute.Set{}),
	}
}

func transform[N int64 | float64]() Option {
	return cmpopts.AcyclicTransformer(fmt.Sprintf("sdktest.Transform.%T", *new(N)),
		func(dps []sdk.DataPoint[N]) []DataPoint[N] {
			out := make([]DataPoint[N], len(dps))
			for i, dp := range dps {
				out[i] = DataPoint[N]{DataPoint: dp, Attributes: attrMap(dp.Attributes)}
			}
			return out
		},
	)
}

func attrMap(set attribute.Set) map[string]any {
	m := make(map[string]any)
	for _, kv := range set.ToSlice() {
		m[string(kv.Key)] = kv.Value.AsInterface()
	}
	return m
}
