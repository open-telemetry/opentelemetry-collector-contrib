// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

// Discoverable is an optional interface that receiver factories can implement
// to indicate they support k8s annotation-based discovery.
type Discoverable interface {
	// ValidateDiscoveryConfig validates that the provided configuration
	// only targets the discovered endpoint and not arbitrary external endpoints.
	ValidateForDiscovery(rawCfg map[string]any, discoveredEndpoint string) error
}

// ValidateEndpointConfig is a helper function for receivers with a simple
// "endpoint" field. Receivers can use this in their ValidateDiscoveryConfig.
func ValidateEndpointConfig(rawCfg map[string]any, discoveredEndpoint string) error {
	endpoint, ok := rawCfg["endpoint"].(string)
	if !ok || endpoint == "" {
		return nil
	}
	return validateEndpoint(endpoint, discoveredEndpoint)
}
