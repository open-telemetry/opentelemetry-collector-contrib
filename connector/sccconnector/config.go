// SPDX-License-Identifier: Apache-2.0

package sccconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/sccconnector"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
}

func (c *Config) Validate() error {
	return nil
}

func createDefaultConfig() component.Config {
	return &Config{}
}
