// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"

type TracesConfig struct {
	// TimeFormats is a list of time formats parsing layouts for Azure Traces Records
	TimeFormats []string `mapstructure:"time_formats"`

	// prevent unkeyed literal initialization
	_ struct{}
}
