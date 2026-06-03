// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package genainormalizerprocessor provides a processor that normalizes GenAI
// telemetry attributes from OpenInference to the official OTel GenAI Semantic
// Conventions.
package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"
