// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package dynamicsamplingprocessor implements a processor that performs adaptive
// trace sampling by evaluating first-match rules against accumulated trace data,
// routing each trace to a sampler that produces a known sample rate, and
// encoding that rate as `ot=th` in W3C TraceState.
package dynamicsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor"
