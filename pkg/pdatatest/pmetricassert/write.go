// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type writeOptions struct {
	includeValues                  bool
	includeHistogramExplicitBounds bool
}

// WriteOption configures the snapshot generation.
type WriteOption interface {
	apply(*writeOptions)
}

type includeValuesOption struct{}

func (includeValuesOption) apply(o *writeOptions) { o.includeValues = true }

// IncludeValues opts into asserting the exact values of number datapoints
// (gauge and sum metrics). When enabled, generated snapshots will include
// the 'value' field.
func IncludeValues() WriteOption {
	return includeValuesOption{}
}

type includeHistogramExplicitBoundsOption struct{}

func (includeHistogramExplicitBoundsOption) apply(o *writeOptions) {
	o.includeHistogramExplicitBounds = true
}

// IncludeHistogramExplicitBounds opts into asserting each histogram datapoint's
// exact explicit bounds without other histogram values.
func IncludeHistogramExplicitBounds() WriteOption {
	return includeHistogramExplicitBoundsOption{}
}

// WriteAssertionFile regenerates the default-strict assertion snapshot at path
// from actual. It is intended to be called manually during test authoring,
// analogous to golden.WriteMetrics, and removed before committing.
//
// By default, emitted snapshots capture identity fields only: resource
// attributes, scope name/version, metric name/type/unit/temporality/monotonic,
// and the set of datapoint attribute permutations. Values, timestamps, and
// exemplars are omitted.
//
// The input metrics must be semantically valid. WriteAssertionFile normalizes
// valid metrics for assertion readability; it does not validate producer
// output.
func WriteAssertionFile(tb testing.TB, path string, actual pmetric.Metrics, opts ...WriteOption) error {
	tb.Helper()
	var o writeOptions
	for _, opt := range opts {
		opt.apply(&o)
	}
	doc := normalize(actual, o)
	return writeDocument(path, doc)
}
