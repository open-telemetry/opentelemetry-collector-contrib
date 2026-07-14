// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
)

// FuzzShouldDrop aggressively hammers the core cardinality logic with
// randomized string inputs to ensure xxhash and HyperLogLog never panic,
// panic out-of-bounds, or deadlock on malformed data.
func FuzzShouldDrop(f *testing.F) {
	// 1. Seed Corpus: Provide a few "normal" inputs to start the fuzzer
	f.Add("http.server.duration", "session_id", "12345-abcde")
	f.Add("db.query", "query_hash", "SELECT * FROM users")
	f.Add("", "", "")                                          // Empty strings
	f.Add("malformed_metric", "weird_key", "\x00\x01\x02\xFF") // Binary junk

	// 2. Setup a lightweight processor outside the loop so we don't
	// re-allocate the 256 shards on every single fuzz iteration.
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 50,
		EpochDurationSeconds:        300,
	}
	// Create the mock OTel settings (which includes the mock MeterProvider and Logger)
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	p, err := newCardinalityProcessor(context.Background(), cfg, set, consumertest.NewNop())
	if err != nil {
		f.Fatalf("Failed to create processor: %v", err)
	}

	cp := p.(*cardinalityProcessor)

	// 3. The Fuzz Target: Go will generate random strings and feed them here
	f.Fuzz(func(_ *testing.T, metricName, attrKey, attrVal string) {
		// The goal isn't to check the boolean result, but simply to prove
		// that this function never panics, regardless of the input.
		_ = cp.shouldDrop(metricName, attrKey, pcommon.NewValueStr(attrVal))
	})
}
