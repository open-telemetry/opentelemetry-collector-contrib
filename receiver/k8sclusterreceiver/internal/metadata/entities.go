// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

// GetEntityEvents processes metadata updates and returns entity events that describe the metadata changes.
func GetEntityEvents(oldMetadata, newMetadata map[metadataPkg.ResourceID]*KubernetesMetadata, timestamp pcommon.Timestamp) metadataPkg.EntityEventsSlice {
	out := metadataPkg.NewEntityEventsSlice()

	for id, oldObj := range oldMetadata {
		if _, ok := newMetadata[id]; !ok {
			// An object was present, but no longer is. Create a "delete" event.
			entityEvent := out.AppendEmpty()
			entityEvent.SetTimestamp(timestamp)
			entityEvent.ID().PutStr(oldObj.ResourceIDKey, string(oldObj.ResourceID))
			entityEvent.SetEntityDelete()
		}
	}

	// All "new" are current objects. Create "state" events. "old" state does not matter.
	for _, newObj := range newMetadata {
		entityEvent := out.AppendEmpty()
		entityEvent.SetTimestamp(timestamp)
		entityEvent.ID().PutStr(newObj.ResourceIDKey, string(newObj.ResourceID))
		state := entityEvent.SetEntityState()
		state.SetEntityType(newObj.EntityType)

		attrs := state.Attributes()
		for k, v := range newObj.Metadata {
			attrs.PutStr(k, v)
		}
	}

	return out
}
