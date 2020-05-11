// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collection

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

const (
	// Keys for K8s metadata
	k8sKeyWorkLoadKind = "k8s.workload.kind"
	k8sKeyWorkLoadName = "k8s.workload.name"
)

type ResourceID string

// KubernetesMetadata associates a resource to a set of properties.
type KubernetesMetadata struct {
	// resourceIDKey is the label key of UID label for the resource.
	resourceIDKey string
	// resourceID is the Kubernetes UID of the resource. In case of
	// containers, this value is the container id.
	resourceID ResourceID
	// metadata is a set of key-value pairs that describe a resource.
	metadata map[string]string
}

// getGenericMetadata is responsible for collecting metadata from K8s resources that
// live on v1.ObjectMeta.
func getGenericMetadata(om *v1.ObjectMeta, resourceType string) *KubernetesMetadata {
	rType := strings.ToLower(resourceType)
	metadata := utils.MergeStringMaps(map[string]string{}, om.Labels)

	metadata[k8sKeyWorkLoadKind] = rType
	metadata[k8sKeyWorkLoadName] = om.Name
	metadata[fmt.Sprintf("%s.creation_timestamp",
		rType)] = om.GetCreationTimestamp().Format(time.RFC3339)

	for _, or := range om.OwnerReferences {
		metadata[strings.ToLower(or.Kind)] = or.Name
		metadata[strings.ToLower(or.Kind)+"_uid"] = string(or.UID)
	}

	return &KubernetesMetadata{
		resourceIDKey: getResourceIDKey(rType),
		resourceID:    ResourceID(om.UID),
		metadata:      metadata,
	}
}

func getResourceIDKey(rType string) string {
	return fmt.Sprintf("k8s.%s.uid", rType)
}

// mergeKubernetesMetadataMaps merges maps of string (resource id) to
// KubernetesMetadata into a single map.
func mergeKubernetesMetadataMaps(maps ...map[ResourceID]*KubernetesMetadata) map[ResourceID]*KubernetesMetadata {
	out := map[ResourceID]*KubernetesMetadata{}
	for _, m := range maps {
		for id, km := range m {
			out[id] = km
		}
	}

	return out
}

// KubernetesMetadataExporter provides an interface to implement
// ConsumeKubernetesMetadata in Exporters that support metadata.
type KubernetesMetadataExporter interface {
	// ConsumeKubernetesMetadata will be invoked every time there's an
	// update to a resource that results in one or more KubernetesMetadataUpdate.
	ConsumeKubernetesMetadata(metadata []*KubernetesMetadataUpdate) error
}

// KubernetesMetadataUpdate provides a delta view of metadata on a resource between
// two revisions of a resource.
type KubernetesMetadataUpdate struct {
	// ResourceIDKey is the label key of UID label for the resource.
	ResourceIDKey string
	// ResourceID is the Kubernetes UID of the resource. In case of
	// containers, this value is the container id.
	ResourceID ResourceID
	MetadataDelta
}

// GetKubernetesMetadataUpdate processes kubernetes metadata updates and returns
// a map of a delta of metadata mapped to each resource.
func GetKubernetesMetadataUpdate(old, new map[ResourceID]*KubernetesMetadata) []*KubernetesMetadataUpdate {
	var out []*KubernetesMetadataUpdate

	for id, oldMetadata := range old {
		// if an object with the same id has a previous revision, take a delta
		// of the metadata.
		if newMetadata, ok := new[id]; ok {
			if metadataDelta := getMetadataDelta(oldMetadata.metadata, newMetadata.metadata); metadataDelta != nil {
				out = append(out, &KubernetesMetadataUpdate{
					ResourceIDKey: oldMetadata.resourceIDKey,
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
			out = append(out, &KubernetesMetadataUpdate{
				ResourceIDKey: km.resourceIDKey,
				ResourceID:    id,
				MetadataDelta: MetadataDelta{MetadataToAdd: km.metadata},
			})
		}
	}

	return out
}

// MetadataDelta keeps track of changes to metadata on resources.
// The fields on this struct should help determine if there have
// been changes to resource metadata such as Kubernetes labels.
// An example of how this is used. Let's say we are dealing with a
// Pod that has the following labels -
// {"env": "test", "team": "otell", "usser": "bob"}. Now, let's say
// there's an update to one or more labels on the same Pod and the
// labels now look like the following -
// {"env": "test", "team": "otel", "user": "bob"}. The k8sclusterreceiver
// upon receiving the event corresponding to the labels updates will
// generate a MetadataDelta with the following values -
// 					MetadataToAdd: {"user": "bob"}
// 					MetadataToRemove: {"usser": "bob"}
// 					MetadataToUpdate: {"team": "otel"}
// Apart from Kubernetes labels, the other metadata collected by this
// receiver are also handled in the same manner.
type MetadataDelta struct {
	// MetadataToAdd contains key-value pairs that are newly added to
	// the resource description in the current revision.
	MetadataToAdd map[string]string
	// MetadataToRemove contains key-value pairs that no longer describe
	// a resource and needs to be removed.
	MetadataToRemove map[string]string
	// MetadataToUpdate contains key-value pairs that have been updated
	// in the current revision compared to the previous revisions(s).
	MetadataToUpdate map[string]string
}

// getMetadataDelta returns MetadataDelta between two sets for properties.
// If the delta between old (oldProps) and new (newProps) revisions of a
// resource end up being empty, nil is returned.
func getMetadataDelta(oldProps, newProps map[string]string) *MetadataDelta {

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
		return &MetadataDelta{
			MetadataToAdd:    toAdd,
			MetadataToRemove: toRemove,
			MetadataToUpdate: toUpdate,
		}
	}

	return nil
}
