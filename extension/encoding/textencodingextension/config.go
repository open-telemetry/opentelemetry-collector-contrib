// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
import (
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

type Config struct {
	Encoding              string `mapstructure:"encoding"`
	MarshalingSeparator   string `mapstructure:"marshaling_separator"`
	UnmarshalingSeparator string `mapstructure:"unmarshaling_separator"`
}

func (c *Config) Validate() error {
	if c.UnmarshalingSeparator != "" {
		if _, err := regexp.Compile(c.UnmarshalingSeparator); err != nil {
			return err
		}
	}
	encCfg := textutils.NewEncodingConfig()
	encCfg.Encoding = c.Encoding
	_, err := encCfg.Build()
	if err != nil {
		return err
	}
	return nil
}
