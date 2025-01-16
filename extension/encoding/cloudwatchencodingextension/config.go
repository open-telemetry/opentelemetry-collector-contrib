// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func (c *Config) Validate() error {
	return nil
}
