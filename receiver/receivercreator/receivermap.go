// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"reflect"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// receiverEntry holds a receiver component along with the resolved config
// that was used to create it. This allows OnChange to compare configs and
// avoid unnecessary restarts when the effective config hasn't changed.
type receiverEntry struct {
	receiver component.Component
	id       component.ID
	// resolvedConfig is the fully expanded template config used to create the receiver
	resolvedConfig userConfigMap
	// resolvedDiscoveredConfig is the discovered config (e.g., endpoint target)
	resolvedDiscoveredConfig userConfigMap
	// computedResourceAttrs are the resource attributes computed from the endpoint env
	// at receiver creation time. These are compared on endpoint changes to detect when
	// metadata changes (like pod labels) would affect telemetry, even if the receiver
	// config itself is unchanged.
	computedResourceAttrs map[string]string
}

// configsEqual returns true if the receiver's effective config and resource attributes
// would be the same with the given parameters. This is used during OnChange to determine
// if a receiver restart is necessary.
func (re *receiverEntry) configsEqual(resolvedConfig, resolvedDiscoveredConfig userConfigMap, computedAttrs map[string]string) bool {
	return reflect.DeepEqual(re.resolvedConfig, resolvedConfig) &&
		reflect.DeepEqual(re.resolvedDiscoveredConfig, resolvedDiscoveredConfig) &&
		reflect.DeepEqual(re.computedResourceAttrs, computedAttrs)
}

// receiverMap is a multimap for mapping one endpoint id to many receiver entries.
// It does not deduplicate the same value being associated with the same key.
type receiverMap map[observer.EndpointID][]receiverEntry

// Put entry into key id. If entry is a duplicate it will still be added.
func (rm receiverMap) Put(id observer.EndpointID, entry receiverEntry) {
	rm[id] = append(rm[id], entry)
}

// Get receiver entries by endpoint id.
func (rm receiverMap) Get(id observer.EndpointID) []receiverEntry {
	return rm[id]
}

// Remove all receiver entries by endpoint id.
func (rm receiverMap) RemoveAll(id observer.EndpointID) {
	delete(rm, id)
}

// Values returns all receiver components in the map.
func (rm receiverMap) Values() (out []component.Component) {
	for _, entries := range rm {
		for _, entry := range entries {
			out = append(out, entry.receiver)
		}
	}
	return out
}

// Size is the number of total receiver entries in the map.
func (rm receiverMap) Size() (out int) {
	for _, entries := range rm {
		out += len(entries)
	}
	return out
}
