// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package drainprocessor implements a processor that applies the Drain log
// clustering algorithm to log records, annotating each record with a derived
// template string and cluster ID.
package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"
