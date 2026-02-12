// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package lookupprocessor contains a processor that enriches telemetry data
// by performing lookups from various sources and adding the results as attributes.
//
// Sources are configured via the processor's configuration using the source.type field.
// Built-in sources include "noop" (for testing) and "yaml" (file-based lookups).
// Third-party sources can be registered using [NewFactoryWithOptions] with [WithSources].
//
// The public API for creating custom sources is in the lookupsource subpackage.
// See [lookupsource.NewSourceFactory] and [lookupsource.NewSource] for details.
package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"
