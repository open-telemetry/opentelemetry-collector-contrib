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
			} `json:"tmName,omitempty"`
			// PoolName is not actually in the /stats response and will be pulled from the normal /virtual response
			PoolName struct {
				Description string `json:"description,omitempty"`
			} `json:"poolName,omitempty"`
			Destination struct {
				Description string `json:"description,omitempty"`
			} `json:"destination,omitempty"`
			ClientsideBitsIn struct {
				Value int64 `json:"value"`
			} `json:"clientside.bitsIn,omitempty"`
			ClientsideBitsOut struct {
				Value int64 `json:"value"`
			} `json:"clientside.bitsOut,omitempty"`
			ClientsideCurConns struct {
				Value int64 `json:"value"`
			} `json:"clientside.curConns,omitempty"`
			ClientsidePktsIn struct {
				Value int64 `json:"value"`
			} `json:"clientside.pktsIn,omitempty"`
			ClientsidePktsOut struct {
				Value int64 `json:"value"`
			} `json:"clientside.pktsOut,omitempty"`
			AvailabilityState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.availabilityState,omitempty"`
			EnabledState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.enabledState,omitempty"`
			TotalRequests struct {
				Value int64 `json:"value"`
			} `json:"totRequests,omitempty"`
		} `json:"entries,omitempty"`
	} `json:"nestedStats,omitempty"`
}
