// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package cardinalityguardianprocessor enforces per-metric-per-label
// cardinality limits using HyperLogLog++ sketches. It is a drop-in pipeline
// component with zero web-server footprint — the binary exports no HTTP
// endpoints.
//
// # Problem
//
// In high-throughput observability pipelines, individual label keys (e.g.
// "session_id", "request_id", "user_id") can take on millions of unique values,
// causing cardinality explosions in downstream time-series databases. Each
// unique label combination creates a new time series; at cloud TSDB pricing,
// a single misbehaving label can cost thousands of dollars per month.
//
// # How it works
//
// For every (metric_name, label_key) pair the processor keeps two HLL sketches
// (current and previous epoch). The delta between them estimates how many new
// unique values the label has acquired this epoch. When that delta exceeds
// MaxCardinalityDeltaPerEpoch the offending attribute is handled per the
// configured EnforcementMode (EnforcementTagOnly, EnforcementOverflowAttribute,
// or EnforcementStripAndReaggregate).
//
// # Architecture
//
//	ConsumeMetrics (hot path, called concurrently by the Collector)
//	  └─ handleAttributes           (per data point)
//	        └─ shouldDrop           (per label key/value pair)
//	              ├─ hashAttrValue       — type-dispatched, zero-alloc Str case
//	              ├─ getShard            — maphash routing to 1/256 of key space
//	              ├─ shard.mu RLock      — fast path when tracker already exists
//	              └─ tracker.insert     — HLL InsertHash + lazy cached estimate
//
//	Background goroutine (one per processor lifetime, started in Start)
//	  └─ rotate — every EpochDurationSeconds, advances the sliding window
//	               across all 256 shards, one shard at a time
//
// Hot-path design rationale lives on the named identifiers themselves — see
// numShards, hashAttrValue, sketchPool, estimateInterval, and tracker.insert.
package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"
