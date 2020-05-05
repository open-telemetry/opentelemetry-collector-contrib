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
	// Keys for K8s properties
	k8sKeyWorkLoadKind = "k8s.workload.kind"
	k8sKeyWorkLoadName = "k8s.workload.name"
)

// KubernetesMetadata associates a resource to a set of properties.
type KubernetesMetadata struct {
	resourceIDKey string
	resourceID    string
	properties    map[string]string
}

// getGenericMetadata is responsible for collecting metadata from K8s resources that
// live on v1.ObjectMeta.
func getGenericMetadata(om *v1.ObjectMeta, resourceType string) *KubernetesMetadata {
	rType := strings.ToLower(resourceType)
	properties := utils.MergeStringMaps(map[string]string{}, om.Labels)

	properties[k8sKeyWorkLoadKind] = rType
	properties[k8sKeyWorkLoadName] = om.Name
	properties[fmt.Sprintf("%s.creation_timestamp",
		rType)] = om.GetCreationTimestamp().Format(time.RFC3339)

	for _, or := range om.OwnerReferences {
		properties[strings.ToLower(or.Kind)] = or.Name
		properties[strings.ToLower(or.Kind)+"_uid"] = string(or.UID)
	}

	return &KubernetesMetadata{
		resourceIDKey: getResourceIDKey(rType),
		resourceID:    string(om.UID),
		properties:    properties,
	}
}

func getResourceIDKey(rType string) string {
	return fmt.Sprintf("k8s.%s.uid", rType)
}

// mergeKubernetesMetadataMaps merges maps of string (resource id) to
// KubernetesMetadata into a single map.
func mergeKubernetesMetadataMaps(maps ...map[string]*KubernetesMetadata) map[string]*KubernetesMetadata {
	out := map[string]*KubernetesMetadata{}
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
	ConsumeKubernetesMetadata(metadata map[string]*KubernetesMetadataUpdate) error
}

// KubernetesMetadataUpdate provides a delta view of properties on a resource.
type KubernetesMetadataUpdate struct {
	ResourceIDKey      string
	ResourceID         string
	PropertiesToAdd    map[string]string
	PropertiesToUpdate map[string]string
	PropertiesToRemove map[string]string
}

// GetKubernetesMetadataUpdate processes kubernetes metadata updates and returns
// a map of a delta of properties mapped to each resource.
func GetKubernetesMetadataUpdate(old, new map[string]*KubernetesMetadata) map[string]*KubernetesMetadataUpdate {
	out := map[string]*KubernetesMetadataUpdate{}

	for id, oldMetadata := range old {
		// if an object with the same id has a previous revision, take a delta
		// of the properties.
		if newMetadata, ok := new[id]; ok {
			toAdd, toRemove, toUpdate := getPropertiesDelta(oldMetadata.properties, newMetadata.properties)
			out[id] = &KubernetesMetadataUpdate{
				ResourceIDKey:      oldMetadata.resourceIDKey,
				ResourceID:         id,
				PropertiesToAdd:    toAdd,
				PropertiesToRemove: toRemove,
				PropertiesToUpdate: toUpdate,
			}
		}
	}

	// In case there are resources in the current revision, that was not in the
	// previous revision, collect properties to be added
	for id, km := range new {
		// if an id is seen for the first time, all properties need to be added.
		if _, ok := old[id]; !ok {
			out[id] = &KubernetesMetadataUpdate{
				ResourceIDKey:   km.resourceIDKey,
				ResourceID:      id,
				PropertiesToAdd: km.properties,
			}
		}
	}

	return out
}

func getPropertiesDelta(oldProps, newProps map[string]string) (map[string]string, map[string]string, map[string]string) {
	toAdd, toRemove, toUpdate := map[string]string{}, map[string]string{}, map[string]string{}

	// If properties exist in the previous revision as well, collect if
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

	return toAdd, toRemove, toUpdate
}
