// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"

// Pools represents the top level json returned by the pool/stats endpoint
type Pools struct {
	Entries map[string]PoolStats `json:"entries"`
}

// PoolStats represents the statistics returned for a single pool
type PoolStats struct {
	NestedStats struct {
		Entries struct {
			Name struct {
				Description string `json:"description,omitempty"`
			} `json:"tmName,omitzero"`
			ServersideBitsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsIn,omitzero"`
			ServersideBitsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsOut,omitzero"`
			ServersideCurConns struct {
				Value int64 `json:"value"`
			} `json:"serverside.curConns,omitzero"`
			ServersidePktsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsIn,omitzero"`
			ServersidePktsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsOut,omitzero"`
			AvailabilityState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.availabilityState,omitzero"`
			EnabledState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.enabledState,omitzero"`
			TotalRequests struct {
				Value int64 `json:"value"`
			} `json:"totRequests,omitzero"`
			TotalMemberCount struct {
				Value int64 `json:"value"`
			} `json:"memberCnt,omitzero"`
			ActiveMemberCount struct {
				Value int64 `json:"value"`
			} `json:"activeMemberCnt,omitzero"`
			AvailableMemberCount struct {
				Value int64 `json:"value"`
			} `json:"availableMemberCnt,omitzero"`
		} `json:"entries,omitzero"`
	} `json:"nestedStats,omitzero"`
}
