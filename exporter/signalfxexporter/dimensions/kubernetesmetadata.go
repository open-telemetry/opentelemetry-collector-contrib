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
	"fmt"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/collector/component/componenterror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

var propNameSanitizer = strings.NewReplacer(
	".", "_",
	"/", "_")

func getDimensionUpdateFromMetadata(metadata collection.KubernetesMetadataUpdate) *DimensionUpdate {
	properties, tags := getPropertiesAndTags(metadata)

	return &DimensionUpdate{
		Name:       propNameSanitizer.Replace(metadata.ResourceIDKey),
		Value:      string(metadata.ResourceID),
		Properties: properties,
		Tags:       tags,
	}
}

func getPropertiesAndTags(kmu collection.KubernetesMetadataUpdate) (map[string]*string, map[string]bool) {
	properties := map[string]*string{}
	tags := map[string]bool{}

	for label, val := range kmu.MetadataToAdd {
		key := propNameSanitizer.Replace(label)
		if key == "" {
			continue
		}

		if val == "" {
			tags[key] = true
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
			tags[key] = false
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
			propVal := val
			properties[key] = &propVal
		}
	}
	return properties, tags
}

func (dc *DimensionClient) PushKubernetesMetadata(metadata []*collection.KubernetesMetadataUpdate) error {
	var errs []error
	for _, m := range metadata {
		dimensionUpdate := getDimensionUpdateFromMetadata(*m)

		if dimensionUpdate.Name == "" || dimensionUpdate.Value == "" {
			atomic.AddInt64(&dc.TotalInvalidDimensions, int64(1))
			return fmt.Errorf("dimensionUpdate %v is missing Name or value, cannot send", dimensionUpdate)
		}

		if err := dc.acceptDimension(dimensionUpdate); err != nil {
			errs = append(errs, err)
		}
	}

	return componenterror.CombineErrors(errs)
}
