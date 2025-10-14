// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"

// VirtualServersDetails represents the top level json returned by the /virtual endpoint
type VirtualServersDetails struct {
	Items []VirtualServerProperties `json:"items"`
}

// VirtualServerProperties represents the properties returned for a single virtual server
type VirtualServerProperties struct {
	SelfLink string `json:"selfLink"`
	PoolName string `json:"pool"`
}

// VirtualServers represents the top level json returned by the virtual/stats endpoint
type VirtualServers struct {
	Entries map[string]VirtualServerStats `json:"entries"`
}

// VirtualServerStats represents the statistics returned for a single virtual server
type VirtualServerStats struct {
	NestedStats struct {
		Entries struct {
			Name struct {
				Description string `json:"description,omitempty"`
			} `json:"tmName,omitzero"`
			// PoolName is not actually in the /stats response and will be pulled from the normal /virtual response
			PoolName struct {
				Description string `json:"description,omitempty"`
			} `json:"poolName,omitzero"`
			Destination struct {
				Description string `json:"description,omitempty"`
			} `json:"destination,omitzero"`
			ClientsideBitsIn struct {
				Value int64 `json:"value"`
			} `json:"clientside.bitsIn,omitzero"`
			ClientsideBitsOut struct {
				Value int64 `json:"value"`
			} `json:"clientside.bitsOut,omitzero"`
			ClientsideCurConns struct {
				Value int64 `json:"value"`
			} `json:"clientside.curConns,omitzero"`
			ClientsidePktsIn struct {
				Value int64 `json:"value"`
			} `json:"clientside.pktsIn,omitzero"`
			ClientsidePktsOut struct {
				Value int64 `json:"value"`
			} `json:"clientside.pktsOut,omitzero"`
			AvailabilityState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.availabilityState,omitzero"`
			EnabledState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.enabledState,omitzero"`
			TotalRequests struct {
				Value int64 `json:"value"`
			} `json:"totRequests,omitzero"`
		} `json:"entries,omitzero"`
	} `json:"nestedStats,omitzero"`
}
