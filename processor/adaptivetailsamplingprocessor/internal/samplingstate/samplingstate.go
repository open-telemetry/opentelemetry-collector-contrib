// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package samplingstate defines the counter-sharing contract that lets
// adaptive_throughput samplers on multiple collector instances observe the
// fleet's combined traffic. Instances publish their per-interval key counts
// additively and read back the merged totals; every instance then recomputes
// the same rate table from the same merged input, so no leader or rate
// distribution is needed.
package samplingstate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/samplingstate"

import "context"

// CounterStore is the interface a sampling-state backend implements. External
// backends are collector extensions that satisfy it structurally; the
// processor resolves them from the host by component ID and type-asserts.
//
// Buckets are interval indexes (floor of wall-clock time over the sampler's
// adjustment interval), so instances with synchronized clocks address the
// same bucket for the same time period without coordinating. samplerID
// namespaces buckets per sampler (the processor uses the rule name); callers
// on different instances must configure the same samplerID and interval for
// their counts to merge.
//
// Implementations must be safe for concurrent use. They must not retain the
// counts map passed to AddCounts, and ReadCounts must return a map the caller
// owns. Backends should expire buckets after a few intervals; the processor
// only ever reads the bucket it just published.
type CounterStore interface {
	// AddCounts folds this instance's counts for one interval bucket into the
	// shared totals. Additions must be atomic per (samplerID, bucket, key) so
	// concurrent instances' contributions merge without loss.
	AddCounts(ctx context.Context, samplerID string, bucket int64, counts map[string]float64) error
	// ReadCounts returns the merged counts published for the bucket so far. A
	// bucket nobody has written is an empty map, not an error.
	ReadCounts(ctx context.Context, samplerID string, bucket int64) (map[string]float64, error)
}
