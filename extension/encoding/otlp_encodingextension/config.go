// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"

import "fmt"

type OTLPProtocol string

const (
	OTLPProtocolJSON OTLPProtocol = "json"
)

type Config struct {
	Protocol OTLPProtocol `mapstructure:"protocol"`
}

func (c *Config) Validate() error {
	switch c.Protocol {
	case OTLPProtocolJSON:
	default:
		return fmt.Errorf("invalid protocol %q", c.Protocol)
	}
	return nil
}
