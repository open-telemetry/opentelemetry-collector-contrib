// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package lookupprocessor contains a processor that enriches telemetry data
// by performing lookups from various sources (files, APIs, databases) and
// adding the results as attributes. Interface values inside the package follow
// the functional composition RFC so implementations are always created through
// constructors exposed by github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup.
// To operate, the processor requires a lookup extension registered in the host;
// no implicit or built-in sources are instantiated by this package.
package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"
