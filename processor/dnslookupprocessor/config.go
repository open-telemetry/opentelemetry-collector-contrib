// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

// Config holds the configuration for the DnsLookup processor.
type Config struct{}

func (cfg *Config) Validate() error {
	return nil
}
