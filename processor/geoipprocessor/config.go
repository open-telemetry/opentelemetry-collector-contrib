// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import "errors"

// Config holds the configuration for the GeoIP processor.
type Config struct {
	// Fields defines a list of resource attributes keys to look for the IP value.
	// The first matching attribute will be used to gather the IP geographical metadata.
	// Example:
	//   fields:
	//     - client.ip
	Fields []string `mapstructure:"fields"`
}

func (cfg *Config) Validate() error {
	if len(cfg.Fields) < 1 {
		return errors.New("must specify at least field to look for the IP when using the geoip processor")
	}
	return nil
}
