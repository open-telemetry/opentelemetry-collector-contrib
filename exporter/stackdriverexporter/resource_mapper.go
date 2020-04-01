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
		if res.Type != mapping.SourceResourceType {
			continue
		}

		result := &monitoredrespb.MonitoredResource{
			Type: mapping.TargetResourceType,
		}

		if len(mapping.LabelMappings) > 0 {
			result.Labels = transformLabels(mapping.LabelMappings, res.Labels)
		} else {
			result.Labels = res.Labels
		}

		return result
	}

	// Keep original behavior by default
	return stackdriver.DefaultMapResource(res)
}

// transformLabels transforms labels according to the configured mappings.
func transformLabels(labelMappings []LabelMapping, input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	// Convert matching labels
	renamedLabels := make(map[string]struct{}, len(labelMappings))
	for _, labelMapping := range labelMappings {
		if v, ok := input[labelMapping.SourceLabelKey]; ok {
			output[labelMapping.TargetLabelKey] = v
			renamedLabels[labelMapping.SourceLabelKey] = struct{}{}
		}
	}
	// Copy remaining labels "as is"
	for k, v := range input {
		_, renamed := renamedLabels[k] // Skip labels that were renamed
		_, exists := output[k]         // Skip duplicate labels
		if !exists && !renamed {
			output[k] = v
		}
	}
	return output
}
