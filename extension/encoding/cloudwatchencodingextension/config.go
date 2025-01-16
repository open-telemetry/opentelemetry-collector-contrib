// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension"

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"
	"go.opentelemetry.io/collector/component"
)

type contentEncoding string

const (
	noEncoding     = ""
	gzipEncoding   = "gzip"
	base64Encoding = "base64"
)

type Config struct {
	Encoding []contentEncoding `mapstructure:"content_encoding"`
	Format   cloudwatch.Format `mapstructure:"format"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Encoding: make([]contentEncoding, 0),
		Format:   cloudwatch.OTelFormat,
	}
}

func (c *Config) Validate() error {
	for _, encoding := range c.Encoding {
		switch encoding {
		case noEncoding:
		case gzipEncoding:
		case base64Encoding:
		default:
			return fmt.Errorf("unknown content encoding %q", encoding)
		}
	}
	if isValid, err := cloudwatch.IsFormatValid(c.Format); !isValid {
		return err
	}
	return nil
}
