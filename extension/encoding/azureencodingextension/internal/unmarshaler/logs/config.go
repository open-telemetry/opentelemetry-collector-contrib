// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

type LogsConfig struct {
	// TimeFormats is a list of time formats parsing layouts for Azure Resource Log Records
	TimeFormats       []string `mapstructure:"time_formats"`
	IncludeCategories []string `mapstructure:"include_categories"`
	ExcludeCategories []string `mapstructure:"exclude_categories"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// DefaultTimeFormats is a list of well-known time formats parsing layouts,
// that was detected for supported Azure Resource Log Categories
// It is used as a default timestamp parsing formats to simplify configuration
// of logs parser for end-users
func DefaultTimeFormats() []string {
	return []string{
		"01/02/2006 15:04:05",
		"1/2/2006 3:04:05.000 PM -07:00",
		"1/2/2006 3:04:05 PM -07:00",
	}
}
