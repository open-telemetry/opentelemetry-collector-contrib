// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension"

import (
	codec "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/codec"
)

type Config struct {
	LogCodecs    map[string]codec.Log
	MetricCodecs map[string]codec.Metric
	TraceCodecs  map[string]codec.Trace
}
