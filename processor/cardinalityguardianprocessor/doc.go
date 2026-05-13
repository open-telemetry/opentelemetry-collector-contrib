// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package cardinalityguardianprocessor implements an OpenTelemetry Collector metrics
// processor that enforces per-metric-per-label cardinality limits using
// HyperLogLog sketches. It is designed to be a drop-in pipeline component with
// zero web-server footprint — the binary exports no HTTP endpoints.
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
// For every (metric_name, label_key) pair the processor maintains two
// HyperLogLog++ sketches — one for the current epoch and one for the
// previous epoch. On each data point the unique count is estimated as the
// delta between those two sketches. When the delta exceeds the configured
// MaxCardinalityDeltaPerEpoch the processor handles the offending attribute
// according to the configured EnforcementMode: tagging it for downstream
// routing, replacing the value with a sentinel, or stripping it entirely.
//
// # Architecture
//
//	ConsumeMetrics (hot path, called concurrently by the Collector)
//	  └─ handleAttributes           (per data point)
//	        └─ shouldDrop           (per label key/value pair)
//	              ├─ xxhash.Sum64String  — zero-alloc uint64 hash of value
//	              ├─ getShard            — maphash routing to 1/256 of key space
//	              ├─ shard.mu RLock      — fast path when tracker already exists
//	              └─ tracker.insert     — HLL InsertHash + lazy cached estimate
//
//	Background goroutine (one per processor lifetime, started in Start)
//	  └─ rotate — every EpochDurationSeconds, advances the sliding window
//	               across all 256 shards, one shard at a time
//
// # Performance design decisions
//
//   - 256-shard RWMutex array: A single global lock would serialize all
//     goroutines in the Collector's metric pipeline. With 256 independently
//     locked shards, contention is reduced to roughly 1/256th under a uniform
//     metric-name distribution, enabling near-linear scaling with CPU count.
//
//   - xxhash.Sum64String instead of Insert([]byte): The axiomhq/hyperloglog
//     library's Insert method internally calls a package-level `hash` function
//     variable. Because the compiler cannot see through function variables, any
//     []byte argument would escape to the heap, adding an allocation per call.
//     xxhash.Sum64String accepts a string directly, computes the 64-bit hash
//     entirely on the stack, and passes the result to InsertHash(uint64), which
//     is a pure value-type operation with zero heap pressure.
//
//   - sync.Pool for HyperLogLog sketches: Allocating a fresh hyperloglog.Sketch
//     for every new (metric, label) pair or every epoch rotation would spike GC
//     pressure during cardinality explosions — precisely when the processor is
//     busiest. The pool pre-allocates sketches and hands them out at O(1) cost.
//     Note: dirty sketches are NOT returned to the pool (see sketchPool for the
//     full explanation), so the pool acts as a pre-allocation cache rather than
//     a traditional free list.
//
//   - Lazy cached estimates: Calling hyperloglog.Sketch.Estimate() triggers an
//     internal mergeSparse() that allocates ~5 heap objects per call when the
//     sketch is in sparse mode. The processor caches the last estimate and only
//     refreshes it every estimateInterval inserts (using a power-of-2 bitmask
//     check, which is a single AND instruction). See tracker.insert for the
//     full two-phase strategy.
package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"
