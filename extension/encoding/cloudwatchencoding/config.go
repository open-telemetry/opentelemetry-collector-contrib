// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

type contentEncoding string

const (
	NoEncoding  = ""
	GZipEncoded = "gzip"
)

type Config struct {
	Encoding contentEncoding `mapstructure:"content_encoding"`
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func (c *Config) Validate() error {
	switch c.Encoding {
	case NoEncoding:
	case GZipEncoded:
	default:
		return fmt.Errorf("unknown content encoding %q", c.Encoding)
	}
	return nil
}
