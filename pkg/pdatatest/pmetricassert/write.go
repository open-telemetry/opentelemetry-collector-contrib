// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type writeOptions struct {
	includeValues bool
}

type WriteOption func(*writeOptions)

// IncludeValues is a WriteOption that includes numeric values like count, sum, min, max, explicit bounds, and bucket counts in the assertion file.
func IncludeValues() WriteOption {
	return func(o *writeOptions) {
		o.includeValues = true
	}
}

// WriteAssertionFile regenerates the default-strict assertion snapshot at path
// from actual. It is intended to be called manually during test authoring,
// analogous to golden.WriteMetrics, and removed before committing.
//
// Emitted snapshots capture identity fields only: resource attributes, scope
// name/version, metric name/type/unit/temporality/monotonic, and the set of
// datapoint attribute permutations. Values, timestamps, and exemplars are
// omitted.
func WriteAssertionFile(tb testing.TB, path string, actual pmetric.Metrics, opts ...WriteOption) error {
	tb.Helper()
	var cfg writeOptions
	for _, opt := range opts {
		opt(&cfg)
	}

	doc := normalize(actual)
	if !cfg.includeValues {
		maskValues(doc)
	}
	return writeDocument(path, doc)
}

func maskValues(doc *document) {
	for i := range doc.Resources {
		for j := range doc.Resources[i].Scopes {
			for k := range doc.Resources[i].Scopes[j].Metrics {
				m := &doc.Resources[i].Scopes[j].Metrics[k]
				for l := range m.Datapoints {
					dp := &m.Datapoints[l]
					dp.Count = nil
					dp.Sum = nil
					dp.ExplicitBounds = nil
					dp.BucketCounts = nil
					dp.Min = nil
					dp.Max = nil
				}
			}
		}
	}
}
