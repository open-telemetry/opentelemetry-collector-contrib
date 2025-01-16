// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	Format cloudwatch.Format `mapstructure:"format"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Format: cloudwatch.OTelFormat,
	}
}

func (c *Config) Validate() error {
	if isValid, err := cloudwatch.IsFormatValid(c.Format); !isValid {
		return err
	}
	return nil
}
