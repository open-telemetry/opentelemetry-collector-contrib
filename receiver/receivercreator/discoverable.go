// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

// Discoverable is an optional interface that receiver configuration types can
// implement to provide custom validation for k8s annotation-based discovery.
// When a receiver's config implements this interface, the receivercreator will
// call ValidateForDiscovery to ensure the annotation-provided configuration
// only targets the discovered endpoint. Receivers that do not implement this
// interface will fall back to basic "endpoint" field validation.
type Discoverable interface {
	// ValidateDiscoveryConfig validates that the provided configuration
	// only targets the discovered endpoint and not arbitrary external endpoints.
	ValidateForDiscovery(rawCfg map[string]any, discoveredEndpoint string) error
}

// ValidateEndpointConfig is a helper that validates a simple top-level
// "endpoint" field against the discovered endpoint. It is used as the
// default validation for receivers that do not implement the Discoverable
// interface. It can also be used by Discoverable implementations for
// receivers with a simple "endpoint" configuration field.
func ValidateEndpointConfig(rawCfg map[string]any, discoveredEndpoint string) error {
	endpoint, ok := rawCfg["endpoint"].(string)
	if !ok || endpoint == "" {
		return nil
	}
	return validateEndpoint(endpoint, discoveredEndpoint)
}
