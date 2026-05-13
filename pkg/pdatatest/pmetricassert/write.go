// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// WriteAssertionFile regenerates the default-strict assertion snapshot at path
// from actual. It is intended to be called manually during test authoring,
// analogous to golden.WriteMetrics, and removed before committing.
//
// Emitted snapshots capture identity fields only: resource attributes, scope
// name/version, metric name/type/unit/temporality/monotonic, and the set of
// datapoint attribute permutations. Values, timestamps, and exemplars are
// omitted.
func WriteAssertionFile(tb testing.TB, path string, actual pmetric.Metrics) error {
	tb.Helper()
	doc := normalize(actual)
	return writeDocument(path, doc)
}
