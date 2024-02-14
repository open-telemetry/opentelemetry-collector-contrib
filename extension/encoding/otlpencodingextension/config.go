// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"
import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

var _ component.ConfigValidator = (*Config)(nil)

type Config struct {
	Protocol string `mapstructure:"protocol"`
}

func (c *Config) Validate() error {
	if c.Protocol != otlpProto && c.Protocol != otlpJSON {
		return fmt.Errorf("unsupported protocol: %q", c.Protocol)
	}

	return nil
}
