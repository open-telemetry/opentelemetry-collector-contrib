// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package avrologencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/avrologencodingextension"

import "errors"

var errNoSchema = errors.New("no schema provided")

type Config struct {
	Schema string `mapstructure:"schema"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if c.Schema == "" {
		return errNoSchema
	}

	return nil
}
