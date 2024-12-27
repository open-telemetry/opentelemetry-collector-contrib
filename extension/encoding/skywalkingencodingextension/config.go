// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/skywalkingencodingextension"
import (
	"fmt"
)

type Config struct {
	Protocol string `mapstructure:"protocol"`
}

func (c *Config) Validate() error {
	if c.Protocol != skywalkingProto {
		return fmt.Errorf("unsupported protocol: %q", c.Protocol)
	}
	return nil
}
