// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpmextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tpmextension"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct{}

func createDefaultConfig() component.Config {
	return &Config{}
}

func (cfg *Config) Validate() error {
	return nil
}
