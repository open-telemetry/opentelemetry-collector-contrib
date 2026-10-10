// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v4"
	"github.com/stretchr/testify/assert"
)

func TestFilterResourcesByTags(t *testing.T) {
	production := "production"
	development := "development"

	resources := []*armresources.GenericResourceExpanded{
		{
			ID: new("/subscriptions/sub/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1"),
			Tags: map[string]*string{
				"environment": &production,
				"team":        new("platform"),
			},
		},
		{
			ID: new("/subscriptions/sub/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm2"),
			Tags: map[string]*string{
				"environment": &development,
			},
		},
		{
			ID: new("/subscriptions/sub/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm3"),
			Tags: map[string]*string{
				"team": new("backend"),
			},
		},
	}

	t.Run("no filters", func(t *testing.T) {
		assert.Len(t, filterResourcesByTags(resources, nil), 3)
	})

	t.Run("tag name and value", func(t *testing.T) {
		assert.Len(t, filterResourcesByTags(resources, []ResourceTagFilter{
			{Name: "environment", Value: &production},
		}), 1)
	})

	t.Run("tag name only", func(t *testing.T) {
		assert.Len(t, filterResourcesByTags(resources, []ResourceTagFilter{
			{Name: "team"},
		}), 2)
	})

	t.Run("multiple filters are ANDed", func(t *testing.T) {
		assert.Len(t, filterResourcesByTags(resources, []ResourceTagFilter{
			{Name: "environment", Value: &production},
			{Name: "team"},
		}), 1)
	})

	t.Run("tag name is case insensitive", func(t *testing.T) {
		assert.Len(t, filterResourcesByTags(resources, []ResourceTagFilter{
			{Name: "ENVIRONMENT", Value: &production},
		}), 1)
	})

	t.Run("tag value is case sensitive", func(t *testing.T) {
		assert.Empty(t, filterResourcesByTags(resources, []ResourceTagFilter{
			{Name: "environment", Value: new("Production")},
		}))
	})
}
