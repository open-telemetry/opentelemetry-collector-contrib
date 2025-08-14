// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

// Discoverable is an interface that receivers can implement to support discovery mode.
// Receivers implementing this interface can provide custom validation logic for their
// configurations when used with the receiver creator's discovery mechanism.
//
// The ValidateDiscovery method ensures that the receiver configuration only scrapes from the
// discovered endpoint, preventing security issues where a configuration might attempt
// to scrape from unintended sources.
type Discoverable interface {
	// ValidateDiscovery validates that the rawCfg unmarshals to a valid configuration that
	// ensures the receiver will only collect data from the discoveredEndpoint.
	//
	// Parameters:
	//   - rawCfg: The raw configuration map as provided in discovery annotations
	//   - discoveredEndpoint: The endpoint discovered by the observer (e.g., "10.1.2.3:8080")
	//
	// Returns:
	//   - error: nil if the configuration is valid and secure, or an error describing
	//            why the configuration is invalid or potentially insecure
	ValidateDiscovery(rawCfg map[string]any, discoveredEndpoint string) error
}
