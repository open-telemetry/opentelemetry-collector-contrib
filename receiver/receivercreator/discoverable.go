// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// Discoverable is implemented by receiver configurations that support annotation-based
// discovery. Receivers whose target does not come from a top-level "endpoint" field must
// implement it, since the default validation only understands "endpoint".
type Discoverable interface {
	// ValidateForDiscovery reports whether rawCfg, supplied by the author of the observed
	// Pod, confines the receiver to discoveredEndpoint. Values in rawCfg are already
	// expanded, so every target is concrete. Implementations must also reject settings
	// that read credentials or files from the Collector itself.
	ValidateForDiscovery(rawCfg map[string]any, discoveredEndpoint string) error
}

// validateDiscoveredConfig validates a configuration built from Pod annotation hints. The
// Pod author is not necessarily trusted by the Collector, so the resulting receiver must
// only ever collect from discoveredEndpoint, which is reported by the observer rather than
// taken from the annotation.
func (run *receiverRunner) validateDiscoveredConfig(factory rcvr.Factory, rawCfg userConfigMap, discoveredEndpoint string) error {
	if discoverable, ok := factory.CreateDefaultConfig().(Discoverable); ok {
		return discoverable.ValidateForDiscovery(rawCfg, discoveredEndpoint)
	}

	run.logger.Warn("receiver does not implement Discoverable, falling back to validating only the top-level endpoint field",
		zap.Stringer("receiver", factory.Type()))
	return validateEndpointConfig(rawCfg, discoveredEndpoint)
}

// validateEndpointConfig validates a top-level "endpoint" field against the discovered
// endpoint. It cannot see targets held in any other field, which is why receivers that
// keep their target elsewhere have to implement Discoverable.
func validateEndpointConfig(rawCfg userConfigMap, discoveredEndpoint string) error {
	endpoint, ok := rawCfg[endpointConfigKey].(string)
	if !ok || endpoint == "" {
		return nil
	}
	return validateEndpoint(endpoint, discoveredEndpoint)
}
