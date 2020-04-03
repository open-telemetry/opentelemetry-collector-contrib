package stackdriverexporter

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
