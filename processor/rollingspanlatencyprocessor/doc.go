// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package rollingspanlatencyprocessor labels spans as "slow" or "very_slow"
// based on a per-key rolling EWMA latency baseline.
package rollingspanlatencyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor"
