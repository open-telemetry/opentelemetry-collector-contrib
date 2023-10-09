// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlp_encodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlp_encodingextension"

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
