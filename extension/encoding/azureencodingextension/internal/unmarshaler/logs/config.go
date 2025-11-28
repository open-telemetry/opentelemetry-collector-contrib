// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

type LogsConfig struct {
	// TimeFormats is a list of time formats parsing layouts for Azure Traces Records
	TimeFormats       []string `mapstructure:"time_formats"`
	IncludeCategories []string `mapstructure:"include_categories"`
	ExcludeCategories []string `mapstructure:"exclude_categories"`

	// prevent unkeyed literal initialization
	_ struct{}
}
