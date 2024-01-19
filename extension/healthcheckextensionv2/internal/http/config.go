// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import "go.opentelemetry.io/collector/config/confighttp"

type Settings struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"`

	Config PathSettings   `mapstructure:"config"`
	Status StatusSettings `mapstructure:"status"`
}

type PathSettings struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type StatusSettings struct {
	PathSettings `mapstructure:",squash"`
	Detailed     bool `mapstructure:"detailed"`
}
