// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package leafspanpruningprocessor identifies duplicate/similar leaf spans
// within a trace and replaces each group with an aggregated summary span.
// Leaf spans are spans that are not referenced as a parent by any other span.
package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"
