// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"bytes"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
)

// filterDimensions transforms a list of azure dimensions into a list of string, taking in account the DimensionConfig
// given by the user.
func filterDimensions(dimensions []*armmonitor.LocalizableString, cfg DimensionsConfig, resourceType, metricName string) []string {
	// Only skip if explicitly disabled. Enabled by default.
	if cfg.Enabled != nil && !*cfg.Enabled {
		return nil
	}

	// If dimensions are overridden for that resource type and metric name, we take it
	if _, resourceTypeFound := cfg.Overrides[resourceType]; resourceTypeFound {
		if newDimensions, metricNameFound := cfg.Overrides[resourceType][metricName]; metricNameFound {
			return newDimensions
		}
	}
	// Otherwise we get all dimensions
	var result []string
	for _, dimension := range dimensions {
		result = append(result, *dimension.Value)
	}
	return result
}

// serializeDimensions build a comma separated string from trimmed, sorted dimensions list.
// It is designed to be used as a key in scraper maps.
func serializeDimensions(dimensions []string) string {
	var dimensionsSlice []string
	for _, dimension := range dimensions {
		if trimmedDimension := strings.TrimSpace(dimension); len(trimmedDimension) > 0 {
			dimensionsSlice = append(dimensionsSlice, trimmedDimension)
		}
	}
	sort.Strings(dimensionsSlice)
	return strings.Join(dimensionsSlice, ",")
}

// buildDimensionsFilter takes a serialized dimensions input to build an Azure Request filter that will allow us to
// receive metrics values split by these dimensions.
func buildDimensionsFilter(dimensionsStr string) *string {
	if len(dimensionsStr) == 0 {
		return nil
	}
	var dimensionsFilter bytes.Buffer
	dimensions := strings.Split(dimensionsStr, ",")
	for i, dimension := range dimensions {
		dimensionsFilter.WriteString(dimension)
		dimensionsFilter.WriteString(" eq '*'")
		if i < len(dimensions)-1 {
			dimensionsFilter.WriteString(" and ")
		}
	}
	result := dimensionsFilter.String()
	return &result
}
