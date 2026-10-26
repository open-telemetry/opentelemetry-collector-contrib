// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package failurecounter lets xottl configure the failure counter of OTTL sequences
// without exposing the counter as part of the stable OTTL API.
package failurecounter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/failurecounter"

import "go.opentelemetry.io/otel/metric"

// Counter wraps the counter incremented when a sequence ignores an error.
// As it lives in an internal package, it can only be built from within OTTL.
type Counter struct {
	metric.Int64Counter
}
