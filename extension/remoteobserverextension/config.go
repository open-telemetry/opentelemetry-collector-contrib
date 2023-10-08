// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteobserverextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remoteobserverextension"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "127.0.0.1:11000",
		},
	}
}
