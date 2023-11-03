// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

const defaultEndpoint = "127.0.0.1:11000"

type TapInfo struct {
	Name     string `mapstructure:"name" json:"name"`
	Endpoint string `mapstructure:"endpoint" json:"endpoint"`
}
type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	Taps                          []TapInfo                `mapstructure:"taps"`
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
	}
}

func (c *Config) Validate() error {
	if len(c.Taps) == 0 {
		return errors.New("no taps defined")
	}
	return nil
}
