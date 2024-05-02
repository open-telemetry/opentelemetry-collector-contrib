// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"
import (
	"fmt"
)

type JaegerProtocol string

const (
	JaegerProtocolProtobuf JaegerProtocol = "protobuf"
	JaegerProtocolJSON     JaegerProtocol = "json"
)

type Config struct {
	Protocol JaegerProtocol `mapstructure:"protocol"`
}

func (c *Config) Validate() error {
	switch c.Protocol {
	case JaegerProtocolProtobuf:
	case JaegerProtocolJSON:
	default:
		return fmt.Errorf("invalid protocol %q", c.Protocol)
	}
	return nil
}
