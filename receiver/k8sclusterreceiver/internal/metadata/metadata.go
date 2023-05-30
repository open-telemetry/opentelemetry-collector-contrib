// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
)

// KubernetesMetadata associates a resource to a set of properties.
type KubernetesMetadata struct {
	// resourceIDKey is the label key of UID label for the resource.
	ResourceIDKey string
	// resourceID is the Kubernetes UID of the resource. In case of
	// containers, this value is the container id.
	ResourceID metadataPkg.ResourceID
	// metadata is a set of key-value pairs that describe a resource.
	Metadata map[string]string
}

// GetGenericMetadata is responsible for collecting metadata from K8s resources that
// live on v1.ObjectMeta.
func GetGenericMetadata(om *v1.ObjectMeta, resourceType string) *KubernetesMetadata {
	rType := strings.ToLower(resourceType)
	metadata := maps.MergeStringMaps(map[string]string{}, om.Labels)

	metadata[constants.K8sKeyWorkLoadKind] = resourceType
	metadata[constants.K8sKeyWorkLoadName] = om.Name
	metadata[fmt.Sprintf("%s.creation_timestamp",
		rType)] = om.GetCreationTimestamp().Format(time.RFC3339)

	for _, or := range om.OwnerReferences {
		kind := strings.ToLower(or.Kind)
		metadata[GetOTelNameFromKind(kind)] = or.Name
		metadata[GetOTelUIDFromKind(kind)] = string(or.UID)
	}

	return &KubernetesMetadata{
		ResourceIDKey: GetOTelUIDFromKind(rType),
		ResourceID:    metadataPkg.ResourceID(om.UID),
		Metadata:      metadata,
	}
}

func GetOTelUIDFromKind(kind string) string {
	return fmt.Sprintf("k8s.%s.uid", kind)
}

func GetOTelNameFromKind(kind string) string {
	return fmt.Sprintf("k8s.%s.name", kind)
}

// mergeKubernetesMetadataMaps merges maps of string (resource id) to
// KubernetesMetadata into a single map.
func MergeKubernetesMetadataMaps(maps ...map[metadataPkg.ResourceID]*KubernetesMetadata) map[metadataPkg.ResourceID]*KubernetesMetadata {
	out := map[metadataPkg.ResourceID]*KubernetesMetadata{}
	for _, m := range maps {
		for id, km := range m {
			out[id] = km
		}
	}

	return out
}

// GetMetadataUpdate processes metadata updates and returns
// a map of a delta of metadata mapped to each resource.
func GetMetadataUpdate(old, new map[metadataPkg.ResourceID]*KubernetesMetadata) []*metadataPkg.MetadataUpdate {
	var out []*metadataPkg.MetadataUpdate

	for id, oldMetadata := range old {
		// if an object with the same id has a previous revision, take a delta
		// of the metadata.
		if newMetadata, ok := new[id]; ok {
			if metadataDelta := getMetadataDelta(oldMetadata.Metadata, newMetadata.Metadata); metadataDelta != nil {
				out = append(out, &metadataPkg.MetadataUpdate{
					ResourceIDKey: oldMetadata.ResourceIDKey,
					ResourceID:    id,
					MetadataDelta: *metadataDelta,
				})
			}
		}
	}

	// In case there are resources in the current revision, that was not in the
	// previous revision, collect metadata to be added
	for id, km := range new {
		// if an id is seen for the first time, all metadata need to be added.
		if _, ok := old[id]; !ok {
			out = append(out, &metadataPkg.MetadataUpdate{
				ResourceIDKey: km.ResourceIDKey,
				ResourceID:    id,
				MetadataDelta: metadataPkg.MetadataDelta{MetadataToAdd: km.Metadata},
			})
		}
	}

	return out
}

// getMetadataDelta returns MetadataDelta between two sets for properties.
// If the delta between old (oldProps) and new (newProps) revisions of a
// resource end up being empty, nil is returned.
func getMetadataDelta(oldProps, newProps map[string]string) *metadataPkg.MetadataDelta {

	toAdd, toRemove, toUpdate := map[string]string{}, map[string]string{}, map[string]string{}

	// If metadata exist in the previous revision as well, collect if
	// the new values are different. Otherwise, the property is a new value
	// and has to be added.
	for key, newVal := range newProps {
		if oldVal, ok := oldProps[key]; ok {
			if oldVal != newVal {
				toUpdate[key] = newVal
			}
		} else {
			toAdd[key] = newVal
		}
	}

	// Properties that don't exist in the latest revision should be removed
	for key, val := range oldProps {
		if _, ok := newProps[key]; !ok {
			toRemove[key] = val
		}
	}

	if len(toAdd) > 0 || len(toRemove) > 0 || len(toUpdate) > 0 {
		return &metadataPkg.MetadataDelta{
			MetadataToAdd:    toAdd,
			MetadataToRemove: toRemove,
			MetadataToUpdate: toUpdate,
		}
	}

	return nil
}
