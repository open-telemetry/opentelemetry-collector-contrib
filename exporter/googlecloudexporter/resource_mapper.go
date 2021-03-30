// Copyright 2019 OpenTelemetry Authors
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

package googlecloudexporter

import (
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/resource"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

type resourceMapper struct {
	mappings []ResourceMapping
}

func (mr *resourceMapper) mapResource(res *resource.Resource) *monitoredrespb.MonitoredResource {
	for _, mapping := range mr.mappings {
		if res.Type != mapping.SourceType {
			continue
		}

		result := &monitoredrespb.MonitoredResource{
			Type: mapping.TargetType,
		}

		labels, ok := transformLabels(mapping.LabelMappings, res.Labels)

		if !ok {
			// Skip mapping, fallback to default handling
			continue
		}

		result.Labels = labels
		return result
	}

	// cloud.zone was renamed to cloud.availability_zone in the semantic conventions.
	// Accept either, for backwards compatibility.
	availabilityZone, ok := res.Labels["cloud.availability_zone"]
	if ok {
		res.Labels["cloud.zone"] = availabilityZone
	}

	// Keep original behavior by default
	return stackdriver.DefaultMapResource(res)
}

// transformLabels transforms labels according to the configured mappings.
// Returns true if all required labels in match are found.
func transformLabels(labelMappings []LabelMapping, input map[string]string) (map[string]string, bool) {
	output := make(map[string]string, len(input))
	// Convert matching labels
	for _, labelMapping := range labelMappings {
		if v, ok := input[labelMapping.SourceKey]; ok {
			output[labelMapping.TargetKey] = v
		} else if !labelMapping.Optional {
			// Required label is missing
			return nil, false
		}
	}
	return output, true
}
