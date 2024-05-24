// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

// Config holds the configuration for the GeoIP processor.
type Config struct{}

func (cfg *Config) Validate() error {
	return nil
}
