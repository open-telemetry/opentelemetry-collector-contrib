// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

var _ component.ConfigValidator = (*Config)(nil)

type Config struct {
	Protocol string `mapstructure:"protocol"`
	Version  string `mapstructure:"version"`
}

func (c *Config) Validate() error {
	if c.Protocol != zipkinProtobufEncoding && c.Protocol != zipkinJSONEncoding && c.Protocol != zipkinThriftEncoding {
		return fmt.Errorf("unsupported protocol: %q", c.Protocol)
	}
	if c.Version != v1 && c.Version != v2 {
		return fmt.Errorf("unsupported version: %q", c.Version)
	}

	return nil
}
