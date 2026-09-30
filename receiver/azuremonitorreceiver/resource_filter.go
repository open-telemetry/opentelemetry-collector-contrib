// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v4"
)

func filterResourcesByTags(
	resources []*armresources.GenericResourceExpanded,
	filters []ResourceTagFilter,
) []*armresources.GenericResourceExpanded {
	if len(filters) == 0 {
		return resources
	}

	filtered := make([]*armresources.GenericResourceExpanded, 0, len(resources))

	for _, resource := range resources {
		if resource == nil || !resourceMatchesTags(resource.Tags, filters) {
			continue
		}

		filtered = append(filtered, resource)
	}

	return filtered
}

func resourceMatchesTags(tags map[string]*string, filters []ResourceTagFilter) bool {
	for _, filter := range filters {
		matched := false

		for name, value := range tags {
			if !strings.EqualFold(name, filter.Name) {
				continue
			}

			if filter.Value == nil {
				matched = true
				break
			}

			if value != nil && *value == *filter.Value {
				matched = true
				break
			}
		}

		if !matched {
			return false
		}
	}

	return true
}
