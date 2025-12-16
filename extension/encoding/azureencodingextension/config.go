// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"
)

type Config struct {
	Traces  traces.TracesConfig   `mapstructure:"traces"`
	Logs    logs.LogsConfig       `mapstructure:"logs"`
	Metrics metrics.MetricsConfig `mapstructure:"metrics"`

	// prevent unkeyed literal initialization
	_ struct{}
}
