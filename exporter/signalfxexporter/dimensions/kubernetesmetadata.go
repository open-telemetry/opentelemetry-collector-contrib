// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dimensions

import (
	"strings"

	"go.opentelemetry.io/collector/component/componenterror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

var propNameSanitizer = strings.NewReplacer(
	".", "_",
	"/", "_")

func getDimensionUpdateFromMetadata(metadata collection.KubernetesMetadataUpdate) *DimensionUpdate {
	properties, tagsToAdd, tagsToRemove := getPropertiesAndTags(metadata)

	return &DimensionUpdate{
		Name:         propNameSanitizer.Replace(metadata.ResourceIDKey),
		Value:        string(metadata.ResourceID),
		Properties:   properties,
		TagsToAdd:    tagsToAdd,
		TagsToRemove: tagsToRemove,
	}
}

func getPropertiesAndTags(kmu collection.KubernetesMetadataUpdate) (map[string]*string, []string, []string) {
	var tagsToAdd []string
	var tagsToRemove []string
	properties := map[string]*string{}

	for label, val := range kmu.MetadataToAdd {
		key := propNameSanitizer.Replace(label)
		if key == "" {
			continue
		}

		if val == "" {
			tagsToAdd = append(tagsToAdd, key)
		} else {
			propVal := val
			properties[key] = &propVal
		}
	}

	for label, val := range kmu.MetadataToRemove {
		key := propNameSanitizer.Replace(label)
		if key == "" {
			continue
		}

		if val == "" {
			tagsToRemove = append(tagsToRemove, key)
		} else {
			properties[key] = nil
		}
	}

	for label, val := range kmu.MetadataToUpdate {
		key := propNameSanitizer.Replace(label)
		if key == "" {
			continue
		}

		// Treat it as a remove if a property update has empty value since
		// this cannot be a tag as tags can either be added or removed but
		// not updated.
		if val == "" {
			properties[key] = nil
		} else {
			properties[key] = &val
		}
	}
	return properties, tagsToAdd, tagsToRemove
}

func (dc *DimensionClient) PushKubernetesMetadata(metadata []*collection.KubernetesMetadataUpdate) error {
	var errs []error
	for _, m := range metadata {
		if err := dc.pushKubernetesMetadataHelper(*m); err != nil {
			errs = append(errs, err)
		}
	}

	return componenterror.CombineErrors(errs)
}

func (dc *DimensionClient) pushKubernetesMetadataHelper(metadata collection.KubernetesMetadataUpdate) error {
	dimensionUpdate := getDimensionUpdateFromMetadata(metadata)
	return dc.acceptDimension(dimensionUpdate)
}
